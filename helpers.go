package parser

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gocolly/colly/v2"
)

const BaseFolderToSave = "data"

func GetValidPath(elem ...string) string {
	items := []string{BaseFolderToSave}
	return path.Join(append(items, elem...)...)
}

func DownloadImageIfNeed(url string, params *ParserParams, base url.URL) (string, error) {

	fullUrl, err := GetValidLink(url, base)
	if err != nil {
		return "", err
	}
	imageName := GetImageNameFromUrl(fullUrl)
	imagePath := GetValidPath(params.ImagesFolderPath, imageName)

	if _, err := os.Stat(imagePath); err == nil {
		return imageName, nil
	}

	res, err := http.Get(fullUrl)
	if err != nil {
		return fullUrl, fmt.Errorf("faild to get image by url: %s", err)
	}
	defer res.Body.Close()

	file, err := os.Create(imagePath)
	if err != nil {
		return fullUrl, fmt.Errorf("faild to create image file `%s`: %s", imagePath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, res.Body)
	if err != nil {
		return fullUrl, fmt.Errorf("faild to write image to file: %s", err)
	}

	return imageName, nil
}

func GetImageNameFromUrl(url string) string {
	split := strings.Split(url, "/")
	return split[len(split)-1]
}

func GetValidLink(link string, base url.URL) (string, error) {
	linkUrl, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(linkUrl).String(), nil
}

func GetValidLinkOr(link string, base url.URL, or string) string {
	validLink, err := GetValidLink(link, base)
	if err != nil {
		return or
	}
	return validLink
}

func CreateAndGetFile(path string, flag int) (*os.File, error) {
	validPath := path
	return os.OpenFile(validPath, flag|os.O_CREATE, 0777)
}

func LogPageVisiting(c *colly.Collector) {
	c.OnError(func(response *colly.Response, err error) {
		fmt.Printf("Ошибка запроса на страницу %s: %s\n", response.Request.URL.String(), err)
	})
	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("Парсим следующую страницу - %s\n", r.URL.String())
	})
}
