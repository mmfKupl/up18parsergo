package main

import (
	"net/url"
	"os"
	"strings"
)

var baseUp18Url = url.URL{
	Scheme: "https",
	Host:   "up18.by",
}

const (
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
		WithoutImages:    true,
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

	if _, err := os.Stat(params.ImagesFolderPath); os.IsNotExist(err) {
		err = os.Mkdir(params.ImagesFolderPath, 0777)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(params.DataFilePath); os.IsNotExist(err) {
		_, err = os.Create(params.DataFilePath)
		if err != nil {
			return err
		}
	}

	return nil
}
