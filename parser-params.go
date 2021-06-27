package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

var baseUp18Url = url.URL{
	Scheme: "https",
	Host:   "up18.by",
}

const (
	baseFolderToSave = "data"

	urlsToParsePathArg        = "--urlsToParse="
	urlsToParsePathArg__short = "--utp="

	urlToParseArg        = "--url="
	urlToParseArg__short = "--u="

	imagesFolderPathArg        = "--folder="
	imagesFolderPathArg__short = "--f="

	dataFilePathArg        = "--fileName="
	dataFilePathArg__short = "--fn="

	withoutImagesArg        = "--withoutImages"
	withoutImagesArg__short = "--wi"
)

type ParserParams struct {
	UrlsToParsePath  string
	UrlToParse       string
	ImagesFolderPath string
	DataFilePath     string
	WithoutImages    bool
}

func NewParserParams() *ParserParams {
	return &ParserParams{
		UrlToParse:       "https://up18.by/brends/toya/",
		ImagesFolderPath: "files",
		DataFilePath:     "data.json",
		WithoutImages:    false,
	}
}

func initParserParams() *ParserParams {
	params := NewParserParams()
	validArguments := os.Args[1:]

	for _, value := range validArguments {
		attrName, attrValue := parseArgument(value)

		switch attrName {
		case urlsToParsePathArg, urlsToParsePathArg__short:
			params.UrlsToParsePath = attrValue
		case urlToParseArg, urlToParseArg__short:
			params.UrlToParse = attrValue
		case imagesFolderPathArg, imagesFolderPathArg__short:
			params.ImagesFolderPath = attrValue
		case dataFilePathArg, dataFilePathArg__short:
			params.DataFilePath = attrValue
		case withoutImagesArg, withoutImagesArg__short:
			params.WithoutImages = true
		}
	}

	return params
}

func parseArgument(argument string) (string, string) {

	splitArgument := strings.SplitAfter(argument, "=")
	partsAmount := len(splitArgument)

	if 0 == partsAmount {
		return "", ""
	}
	if 1 == partsAmount {
		return splitArgument[0], ""
	}

	return splitArgument[0], splitArgument[1]
}

func initParser(params *ParserParams) error {

	if _, err := os.Stat(baseFolderToSave); os.IsNotExist(err) {
		err = os.Mkdir(baseFolderToSave, 0777)
		if err != nil {
			return fmt.Errorf("неудалось создать базовую папку `%s`: %s", baseFolderToSave, err)
		}
	}

	folderPath := getValidPath(params.ImagesFolderPath)

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		err = os.Mkdir(folderPath, 0777)
		if err != nil {
			return fmt.Errorf("неудалось создать папку с картинками `%s`: %s", folderPath, err)
		}
	}

	dataFilePath := getValidPath(params.DataFilePath)

	if _, err := os.Stat(dataFilePath); os.IsNotExist(err) {
		_, err = os.Create(dataFilePath)
		if err != nil {
			return fmt.Errorf("неудалось создать файл с данными `%s`: %s", dataFilePath, err)
		}
	}

	return nil
}
