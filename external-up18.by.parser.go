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

var baseExternalUp18Url = url.URL{
	Scheme: "https",
	Host:   "up18.by",
}

func StartInternalUp18Parser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseExternalUp18Url.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseExternalUp18Url.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseExternalUp18Url.Host), colly.Async(true))
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
	findItemsAndVisitIt_ExternalUp18(c, tempPageToVisitChan, &wgForItems, parserParams)
	parseItemPage_ExternalUp18(c, parserParams, tempItemsToSaveChan)

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

func findItemsAndVisitIt_ExternalUp18(c *colly.Collector, pageChan chan<- rxgo.Item, wg *sync.WaitGroup, params *ParserParams) {
	// on main container
	pageNumber := 1
	c.OnHTML(".itemListWrapper", func(e *colly.HTMLElement) {
		if pageNumber == 1 {
			currentPageNumber := e.DOM.Find(".pagination span").Text()
			currentPossiblePaheNumber, _ := strconv.Atoi(strings.TrimSpace(currentPageNumber))
			if currentPossiblePaheNumber > 1 {
				pageNumber = currentPossiblePaheNumber
			}
		}
		visitPage := func(s *goquery.Selection) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseExternalUp18Url, href)

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

		articlesSelector := ".itemList .item .itemTitle a"
		articles := e.DOM.Find(articlesSelector)

		fmt.Println("Количество элементов на странице: ", articles.Length())

		nextPaginatorButton := e.DOM.Find(".pagination span + a")
		nextHref, _ := nextPaginatorButton.Attr("href")
		nextHref = strings.TrimSpace(nextHref)

		isLastPage := nextHref == ""

		articles.Each(func(i int, s *goquery.Selection) {
			visitPage(s)
		})

		if isLastPage || params.NotFollowPagination {
			fmt.Println("Пагинация закончилась")
			pageChan <- rxgo.Item{V: "finish"}
			return
		}

		nextLinkTo := GetValidLinkOr(nextHref, baseExternalUp18Url, nextHref)

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

func parseItemPage_ExternalUp18(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML(".container .inner-container", func(e *colly.HTMLElement) {
		var articul string

		e.ForEach(".inner-info__item", func(_ int, el *colly.HTMLElement) {
			name := strings.TrimSpace(el.ChildText(".inner-info__name-in"))
			if name == "Артикул" {
				articul = strings.TrimSpace(el.ChildText(".inner-info__val-in"))
			}
		})

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseExternalUp18Url, href)

		name := strings.TrimSpace(e.ChildText(".inner-container__title"))

		imagesLinks := e.DOM.Find(".inner-img__big-pic-list.big-pic-list .big-pic-list__pic").Map(func(i int, selection *goquery.Selection) string {
			return selection.AttrOr("href", "")
		})

		if params.EmptyImageToSet != "" && len(imagesLinks) == 1 && strings.HasSuffix(imagesLinks[0], "images/product/nofoto.jpg") {
			imagesLinks = []string{params.EmptyImageToSet}
		}

		var downloadedImages []string

		if imagesLinks != nil {
			for _, imgLink := range imagesLinks {
				downloadedImageByLink, err := DownloadImageIfNeed(imgLink, params, baseExternalUp18Url)
				if err != nil {
					fmt.Println(err)
					continue
				}
				downloadedImages = append(downloadedImages, downloadedImageByLink)
			}
		}

		item := &ExternalItem{
			Articul: articul,
			Images:  downloadedImages,
			LinkTo:  linkTo,
			Name:    name,
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}
