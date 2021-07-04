package parser

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/microcosm-cc/bluemonday"
)

var baseDewaltUrl = url.URL{
	Scheme: "https",
	Host:   "shop.dewalt.ru",
}

var baseStanlyUrl = url.URL{
	Scheme: "https",
	Host:   "shop.stanley.ru",
}

var baseBlackAndDeckertUrl = url.URL{
	Scheme: "https",
	Host:   "shop.blackanddecker.ru",
}

var baseUrl url.URL

var sanitizer = bluemonday.UGCPolicy().SkipElementsContent("a")

func StartDW_ST_BADParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}

	if parsedBaseUrl.Host != baseDewaltUrl.Host && parsedBaseUrl.Host != baseStanlyUrl.Host && parsedBaseUrl.Host != baseBlackAndDeckertUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s, %s, %s\n", parserParams.UrlToParse, baseDewaltUrl.String(), baseStanlyUrl.String(), baseBlackAndDeckertUrl.String())
		os.Exit(1)
	}
	baseUrl = *parsedBaseUrl

	c := colly.NewCollector(colly.AllowedDomains(baseUrl.Host), colly.Async(true))

	itemsToSaveChan := make(chan Item)
	var wg sync.WaitGroup

	err = listenItemsAndSaveToFile_dw(itemsToSaveChan, parserParams, &wg)
	if err != nil {
		fmt.Printf("Не удалось установить соединение с файлом: %s\n", err)
		os.Exit(1)
	}

	findNewPageAndVisitIt_dw(c)
	logPageVisiting_dw(c)
	findAndParseItemsOnPage_dw(c, parserParams, itemsToSaveChan, &wg)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось открыть начальную страницу %s: %s\n.", parserParams.UrlToParse, err)
	}

	c.Wait()
	close(itemsToSaveChan)
	wg.Wait()
}

func findAndParseItemsOnPage_dw(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- Item, wg *sync.WaitGroup) {
	c.OnHTML(".main > .category-wrapper > .category-products .product-cont .product-item__side .product-item__image-section > a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		linkTo, err := GetValidLink(href, baseUrl)
		if err != nil {
			linkTo = href
		}

		err = c.Visit(linkTo)
		if err != nil {
			fmt.Printf("Не удалось открыть страницу с товаром %s: %s\n.", linkTo, err)
		}
	})

	c.OnHTML(".page-wrapper > .main-container > .container > .main", func(e *colly.HTMLElement) {
		if e.DOM.Parent().Find(".main > .h1_category-title").Length() != 0 {
			// skip non item pages
			return
		}

		articul := strings.TrimSpace(e.ChildText(".product-card .product-card__sku span"))
		href := e.Request.URL.String()
		linkTo, err := GetValidLink(href, baseUrl)
		if err != nil {
			linkTo = href
		}

		descriptionText := e.ChildText(".description__info-text")
		description := ""
		descriptionElement := e.DOM.Find(".description__info-text")
		if descriptionElement == nil {
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionText
			}
		}

		technicalAttrText := e.ChildText(".product-view__attributes")
		technicalAttr := ""
		technicalAttrElement := e.DOM.Find(".product-view__attributes")
		if technicalAttrElement == nil {
			technicalAttr = technicalAttrText
		} else {

			technicalAttrElement.Find(".product-view__attribute-title").Each(func(_ int, selection *goquery.Selection) {
				text := selection.Text()
				if text == "Дополнительная информация" {
					selection.Parent().Remove()
				}
			})

			technicalAttr, err = technicalAttrElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить технические атрибуты элемента %s в формате html (%s).\n", articul, linkTo)
				technicalAttr = technicalAttrText
			}
		}

		name := strings.TrimSpace(e.ChildText(".product-card .product-card__title"))

		imageLink := e.ChildAttr(".images-gallery__items .images-gallery__slide", "data-src")
		image := imageLink
		if !params.WithoutImages && imageLink != "" {
			image, err = DownloadImageIfNeed(imageLink, params, baseUrl)
			if err != nil {
				fmt.Println(err)
			}
		}

		technicalAttr = fmt.Sprintf("<div class=\"dw-pars\">%s</div>", sanitizer.Sanitize(technicalAttr))

		item := &ExternalItem{
			Articul:       articul,
			Description:   sanitizer.Sanitize(description),
			Image:         image,
			LinkTo:        linkTo,
			Name:          name,
			TechnicalAttr: technicalAttr,
		}

		wg.Add(1)
		itemsToSaveChan <- item
	})
}

func listenItemsAndSaveToFile_dw(itemsToSaveChan <-chan Item, params *ParserParams, wg *sync.WaitGroup) error {
	filePath := GetValidPath(params.DataFilePath)
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0777)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		for item := range itemsToSaveChan {
			err := AppendItemToFile(item, file)
			if err != nil {
				fmt.Printf("Неудалось записать в файл: %s, %s: %s\n", item.GetLink(), item.GetId(), err)
				AppendUnparsedItemToFile(item)
			}
			wg.Done()
		}
		wg.Done()
		file.Close()
	}()

	return nil
}

func findNewPageAndVisitIt_dw(c *colly.Collector) {
	c.OnHTML(".toolbar-pages__list", func(e *colly.HTMLElement) {
		var activeElement *colly.HTMLElement
		var activeElementIndex int

		var nextPageElement *colly.HTMLElement

		e.ForEach(".toolbar-pages__link", func(i int, element *colly.HTMLElement) {
			if element.DOM.Is(".toolbar-pages__link_active") {
				activeElement = element
				activeElementIndex = i
				return
			}

			if activeElement != nil && activeElementIndex == i-1 {
				nextPageElement = element
			}
		})

		if nextPageElement == nil {
			fmt.Printf("Неудалось найти следующую страницу: %#v\n", e.DOM)
			return
		}

		href := nextPageElement.Attr("href")
		validUrl, err := GetValidLink(href, baseUrl)
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

func logPageVisiting_dw(c *colly.Collector) {
	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("Парсим следующую страницу - %s\n", r.URL.String())
	})
}
