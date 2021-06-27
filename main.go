package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

func main() {
	defer fmt.Println("Конец.")
	parserParams := initParserParams()
	err := initParser(parserParams)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseUp18Url.Host), colly.Async(true))

	itemsToSaveChan := make(chan *Item)
	var wg sync.WaitGroup

	err = listenItemsAndSaveToFile(itemsToSaveChan, parserParams, &wg)
	if err != nil {
		fmt.Printf("Не удалось установить соединение с файлом: %s\n", err)
		os.Exit(1)
	}
	findNewPageAndVisitIt(c)
	logPageVisiting(c)
	findAndParseItemsOnPage(c, parserParams, itemsToSaveChan, &wg)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Println(err)
	}

	c.Wait()
	close(itemsToSaveChan)
	wg.Wait()
}

func findAndParseItemsOnPage(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- *Item, wg *sync.WaitGroup) {
	c.OnHTML(".itemList .item", func(e *colly.HTMLElement) {
		price := strings.ReplaceAll(e.ChildText("[itemProp=\"price\"]"), " ", "")
		artikul := strings.TrimSpace(e.ChildText(".itemArt span"))
		itemTitle := strings.TrimSpace(e.ChildText(".itemTitle span"))
		href := e.ChildAttr(".itemTitle a", "href")
		linkTo, err := getValidLink(href)
		if err != nil {
			linkTo = href
		}

		imageLink := e.ChildAttr("img", "src")
		image := imageLink
		if !params.WithoutImages {
			image, err = downloadImageIfNeed(imageLink, params)
			if err != nil {
				fmt.Println(err)
			}
			if strings.HasSuffix(image, "nofoto.jpg") {
				image = ""
			}
		}

		item := &Item{
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

func listenItemsAndSaveToFile(itemsToSaveChan <-chan *Item, params *ParserParams, wg *sync.WaitGroup) error {
	filePath := getValidPath(params.DataFilePath)
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0777)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		for item := range itemsToSaveChan {
			err := appendItemToFile(item, file)
			if err != nil {
				fmt.Printf("Неудалось записать в файл: %s, %s: %s\n", item.Artikul, item.LinkTo, err)
				appendUnparsedItemToFile(item)
			}
			wg.Done()
		}
		wg.Done()
		file.Close()
	}()

	return nil
}

func findNewPageAndVisitIt(c *colly.Collector) {
	c.OnHTML(".pagination span + a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		validUrl, err := getValidLink(href)
		if err != nil {
			fmt.Printf("Неудалось получить правильную ссылку для `%s`: %s\n", href, err)
			validUrl = href
		}
		err = c.Visit(validUrl)
		if err != nil {
			fmt.Printf("Неудалось посетить следующую страницу `%s`: %s\n", validUrl, err)
			writeCrushedUrlToFile(validUrl)
		}
	})
}

func logPageVisiting(c *colly.Collector) {
	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("Парсим следующую страницу - %s\n", r.URL.String())
	})
}
