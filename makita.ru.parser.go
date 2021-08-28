package parser

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/microcosm-cc/bluemonday"
	"github.com/reactivex/rxgo/v2"
)

var baseMakitaUrl = url.URL{
	Scheme: "https",
	Host:   "www.makita.ru",
}

var mkSanitizer = bluemonday.UGCPolicy().SkipElementsContent("a")

func StartMakitaParser(parserParams *ParserParams) {
	mkSanitizer.AllowAttrs("class").OnElements("i")
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseMakitaUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseMakitaUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseMakitaUrl.Host), colly.Async(true))
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
	findItemsAndVisitIt(c, tempPageToVisitChan)
	parseItemPage_mk(c, parserParams, tempItemsToSaveChan)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось открыть начальную страницу %s: %s\n.", parserParams.UrlToParse, err)
	}

	c.Wait()
	tempPageToVisitChan <- rxgo.Item{}
	close(itemsToSaveChan)
	wg.Wait()
}

func findItemsAndVisitIt(c *colly.Collector, pageChan chan<- rxgo.Item) {
	// on main container
	c.OnHTML(".content_container > .section.group", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection, isItem bool) {
			href, _ := s.Attr("href")
			linkTo := GetValidLinkOr(href, baseMakitaUrl, href)

			if isItem {
				pageChan <- rxgo.Item{V: linkTo}
			} else {
				err := c.Visit(linkTo)
				if err != nil {
					fmt.Printf("Не удалось открыть страницу с товаром %s: %s\n.", linkTo, err)
				}
			}
		}

		articleGroupsSelector := ".main > .default_artikelgroepen .article_groups.overview_tiles .article_group"
		articleGroups := e.DOM.Find(articleGroupsSelector)
		articleGroupsLength := articleGroups.Length()

		if articleGroupsLength != 0 {
			// in group we not check amount of items in page because I didn't find any pages with more then one page with groups
			articleGroups.Each(func(i int, s *goquery.Selection) {
				aElement := s.Find(".content.tile a")
				visitPage(aElement, false)
			})
			return
		}

		paginatorUrlParamName := "paging_page"
		articlesSelector := ".main > .default_artikelen_2 .article_overview.overview_tiles .article"
		articles := e.DOM.Find(articlesSelector)
		articlesLength := articles.Length()

		// default 12 items on page
		if articlesLength == 12 {
			currentHref := e.Request.URL.String()
			validCurrentHref := GetValidLinkOr(currentHref, baseMakitaUrl, currentHref)

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
			aElement := s.Find(".product-title a")
			visitPage(aElement, true)
		})
	})
}

func parseItemPage_mk(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML(".main > .main_inner > .default_artikel_v2", func(e *colly.HTMLElement) {
		var err error

		articul := strings.TrimSpace(e.ChildText(".product-title .product-number"))
		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseMakitaUrl, href)

		descriptionText := e.ChildText(".product-description")
		description := ""
		descriptionElement := e.DOM.Find(".product-description")
		if descriptionElement == nil {
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionText
			}
		}

		technicalAttrText := e.ChildText(".techspecs")
		technicalAttr := ""
		technicalAttrElement := e.DOM.Find(".techspecs")
		if technicalAttrElement == nil {
			technicalAttr = technicalAttrText
		} else {
			technicalAttr, err = technicalAttrElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить технические атрибуты элемента %s в формате html (%s).\n", articul, linkTo)
				technicalAttr = technicalAttrText
			}
		}

		name := strings.TrimSpace(e.ChildText(".product-title h1"))

		imageLink := e.ChildAttr(".large .swiper-wrapper .detail-img", "href")
		image := imageLink
		if !params.WithoutImages && imageLink != "" {
			image, err = DownloadImageIfNeed(imageLink, params, baseMakitaUrl)
			if err != nil {
				fmt.Println(err)
			}
		}

		item := &ExternalItem{
			Articul:       articul,
			Description:   RemoveAllEnters(mkSanitizer.Sanitize(description)),
			Image:         image,
			LinkTo:        linkTo,
			Name:          name,
			TechnicalAttr: RemoveAllEnters(fmt.Sprintf("<div class=\"dw-pars\">%s</div>", mkSanitizer.Sanitize(technicalAttr))),
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}

func RemoveAllEnters(s string) string {
	return strings.ReplaceAll(
		strings.TrimSpace(s),
		"\n",
		"",
	)
}
