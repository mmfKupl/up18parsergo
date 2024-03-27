package parser

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/reactivex/rxgo/v2"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var baseDW4YouUrl = url.URL{
	Scheme: "https",
	Host:   "www.dw4you.ru",
}

func StartDW4YouParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseDW4YouUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseDW4YouUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseDW4YouUrl.Host), colly.Async(true))
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
	findItemsAndVisitItDW4You(c, tempPageToVisitChan, &wgForItems)
	parseItemPage_DW4You(c, parserParams, tempItemsToSaveChan)

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

func findItemsAndVisitItDW4You(c *colly.Collector, pageChan chan<- rxgo.Item, wg *sync.WaitGroup) {
	// on main container
	pageNumber := 1
	c.OnHTML("#mfilter-content-container", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseDW4YouUrl, href)

			parsedLink, err := url.Parse(linkTo)
			if err != nil {
				pageChan <- rxgo.Item{V: linkTo}
				return
			}

			linkQuery := parsedLink.Query()
			linkQuery.Set("PAGE_NUMBER", strconv.Itoa(pageNumber))

			parsedLink.RawQuery = linkQuery.Encode()
			finalLink := parsedLink.String()

			pageChan <- rxgo.Item{V: finalLink}
			wg.Add(1)
		}

		articlesSelector := ".product-layout .caption h4 a"
		articles := e.DOM.Find(articlesSelector)

		fmt.Println("Количество элементов на странице: ", articles.Length())

		nextPaginatorButton := e.DOM.Find(".pagination li a").FilterFunction(func(i int, selection *goquery.Selection) bool {
			return selection.Text() == ">"
		})
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

		nextLinkTo := GetValidLinkOr(nextHref, baseDW4YouUrl, nextHref)

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

func parseItemPage_DW4You(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML("#content.col-sm-12", func(e *colly.HTMLElement) {
		var err error

		articul := e.DOM.Find(".list-unstyled li").FilterFunction(func(i int, selection *goquery.Selection) bool {
			return strings.Contains(selection.Text(), "Артикул")
		}).Text()
		articul = strings.ReplaceAll(articul, "Артикул:", "")
		articul = strings.ReplaceAll(articul, ".", "")
		articul = strings.ReplaceAll(articul, " ", "")
		articul = strings.TrimSpace(articul)

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseDW4YouUrl, href)

		name := strings.TrimSpace(e.ChildText(".col-sm-4 h1"))

		imageLink, _ := e.DOM.Find(".col-sm-8 .thumbnails a.thumbnail").Attr("href")
		image := imageLink
		if !params.WithoutImages && imageLink != "" {
			image, err = DownloadImageIfNeed(imageLink, params, baseDW4YouUrl)
			if err != nil {
				fmt.Println(err)
			}
		}

		descriptionText := e.ChildText("#tab-description")
		description := ""
		descriptionElement := e.DOM.Find("#tab-description")
		if descriptionElement == nil {
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionText
			}
		}

		item := &ExternalItem{
			Articul:     articul,
			Description: strings.TrimSpace(sanitizer.SkipElementsContent("br").Sanitize(description)),
			Image:       image,
			LinkTo:      linkTo,
			Name:        name,
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}
