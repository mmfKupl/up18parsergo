package parser

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/reactivex/rxgo/v2"
)

var baseRuntecShopUrl = url.URL{
	Scheme: "https",
	Host:   "runtec-shop.ru",
}

const (
	runtecShopMaxRetries = 4
	runtecShopRetryKey   = "runtec-shop-retry"
)

func StartRuntecShopParser(parserParams *ParserParams) {
	parsedBaseUrl, err := url.Parse(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось определить базовый урл для сайта %s: %s\n", parserParams.UrlToParse, err)
		os.Exit(1)
	}
	if parsedBaseUrl.Host != baseRuntecShopUrl.Host {
		fmt.Printf("Передана невалидная ссылка на сайт - %s, используйте ссылки со следующих сайтов - %s\n", parserParams.UrlToParse, baseRuntecShopUrl.String())
		os.Exit(1)
	}

	c := colly.NewCollector(colly.AllowedDomains(baseRuntecShopUrl.Host), colly.Async(true))
	c.SetRequestTimeout(3 * time.Minute)
	err = c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       1500 * time.Millisecond,
		RandomDelay: 1500 * time.Millisecond,
	})
	if err != nil {
		fmt.Printf("Не удалось установить лимит запросов: %s\n", err)
		os.Exit(1)
	}
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36")
		r.Headers.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	})

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
	var initialPageDone sync.Once
	var queuedPages sync.Map

	doneInitialPage := func() {
		initialPageDone.Do(func() {
			wgForItems.Done()
		})
	}
	donePage := func(link string) {
		if _, loaded := queuedPages.LoadAndDelete(link); loaded {
			wgForItems.Done()
			return
		}
		doneInitialPage()
	}

	rxgo.FromChannel(tempPageToVisitChan).ForEach(func(page interface{}) {
		if link, ok := page.(string); ok {
			if link == "finish" {
				doneInitialPage()
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
					donePage(link)
				}
			} else {
				fmt.Println("Страница уже посещена - ", link)
				donePage(link)
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
			donePage(validItem.GetLink())
		} else {
			if item != nil {
				fmt.Printf("Не удалось определить элемент - %s\n", item)
			}
		}
	}, nil, func() {})

	LogPageVisiting(c)
	c.OnError(func(response *colly.Response, err error) {
		if retryRuntecShopRequest(response, err) {
			return
		}
		if response == nil || response.Request == nil {
			doneInitialPage()
			return
		}
		donePage(response.Request.URL.String())
	})
	wgForItems.Add(1)
	findItemsAndVisitIt_RuntecShop(c, parserParams, tempPageToVisitChan, &wgForItems, &queuedPages)
	parseItemPage_RuntecShop(c, parserParams, tempItemsToSaveChan)

	err = c.Visit(parserParams.UrlToParse)
	if err != nil {
		fmt.Printf("Не удалось открыть начальную страницу %s: %s\n.", parserParams.UrlToParse, err)
		doneInitialPage()
	}

	wgForItems.Wait()
	close(tempItemsToSaveChan)
	close(itemsToSaveChan)

	wg.Wait()
	fmt.Println("Завершение работы парсера")
}

func findItemsAndVisitIt_RuntecShop(c *colly.Collector, params *ParserParams, pageChan chan<- rxgo.Item, wg *sync.WaitGroup, queuedPages *sync.Map) {
	c.OnHTML("#catalog_section_app", func(e *colly.HTMLElement) {
		if strings.Contains(string(e.Response.Body), `id="catalog_element_runtec"`) {
			return
		}

		articlesSelector := ".catalog-items-grid .product-item-container .sale-item-name"
		articles := e.DOM.Find(articlesSelector)

		fmt.Println("Количество элементов на странице: ", articles.Length())

		articles.Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			href = strings.TrimSpace(href)
			if href == "" {
				return
			}

			linkTo := GetValidLinkOr(href, baseRuntecShopUrl, href)
			queuedPages.Store(linkTo, true)
			wg.Add(1)
			pageChan <- rxgo.Item{V: linkTo}
		})

		if params.NotFollowPagination {
			fmt.Println("Пагинация отключена")
			pageChan <- rxgo.Item{V: "finish"}
			return
		}

		nextHref := getRuntecShopNextPageHref(e.DOM)

		if nextHref == "" {
			fmt.Println("Пагинация закончилась")
			pageChan <- rxgo.Item{V: "finish"}
			return
		}

		nextLinkTo := GetValidLinkOr(nextHref, baseRuntecShopUrl, nextHref)
		fmt.Println("Будет посещена следующая страница (пагинация)", nextLinkTo)
		err := c.Visit(nextLinkTo)
		if err != nil {
			fmt.Printf("Не удалось перейти на следующую страницу для ссылки %s: %s.\n", nextLinkTo, err)
			pageChan <- rxgo.Item{V: "finish"}
		}
	})
}

