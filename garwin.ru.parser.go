package parser

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var baseGarwinUrl = url.URL{
	Scheme: "https",
	Host:   "garwin.ru",
}

func StartGarwinParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseGarwinUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseGarwinUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseGarwinUrl.Host), colly.Async(true))
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
	findItemsAndVisitIt_Garwin(c, tempPageToVisitChan, &wgForItems)
	parseItemPage_Garwin(c, parserParams, tempItemsToSaveChan)

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

func findItemsAndVisitIt_Garwin(c *colly.Collector, pageChan chan<- rxgo.Item, wg *sync.WaitGroup) {
	// on main container
	pageNumber := 1
	c.OnHTML(".CatalogPage__Main", func(e *colly.HTMLElement) {
		visitPage := func(s *goquery.Selection) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			linkTo := GetValidLinkOr(href, baseGarwinUrl, href)

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

		articlesSelector := ".CatalogPage__Products .ProductListingOverlayLink"
		articles := e.DOM.Find(articlesSelector)

		fmt.Println("Количество элементов на странице: ", articles.Length())

		nextPaginatorButton := e.DOM.Find(".PaginatorPages__Next")
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

		nextLinkTo := GetValidLinkOr(nextHref, baseGarwinUrl, nextHref)

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

func parseItemPage_Garwin(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML(".ProductPage", func(e *colly.HTMLElement) {
		var err error

		articul := e.DOM.Find(".ProductPageInfo__Sku").Text()
		articul = strings.TrimSpace(articul)

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseGarwinUrl, href)

		name := strings.TrimSpace(e.ChildText(".ProductPageInfo__Title"))

		imagesLinks, err := extractImageURLsFromHTML(e.Response.Body)
		if err != nil {
			fmt.Printf("failed to extract image URLs: %s\n", err)
			imagesLinks = nil
		}

		var downloadedImages []string

		if imagesLinks != nil {
			for _, imgLink := range imagesLinks {
				downloadedImageByLink, err := DownloadImageIfNeed(imgLink, params, baseGarwinUrl)
				if err != nil {
					fmt.Println(err)
					continue
				}
				downloadedImages = append(downloadedImages, downloadedImageByLink)
			}
		}

		descriptionText := e.ChildText(".ProductDetailDescription__Text")
		description := ""
		descriptionElement := e.DOM.Find(".ProductDetailDescription__Text")
		if descriptionElement == nil {
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionText
			}
		}

		technicalAttrText := e.ChildText(".ProductDetailSection .ProductDetailCharacteristics")
		technicalAttr := ""
		technicalAttrElement := e.DOM.Find(".ProductDetailSection .ProductDetailCharacteristics")
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
			Articul:     articul,
			Description: strings.TrimSpace(sanitizer.SkipElementsContent("br").Sanitize(description)),
			//Image:         downloadedImage,
			Images:        downloadedImages,
			LinkTo:        linkTo,
			Name:          name,
			TechnicalAttr: strings.TrimSpace(sanitizer.SkipElementsContent("br").Sanitize(technicalAttr)),
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}

// splitAndTrim splits a string by commas and trims spaces from each element.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// splitByCommaOutsideQuotes splits a string by commas but ignores commas inside quotes.
func splitByCommaOutsideQuotes(input string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(input); i++ {
		char := input[i]

		switch char {
		case '"':
			inQuotes = !inQuotes // Toggle inQuotes state when encountering a quote
			current.WriteByte(char)
		case ',':
			if inQuotes {
				// If inside quotes, keep the comma as part of the value
				current.WriteByte(char)
			} else {
				// If outside quotes, split here
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			}
		default:
			current.WriteByte(char)
		}
	}

	// Add the last portion of the string
	result = append(result, strings.TrimSpace(current.String()))

	return result
}

// extractImageURLsFromHTML receives the input data as a byte slice,
// locates the root self-invoking function assigned to window.__NUXT__,
// extracts its parameter list and its invocation argument list,
// then finds the images array and for each image id (literal or variable)
// builds and returns the full image URL using width 768 and JPEG format.
func extractImageURLsFromHTML(data []byte) ([]string, error) {
	text := string(data)

	// Revised regex for the root function.
	// It matches: window.__NUXT__=(function(...){...}(...))
	reFunc := regexp.MustCompile(`window\.__NUXT__=\(function\((.*)\){.*}\((.*)\)\)`)
	funcMatches := reFunc.FindStringSubmatch(text)
	var paramList, argList []string
	if funcMatches != nil && len(funcMatches) >= 3 {
		paramList = splitAndTrim(funcMatches[1])
		argList = splitByCommaOutsideQuotes(funcMatches[2])
		// Note: if counts differ, later lookups may fail.
	}

	// Find the images array block.
	reImagesBlock := regexp.MustCompile(`images\s*:\s*\[([\s\S]*?)\]`)
	imagesBlockMatch := reImagesBlock.FindStringSubmatch(text)
	if imagesBlockMatch == nil || len(imagesBlockMatch) < 2 {
		return nil, fmt.Errorf("no images block found")
	}
	imagesContent := imagesBlockMatch[1]

	// This regex matches an id field that is either a quoted string literal or a bare variable.
	reImageID := regexp.MustCompile(`id\s*:\s*(?:"([^"]+)"|([A-Za-z0-9_$]+))`)
	idMatches := reImageID.FindAllStringSubmatch(imagesContent, -1)
	if idMatches == nil {
		return nil, fmt.Errorf("no image IDs found")
	}

	var urls []string
	for _, match := range idMatches {
		var imageID string
		if match[1] != "" {
			// Literal string: use it directly.
			imageID = match[1]
		} else if match[2] != "" && len(paramList) > 0 && len(argList) > 0 {
			// Variable reference: find its index in the parameter list.
			varName := match[2]
			index := -1
			for i, p := range paramList {
				if p == varName {
					index = i
					break
				}
			}
			if index >= 0 && index < len(argList) {
				// Remove any surrounding quotes from the argument.
				imageID = strings.Trim(argList[index], `"`)
			} else {
				continue
			}
		} else {
			continue
		}

		// Validate that the imageID is long enough to extract path parts.
		if len(imageID) < 4 {
			continue
		}
		part1 := imageID[0:2]
		part2 := imageID[2:4]
		// Build the URL using width 768 and JPEG format ("r" suffix).
		imageUrl := fmt.Sprintf("https://media.garwin.ru/images/products/%s/%s/%s-w768r.jpeg", part1, part2, imageID)
		headResp, err := http.Head(imageUrl)
		if err == nil && headResp.StatusCode != http.StatusNotFound {
			urls = append(urls, imageUrl)
		} else {
			imageUrl = fmt.Sprintf("https://media.garwin.ru/images/products/%s/%s/%s-w768p.webp", part1, part2, imageID)
			headResp, err = http.Head(imageUrl)
			if err == nil && headResp.StatusCode != http.StatusNotFound {
				urls = append(urls, imageUrl)
			}
		}
	}
	return urls, nil
}
