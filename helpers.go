package parser

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
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

func CreateAndGetFile(path string, flag int) (*os.File, error) {
	validPath := path
	return os.OpenFile(validPath, flag|os.O_CREATE, 0777)
}