func parseItemPage_RuntecShop(c *colly.Collector, params *ParserParams, itemsToSaveChan chan<- rxgo.Item) {
	c.OnHTML("#catalog_element_runtec", func(e *colly.HTMLElement) {
		var err error

		articul := getRuntecShopArticul(e.DOM)

		href := e.Request.URL.String()
		linkTo := GetValidLinkOr(href, baseRuntecShopUrl, href)

		name := strings.TrimSpace(e.ChildText("h1.title"))

		imagesLinks := getRuntecShopImageLinks(e.DOM)
		var downloadedImages []string

		for _, imgLink := range imagesLinks {
			downloadedImageByLink, err := DownloadRuntecShopImageIfNeed(imgLink, params)
			if err != nil {
				fmt.Println(err)
				continue
			}
			downloadedImages = append(downloadedImages, downloadedImageByLink)
		}

		descriptionText := ""
		description := ""
		descriptionElement := findRuntecShopTab(e.DOM, "description")
		if descriptionElement == nil {
			descriptionText = e.ChildText(".tab_content")
			description = descriptionText
		} else {
			description, err = descriptionElement.Html()
			if err != nil {
				fmt.Printf("Не получилось получить описание элемента %s в формате html (%s).\n", articul, linkTo)
				description = descriptionElement.Text()
			}
		}

		technicalAttr := getRuntecShopTechnicalAttr(e.DOM)

		item := &ExternalItem{
			Articul:       articul,
			Description:   ReplaceMultiSpaces(sanitizer.SkipElementsContent("br").Sanitize(description)),
			Images:        downloadedImages,
			LinkTo:        linkTo,
			Name:          name,
			TechnicalAttr: ReplaceMultiSpaces(sanitizer.SkipElementsContent("br").Sanitize(technicalAttr)),
		}

		itemsToSaveChan <- rxgo.Item{V: item}
	})
}

func getRuntecShopArticul(root *goquery.Selection) string {
	articul := strings.TrimSpace(root.Find(".item-pay-box-mob .art").First().Text())
	if articul == "" {
		articul = strings.TrimSpace(root.Find(".art_avail .art").First().Text())
	}
	if articul == "" {
		articul = strings.TrimSpace(root.Find(".art").First().Text())
	}

	articul = strings.TrimSpace(strings.TrimPrefix(articul, "Арт:"))
	return ReplaceMultiSpaces(articul)
}

func getRuntecShopImageLinks(root *goquery.Selection) []string {
	seen := map[string]bool{}
	imagesLinks := make([]string, 0)

	root.Find(".properties-card-container_slider img, .mobile.pics-slider img").Each(func(i int, s *goquery.Selection) {
		src := strings.TrimSpace(s.AttrOr("src", ""))
		if src == "" || !strings.Contains(src, "/upload/") {
			return
		}

		fullLink := GetValidLinkOr(src, baseRuntecShopUrl, src)
		uniqueKey := GetRuntecShopOriginalImageLink(fullLink)
		if uniqueKey == "" {
			uniqueKey = fullLink
		}

		if seen[uniqueKey] {
			return
		}
		seen[uniqueKey] = true
		imagesLinks = append(imagesLinks, fullLink)
	})

	return imagesLinks
}

func findRuntecShopTab(root *goquery.Selection, tabName string) *goquery.Selection {
	var found *goquery.Selection
	root.Find(".tab_content > div").EachWithBreak(func(i int, s *goquery.Selection) bool {
		vShow := s.AttrOr("v-show", "")
		if strings.Contains(vShow, "'"+tabName+"'") || strings.Contains(vShow, `"`+tabName+`"`) {
			found = s
			return false
		}
		return true
	})
	return found
}

