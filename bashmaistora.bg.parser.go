package parser

import (
	"context"
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

var baseBashmaistoraUrl = url.URL{
	Scheme: "https",
	Host:   "bashmaistora.bg",
}

func StartBashmaistoraParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseBashmaistoraUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseBashmaistoraUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseBashmaistoraUrl.Host), colly.Async(true))
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
	findItemsAndVisitItBash(c, tempPageToVisitChan)
	parseItemPage_bsh(c, parserParams, tempItemsToSaveChan)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось открыть начальную страницу %s: %s\n.", parserParams.UrlToParse, err)
	}

	c.Wait()
	tempPageToVisitChan <- rxgo.Item{}
	close(itemsToSaveChan)
	wg.Wait()
}

func findItemsAndVisitItBash(c *colly.Collector, pageChan chan<- rxgo.Item) {
	// on main container
	c.OnHTML("main#content .two_cols_page .right_content", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection, isItem bool) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseBashmaistoraUrl, href)

			if isItem {
				pageChan <- rxgo.Item{V: linkTo}
			} else {
				err := c.Visit(linkTo)
				if err != nil {
					fmt.Printf("Не удалось открыть страницу с товаром %s: %s\n.", linkTo, err)
				}
			}
		}

		articleGroupsSelector := ".right_content_cats > a.cats_box"
		articleGroups := e.DOM.Find(articleGroupsSelector)
		articleGroupsLength := articleGroups.Length()

		if articleGroupsLength != 0 {
			// in group we not check amount of items in page because I didn't find any pages with more then one page with groups
			articleGroups.Each(func(i int, s *goquery.Selection) {
				visitPage(s, false)
			})
			return
		}

		paginatorUrlParamName := "p"
		articlesSelector := ".list_products > .p_slider_box"
		articles := e.DOM.Find(articlesSelector)
		articlesLength := articles.Length()

		// default 100 items on page
		if articlesLength == 100 {
			currentHref := e.Request.URL.String()
			validCurrentHref := GetValidLinkOr(currentHref, baseBashmaistoraUrl, currentHref)

			validCurrentUrl, err := url.Parse(validCurrentHref)
			if err != nil {
				newUrlToVisit := validCurrentHref + fmt.Sprintf("?%s=2", paginatorUrlParamName)
				err = c.Visit(newUrlToVisit)
				if err != nil {
					fmt.Printf("Не удалось перейти на следующую страницу для ссылки %s - %s.\n", validCurrentHref, newUrlToVisit)
				}
			} else {
				urlQuery := validCurrentUrl.Query()
				paginatorUrlParam := urlQuery.Get(paginatorUrlParamName)
				numberParam, err := strconv.Atoi(paginatorUrlParam)
				if err != nil {
					numberParam = 1
				}
				numberParam++
				urlQuery.Set(paginatorUrlParamName, fmt.Sprintf("%v", numberParam))

				validCurrentUrl.RawQuery = urlQuery.Encode()
				stringUrlToVisit := validCurrentUrl.String()

				err = c.Visit(stringUrlToVisit)
				if err != nil {
					fmt.Printf("Не удалось перейти на следующую страницу для ссылки %s.\n", stringUrlToVisit)
				}
			}
		}

		articles.Each(func(i int, s *goquery.Selection) {
			aElement := s.Find("a.img_wrap")
			visitPage(aElement, true)
		})
	})
}

func parseItemPage_bsh(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML("main#content .product_top_row", func(e *colly.HTMLElement) {
		var err error

		articul := strings.TrimSpace(e.ChildText(".middle_part .small_info.left + .left.w100 > .small_info.left"))
		articul = strings.ReplaceAll(articul, "Кат. номер: ", "")
		articul = strings.TrimSpace(articul)

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseBashmaistoraUrl, href)

		name := strings.TrimSpace(e.ChildText(".heading_wrap .heading"))

		imageLink := e.ChildAttr(".left_part .product_img a.MagicZoom", "href")
		image := imageLink
		if !params.WithoutImages && imageLink != "" {
			image, err = DownloadImageIfNeed(imageLink, params, baseBashmaistoraUrl)
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
