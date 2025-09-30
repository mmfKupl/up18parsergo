package parser

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/reactivex/rxgo/v2"
)

var baseMitoolsByUrl = url.URL{
	Scheme: "https",
	Host:   "mitools.by",
}

func StartMitoolsByParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseMitoolsByUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseMitoolsByUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseMitoolsByUrl.Host), colly.Async(true))
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
	findItemsAndVisitIt_MitoolsBy(c, tempPageToVisitChan, &wgForItems)
	parseItemPage_MitoolsBy(c, parserParams, tempItemsToSaveChan)

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

func findItemsAndVisitIt_MitoolsBy(c *colly.Collector, pageChan chan<- rxgo.Item, wg *sync.WaitGroup) {
	// on main container
	pageNumber := 1
	c.OnHTML(".catalog_page.basket_normal", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseMitoolsByUrl, href)

			parsedLink, err := url.Parse(linkTo)
			if err != nil {
				pageChan <- rxgo.Item{V: linkTo}
				return
			}

			linkQuery := parsedLink.Query()
			linkQuery.Set("page", strconv.Itoa(pageNumber))

			parsedLink.RawQuery = linkQuery.Encode()
			finalLink := parsedLink.String()

			pageChan <- rxgo.Item{V: finalLink}
			wg.Add(1)
		}

		articlesSelector := ".display_list .list_item_wrapp .list_item .item-title .dark_link"
		articles := e.DOM.Find(articlesSelector)

		fmt.Println("Количество элементов на странице: ", articles.Length())

		nextPaginatorButton := e.DOM.Find(".flex-nav-next .flex-next")
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

		nextLinkTo := GetValidLinkOr(nextHref, baseMitoolsByUrl, nextHref)

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

func parseItemPage_MitoolsBy(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML(".wraps.hover_shine", func(e *colly.HTMLElement) {
		var err error

		articul := e.DOM.Find(".article__value").Text()
		articul = strings.TrimSpace(articul)

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseMitoolsByUrl, href)

		name := strings.TrimSpace(e.ChildText(".topic__heading#pagetitle"))

		imagesLinks := e.DOM.Find(".product-detail-gallery__container .product-detail-gallery__slider .product-detail-gallery__item.product-detail-gallery__item--middle .product-detail-gallery__link").Map(func(i int, selection *goquery.Selection) string {
			return selection.AttrOr("href", "")
		})

		if err != nil {
			fmt.Printf("failed to extract image URLs: %s\n", err)
			imagesLinks = nil
		}

		var downloadedImages []string

		if imagesLinks != nil {
			for _, imgLink := range imagesLinks {
				downloadedImageByLink, err := DownloadImageIfNeedInLowerRegister(imgLink, params, baseMitoolsByUrl)
				if err != nil {
					fmt.Println(err)
					continue
				}
				downloadedImages = append(downloadedImages, downloadedImageByLink)
			}
		}

		descriptionText := e.ChildText(".bottom-info .content ul")
		description := ""
		descriptionElement := e.DOM.Find(".bottom-info .content ul")
		if descriptionElement == nil {
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionText
			}
		}

		technicalAttrText := e.ChildText(".char_block.bordered")
		technicalAttr := ""
		technicalAttrElement := e.DOM.Find(".char_block.bordered")
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
			TechnicalAttr: stripIDsAndRemoveEmptyTables(ReplaceMultiSpaces(sanitizer.SkipElementsContent("br").Sanitize(technicalAttr))),
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}

func stripIDsAndRemoveEmptyTables(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}

	// Remove all id attributes
	doc.Find("[id]").Each(func(_ int, s *goquery.Selection) {
		s.RemoveAttr("id")
	})

	// Remove empty tables (no visible text after trimming)
	doc.Find("table").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "" {
			s.Remove()
		}
	})

	out, err := doc.Find("body").Html()
	if err != nil || strings.TrimSpace(out) == "" {
		// Fallback to full doc HTML if body extraction fails
		if h, err2 := doc.Html(); err2 == nil {
			return h
		}
		return html
	}
	return out
}