func getRuntecShopTechnicalAttr(root *goquery.Selection) string {
	propertiesList := root.Find("#properties-list .prop_item")
	if propertiesList.Length() == 0 {
		propertiesList = root.Find(".produt_props .flex-nowrap")
	}

	var builder strings.Builder
	propertiesList.Each(func(i int, s *goquery.Selection) {
		key := ReplaceMultiSpaces(strings.TrimSuffix(strings.TrimSpace(s.Find(".prop_key").First().Text()), ":"))
		if key == "" {
			key = ReplaceMultiSpaces(strings.TrimSuffix(strings.TrimSpace(s.Children().First().Text()), ":"))
		}

		value := ReplaceMultiSpaces(strings.TrimSpace(s.Children().Last().Text()))
		if key == "" || value == "" {
			return
		}

		builder.WriteString("<p>")
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(value)
		builder.WriteString("</p>")
	})

	if builder.Len() > 0 {
		return builder.String()
	}

	technicalAttrElement := findRuntecShopTab(root, "properties")
	if technicalAttrElement == nil {
		return ""
	}

	technicalAttr, err := technicalAttrElement.Html()
	if err != nil {
		return technicalAttrElement.Text()
	}
	return technicalAttr
}

func getRuntecShopNextPageHref(root *goquery.Selection) string {
	activePageFound := false
	nextHref := ""

	root.Find(".runtec_pagination .pages a.page_number").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if activePageFound {
			nextHref = strings.TrimSpace(s.AttrOr("href", ""))
			return false
		}

		class := " " + s.AttrOr("class", "") + " "
		if strings.Contains(class, " active ") {
			activePageFound = true
		}

		return true
	})

	if nextHref != "" {
		return nextHref
	}

	nextHref = strings.TrimSpace(root.Find(".load_more").AttrOr("data-url", ""))
	return nextHref
}

func DownloadRuntecShopImageIfNeed(imgLink string, params *ParserParams) (string, error) {
	fullLink := GetValidLinkOr(imgLink, baseRuntecShopUrl, imgLink)
	originalLink := GetRuntecShopOriginalImageLink(fullLink)

	if params.WithoutImages {
		if originalLink != "" {
			return originalLink, nil
		}
		return fullLink, nil
	}

	linkToDownload := fullLink
	if originalLink != "" && originalLink != fullLink && runtecShopImageExists(originalLink) {
		linkToDownload = originalLink
	}

	return DownloadImageIfNeed(linkToDownload, params, baseRuntecShopUrl)
}

func GetRuntecShopOriginalImageLink(imgLink string) string {
	parsedLink, err := url.Parse(imgLink)
	if err != nil {
		return ""
	}

	segments := strings.Split(parsedLink.Path, "/")
	for i, segment := range segments {
		if segment != "resize_cache" || i+4 >= len(segments) || segments[i+1] != "iblock" {
			continue
		}

		originalSegments := append([]string{}, segments[:i]...)
		originalSegments = append(originalSegments, "iblock", segments[i+2], segments[len(segments)-1])
		parsedLink.Path = strings.Join(originalSegments, "/")
		return parsedLink.String()
	}

	return imgLink
}

func runtecShopImageExists(imgLink string) bool {
	client := http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodHead, imgLink, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices && strings.HasPrefix(contentType, "image/")
}

func retryRuntecShopRequest(response *colly.Response, requestErr error) bool {
	if response == nil || response.Request == nil || !shouldRetryRuntecShopRequest(response) {
		return false
	}

	retry, _ := response.Request.Ctx.GetAny(runtecShopRetryKey).(int)
	if retry >= runtecShopMaxRetries {
		return false
	}

	retry++
	response.Request.Ctx.Put(runtecShopRetryKey, retry)

	delay := time.Duration(retry) * 2 * time.Second
	fmt.Printf(
		"Временная ошибка Runtec для %s: status=%d err=%v. Повторная попытка %d/%d через %s.\n",
		response.Request.URL.String(),
		response.StatusCode,
		requestErr,
		retry,
		runtecShopMaxRetries,
		delay,
	)
	time.Sleep(delay)

	err := response.Request.Retry()
	if err != nil {
		fmt.Printf("Не удалось повторить запрос %s: %s\n", response.Request.URL.String(), err)
		return false
	}

	return true
}

func shouldRetryRuntecShopRequest(response *colly.Response) bool {
	return response.StatusCode == 0 ||
		response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode >= http.StatusInternalServerError
}
