package parser

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gocolly/colly/v2"
)

const BaseFolderToSave = "data"

func GetValidPath(elem ...string) string {
	items := []string{BaseFolderToSave}
	return path.Join(append(items, elem...)...)
}

func DownloadImageIfNeedInLowerRegister(url string, params *ParserParams, base url.URL) (string, error) {
	return DownloadNamedImageIfNeed(url, params, base, "", true)
}

func DownloadImageIfNeed(url string, params *ParserParams, base url.URL) (string, error) {
	return DownloadNamedImageIfNeed(url, params, base, "", false)
}

func DownloadNamedImageIfNeed(url string, params *ParserParams, base url.URL, imageName string, lower bool) (string, error) {
	fullUrl, err := GetValidLink(url, base)
	if err != nil {
		return "", err
	}
	if imageName == "" {
		imageName = GetImageNameFromUrl(fullUrl)
	} else {
		fileType := GetImageType(fullUrl)
		imageName += "." + fileType
	}

	if lower {
		imageName = strings.ToLower(imageName)
	}

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

func GetImageType(path string) string {
	split := strings.Split(path, ".")
	return split[len(split)-1]
}

func GetValidLink(link string, base url.URL) (string, error) {
	linkUrl, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(linkUrl).String()

	// Decode the path to keep non-ASCII characters
	unescaped, err := url.PathUnescape(resolved)
	if err != nil {
		return "", err
	}

	return unescaped, nil
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

func ReplaceMultiSpaces(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}
