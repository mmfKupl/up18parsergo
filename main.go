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

func main() {
	parserParams := initParserParams()
	err := initParser(parserParams)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fileName, err := downloadImageIfNeed("https://up18.by/images/new_product/Fein/71293362000.jpg", parserParams)
	if err != nil {
		fmt.Println("Image downloading failed")
	}
	fmt.Println("fileName - " + fileName)
	// url := "https://up18.by/brends/makita"
	//
	// c := colly.NewCollector(colly.AllowedDomains("up18.by"))
	//
	// c.OnHTML(".pagination span + a", func(e *colly.HTMLElement) {
	// 	fmt.Println(e.Text)
	// 	fmt.Println(e.Attr("href"))
	// 	c.Visit(e.Attr("href"))
	// })
	//
	// c.OnRequest(func(r *colly.Request) {
	// 	fmt.Printf("Visit %s\n", r.URL.String())
	// })
	//
	// c.OnResponse(func(r *colly.Response) {
	// 	fmt.Printf("Responce %+v", r.StatusCode)
	// })
	//
	// err := c.Visit(url)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	//
	// c.Wait()
	// fmt.Println("End.")
}

func downloadImageIfNeed(url string, params *ParserParams) (string, error) {

	fullUrl, err := getValidLink(url)
	if err != nil {
		return "", err
	}
	imageName := getImageNameFromUrl(fullUrl)
	imagePath := path.Join(params.ImagesFolderPath, imageName)

	if _, err := os.Stat(imagePath); err == nil {
		fmt.Println("FileExist")
		return imageName, nil
	}
	fmt.Println("File Not Exist")

	res, err := http.Get(fullUrl)
	if err != nil {
		return fullUrl, err
	}
	defer res.Body.Close()

	file, err := os.Create(imagePath)
	if err != nil {
		return fullUrl, err
	}
	defer file.Close()

	_, err = io.Copy(file, res.Body)
	if err != nil {
		return fullUrl, err
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
