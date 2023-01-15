package parser

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/reactivex/rxgo/v2"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var baseJapanUkraine = url.URL{
	Scheme: "https",
	Host:   "japan-ukraine.com",
}

func JapanUkraineParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseJapanUkraine.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseJapanUkraine.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseJapanUkraine.Host), colly.Async(true))
	c.SetRequestTimeout(3 * time.Minute)

	itemsToSaveChan := make(chan Item)
	var wg sync.WaitGroup

	err = ListenExternalItemsAndSaveToFile(itemsToSaveChan, parserParams, &wg)
	if err != nil {
		fmt.Printf("Не удалось установить соединение с файлом: %s\n", err)
		os.Exit(1)
	}

	tempItemsToSaveChan := make(chan rxgo.Item)
	tempPageToVisitChan := make(chan rxgo.Item)

	zipper := func(_ context.Context, item interface{}, page interface{}) (interface{}, error) {
		if validItem, ok := item.(Item); ok && validItem != nil {
			wg.Add(1)
			itemsToSaveChan <- validItem
		} else {
			if item != nil {
				fmt.Printf("Не удалось определить элемент - %s\n", item)
			}
		}

		if link, ok := page.(string); ok {
			err := c.Visit(link)
			if err != nil {
				fmt.Println("Не удалось посетить ссылку - ", link)
			}
		} else {
			fmt.Printf("Не удалось определить ссылку - %s\n", link)
		}
		return nil, nil
	}

	rxgo.FromChannel(tempItemsToSaveChan).ZipFromIterable(rxgo.FromChannel(tempPageToVisitChan), zipper).ForEach(func(_ interface{}) {}, nil, nil)

	tempItemsToSaveChan <- rxgo.Item{}

	LogPageVisiting(c)
	findItemsAndVisitItJU(c, tempPageToVisitChan)
	parseItemPage_ju(c, parserParams, tempItemsToSaveChan)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось открыть начальную страницу %s: %s\n.", parserParams.UrlToParse, err)
	}

	c.Wait()
	tempPageToVisitChan <- rxgo.Item{}
	close(itemsToSaveChan)
	wg.Wait()
}

func findItemsAndVisitItJU(c *colly.Collector, pageChan chan<- rxgo.Item) {
	// on main container
	c.OnHTML(".main > ul", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection, isItem bool) {

			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseJapanUkraine, href)

			if isItem {
				pageChan <- rxgo.Item{V: linkTo}
			} else {
				err := c.Visit(linkTo)
				if err != nil {
					fmt.Printf("Не удалось открыть страницу с товаром %s: %s\n.", linkTo, err)
				}
			}
		}

		articleGroupsSelector := "li > a"
		articleGroups := e.DOM.Find(articleGroupsSelector)

		articleGroups.Each(func(i int, s *goquery.Selection) {
			visitPage(s, false)
		})

		articlesSelector := "li a.name-link"
		articles := e.DOM.Find(articlesSelector)

		articles.Each(func(i int, s *goquery.Selection) {
			visitPage(s, true)
		})
	})
}

func parseItemPage_ju(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML(".main", func(e *colly.HTMLElement) {
		var err error

		codeClass := e.ChildAttr("#karta_main .kod", "class")

		if codeClass != "kod" {
			return
		}

		articul := strings.TrimSpace(e.ChildText("h1"))
		_, articul, _ = strings.Cut(articul, "MAKITA")
		articul = strings.ReplaceAll(articul, " ", "")
		articul = strings.TrimSpace(articul)

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseJapanUkraine, href)

		name := strings.TrimSpace(e.ChildText("h1"))

		imageLink := e.ChildAttr("#img_main img", "src")

		image := imageLink
		if !params.WithoutImages && imageLink != "" {
			image, err = DownloadNamedImageIfNeed(imageLink, params, baseJapanUkraine, articul)
			if err != nil {
				fmt.Println(err)
			}
		}

		item := &ExternalItem{
			Articul:       articul,
			Description:   "",
			Image:         image,
			LinkTo:        linkTo,
			Name:          name,
			TechnicalAttr: "",
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}
