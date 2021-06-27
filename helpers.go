package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

func getValidPath(elem ...string) string {
	items := []string{baseFolderToSave}
	return path.Join(append(items, elem...)...)
}

func downloadImageIfNeed(url string, params *ParserParams) (string, error) {

	fullUrl, err := getValidLink(url)
	if err != nil {
		return "", err
	}
	imageName := getImageNameFromUrl(fullUrl)
	imagePath := getValidPath(params.ImagesFolderPath, imageName)

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

func getImageNameFromUrl(url string) string {
	split := strings.Split(url, "/")
	return split[len(split)-1]
}

func getValidLink(link string) (string, error) {
	linkUrl, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	return baseUp18Url.ResolveReference(linkUrl).String(), nil
}

func createAndGetFile(path string, flag int) (*os.File, error) {
	validPath := path
	return os.OpenFile(validPath, flag|os.O_CREATE, 0777)
}
