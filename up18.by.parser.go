package parser

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

var baseUp18Url = url.URL{
	Scheme: "https",
	Host:   "up18.by",
}

func StartUp18Parser(parserParams *ParserParams) {
	c := colly.NewCollector(colly.AllowedDomains(baseUp18Url.Host), colly.Async(true))

	itemsToSaveChan := make(chan Item)
	var wg sync.WaitGroup

	err := ListenInternalItemsAndSaveToFile(itemsToSaveChan, parserParams, &wg)
	if err != nil {
		fmt.Printf("Не удалось установить соединение с файлом: %s\n", err)
		os.Exit(1)
	}
	findNewPageAndVisitIt(c)
	LogPageVisiting(c)
	findAndParseItemsOnPage(c, parserParams, itemsToSaveChan, &wg)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Println(err)
	}

	c.Wait()
	close(itemsToSaveChan)
	wg.Wait()
}

func findAndParseItemsOnPage(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- Item, wg *sync.WaitGroup) {
	c.OnHTML(".itemList .item", func(e *colly.HTMLElement) {
		var err error

		price := strings.ReplaceAll(e.ChildText(".price.item-price__actual span"), " ", "")
		artikul := strings.TrimSpace(e.ChildText(".itemArt span"))
		itemTitle := strings.TrimSpace(e.ChildText(".itemTitle span"))
		href := e.ChildAttr(".itemTitle a", "href")
		linkTo := GetValidLinkOr(href, baseUp18Url, href)

		imageLink := e.ChildAttr("img", "src")
		image := imageLink
		if !params.WithoutImages {
			image, err = DownloadImageIfNeed(imageLink, params, baseUp18Url)
			if err != nil {
				fmt.Println(err)
			}
			if strings.HasSuffix(image, "nofoto.jpg") {
				image = ""
			}
		}

		item := &InternalItem{
			Price:     price,
			Artikul:   artikul,
			ItemTitle: itemTitle,
			LinkTo:    linkTo,
			Image:     image,
		}

		wg.Add(1)
		itemsToSaveChan <- item
	})
}

func findNewPageAndVisitIt(c *colly.Collector) {
	c.OnHTML(".pagination span + a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		validUrl, err := GetValidLink(href, baseUp18Url)
		if err != nil {
			fmt.Printf("Неудалось получить правильную ссылку для `%s`: %s\n", href, err)
			validUrl = href
		}
		err = c.Visit(validUrl)
		if err != nil {
			fmt.Printf("Неудалось посетить следующую страницу `%s`: %s\n", validUrl, err)
			WriteCrushedUrlToFile(validUrl)
		}
	})
}
