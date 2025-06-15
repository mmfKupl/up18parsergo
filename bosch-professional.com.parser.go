package parser

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/reactivex/rxgo/v2"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var baseBoshProfUrl = url.URL{
	Scheme: "https",
	Host:   "www.bosch-professional.com",
}

func StartBoshProfParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseBoshProfUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseBoshProfUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseBoshProfUrl.Host), colly.Async(true))
	c.SetRequestTimeout(3 * time.Minute)

	itemsToSaveChan := make(chan Item)
	var wg sync.WaitGroup
	var wgForItems sync.WaitGroup

	err = ListenExternalItemsAndSaveToFile(itemsToSaveChan, parserParams, &wg)
	if err != nil {
		fmt.Printf("Не удалось установить соединение с файлом: %s\n", err)
		os.Exit(1)
	}

	tempItemsToSaveChan := make(chan rxgo.Item)
	tempPageToVisitChan := make(chan rxgo.Item)

	rxgo.FromChannel(tempPageToVisitChan).ForEach(func(page interface{}) {
		if link, ok := page.(string); ok {

			if link == "finish" {
				wgForItems.Done()
				close(tempPageToVisitChan)
				return
			}

			fmt.Println("Будет посещена следующая страница (zipper)", link)
			visited, err := c.HasVisited(link)
			if err != nil {
				fmt.Println("Не удалось проверить ссылку на посещение - ", link, err)
			}
			if !visited {
				err := c.Visit(link)
				if err != nil {
					fmt.Println("Не удалось посетить ссылку - ", link, err)
				}
			} else {
				fmt.Println("Страница уже посещена - ", link)
			}
		} else {
			fmt.Printf("Не удалось определить ссылку - %s\n", link)
		}
	}, nil,
		func() {
			fmt.Println("Комплит потока")
		},
	)

	rxgo.FromChannel(tempItemsToSaveChan).ForEach(func(item interface{}) {
		if validItem, ok := item.(Item); ok && validItem != nil {
			wg.Add(1)
			fmt.Println("Будет сохранен элемент:", validItem.GetId())
			itemsToSaveChan <- validItem
			wgForItems.Done()
		} else {
			if item != nil {
				fmt.Printf("Не удалось определить элемент - %s\n", item)
			}
		}
	}, nil, func() {})

	LogPageVisiting(c)
	wgForItems.Add(1)
	findItemsAndVisitIt_BoshProf(c, tempPageToVisitChan, &wgForItems)
	parseItemPage_BoshProf(c, parserParams, tempItemsToSaveChan)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось открыть начальную страницу %s: %s\n.", parserParams.UrlToParse, err)
	}

	wgForItems.Wait()
	close(tempItemsToSaveChan)
	close(itemsToSaveChan)

	wg.Wait()
	fmt.Println("Завершение работы парсера")
}

func findItemsAndVisitIt_BoshProf(c *colly.Collector, pageChan chan<- rxgo.Item, wg *sync.WaitGroup) {
	// on main container
	pageNumber := 1
	c.OnHTML(".category-filters", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseBoshProfUrl, href)

			parsedLink, err := url.Parse(linkTo)
			if err != nil {
				pageChan <- rxgo.Item{V: linkTo}
				return
			}

			pagePath := "page/" + strconv.Itoa(pageNumber) + "/"
			finalLink, err := url.JoinPath(linkTo, pagePath)
			if err != nil {
				finalLink = path.Join(parsedLink.Path, pagePath)
				pageChan <- rxgo.Item{V: finalLink}
				return
			}

			pageChan <- rxgo.Item{V: finalLink}
			wg.Add(1)
		}

		articlesSelector := ".category-grid-tile__link-wrapper"
		articles := e.DOM.Find(articlesSelector)

		fmt.Println("Количество элементов на странице: ", articles.Length())

		nextPaginatorButton := e.DOM.Find(".m-ghostblock__nav-item.active + a.m-ghostblock__nav-item")
		nextHref, _ := nextPaginatorButton.Attr("href")
		nextHref = strings.TrimSpace(nextHref)

		isLastPage := nextHref == ""

		articles.Each(func(i int, s *goquery.Selection) {
			visitPage(s)
		})

		if isLastPage {
			fmt.Println("Пагинация закончилась")
			pageChan <- rxgo.Item{V: "finish"}
			return
		}

		nextLinkTo := GetValidLinkOr(nextHref, baseBoshProfUrl, nextHref)

		if nextLinkTo != "" {
			pageNumber++
			fmt.Println("Будет посещена следующая страница (пагинация)", nextLinkTo)
			err := c.Visit(nextLinkTo)
			if err != nil {
				fmt.Printf("Не удалось перейти на следующую страницу для ссылки %s.\n", nextLinkTo)
			}
		}
	})
}

func parseItemPage_BoshProf(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML(".t-productdetailpage main", func(e *colly.HTMLElement) {
		var err error

		articul := e.DOM.Find(".o-b-product_variations__product--selected .a-ordernumber").Text()
		articul = strings.ReplaceAll(strings.TrimSpace(articul), " ", "")

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseBoshProfUrl, href)

		name := strings.TrimSpace(e.ChildText(".product-detail-stage__title"))

		imagesLinks := e.DOM.Find(".modal-container.container .product-detail-stage__slider-wrapper img.lazyload[data-src]").
			Map(func(i int, s *goquery.Selection) string {
				dataSrc, _ := s.Attr("data-src")
				unescapedSrc, err := url.QueryUnescape(dataSrc)
				if err != nil {
					return dataSrc
				}
				return unescapedSrc
			})
		if err != nil {
			fmt.Printf("failed to extract image URLs: %s\n", err)
			imagesLinks = nil
		}

		var downloadedImages []string

		if imagesLinks != nil {
			for imageIndex, imgLink := range imagesLinks {
				imageName := fmt.Sprintf("%s-%s", GetImageNameFromUrl(imgLink), strconv.Itoa(imageIndex))
				downloadedImageByLink, err := DownloadNamedImageIfNeed(imgLink, params, baseBoshProfUrl, imageName, false)
				if err != nil {
					fmt.Println(err)
					continue
				}
				downloadedImages = append(downloadedImages, downloadedImageByLink)
			}
		}

		descriptionText := e.ChildText(".m-a-product_highlights.trackingModule .col-md-10")
		description := ""
		descriptionElement := e.DOM.Find(".m-a-product_highlights.trackingModule .col-md-10")
		if descriptionElement == nil {
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionText
			}
		}

		technicalAttrText := e.ChildText(".o-technical_data.trackingModule .row:nth-child(2) .table:not(.hidden)")
		technicalAttr := ""
		technicalAttrElement := e.DOM.Find(".o-technical_data.trackingModule .row:nth-child(2) .table:not(.hidden)")
		if technicalAttrElement == nil {
			technicalAttr = technicalAttrText
		} else {
			technicalAttr, err = technicalAttrElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить технические атрибуты элемента %s в формате html (%s).\n", articul, linkTo)
				technicalAttr = technicalAttrText
			}
		}

		item := &ExternalItem{
			Articul:       articul,
			Description:   ReplaceMultiSpaces(sanitizer.SkipElementsContent("br").Sanitize(description)),
			Images:        downloadedImages,
			LinkTo:        linkTo,
			Name:          name,
			TechnicalAttr: ReplaceMultiSpaces(sanitizer.SkipElementsContent("br").Sanitize(technicalAttr)),
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}
