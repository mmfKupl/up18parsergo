package parser

import (
	"flag"
	"fmt"
	"os"
)

const (
	InternalParserMode Mode = "internal"
	ExternalParserMode Mode = "external"

	DefaultParserMode = InternalParserMode
)

const (
	ParserModeArg              = "mode"
	ParserModeArg__description = "Устанавливает режим парсера:\n" +
		"'internal' (по умолчанию) - для ссылок с up18.by для дальнейшей проверки наличия на сайте,\n" +
		"'external' - для ссылок для дальнейшей загрузки в каталог.\n"

	urlToParseArg              = "url"
	urlToParseArg__short       = "u"
	urlToParseArg__description = "Ссылка на страницу, с которой будут собраны данные.\n"

	urlsToParsePathArg              = "urlsToParse"
	urlsToParsePathArg__short       = "utp"
	urlsToParsePathArg__description = "Путь к json файлу, в котором лежит массив ссылок, которых нужно распарсить.\n"

	DefaultDataFilePath          = "data.json"
	dataFilePathArg              = "fileName"
	dataFilePathArg__short       = "fn"
	dataFilePathArg__description = "(по умолчанию `data.json`) Название файла, в который будет загружена скачанная информация.\n"

	DefaultImagesFolderPath          = "files"
	imagesFolderPathArg              = "folder"
	imagesFolderPathArg__short       = "f"
	imagesFolderPathArg__description = "(по умолчанию `files`) Название папки куда будут скачаны картинки.\n"

	DefaultWithoutImages          = false
	withoutImagesArg              = "withoutImages"
	withoutImagesArg__short       = "wi"
	withoutImagesArg__description = "(по умолчанию false) Указывает нужно ли скачивать картинки или нет.\n"
)

func NewParserParams() *ParserParams {
	return &ParserParams{
		ParserMode:       DefaultParserMode,
		UrlToParse:       "",
		ImagesFolderPath: DefaultImagesFolderPath,
		DataFilePath:     DefaultDataFilePath,
		WithoutImages:    DefaultWithoutImages,
	}
}

func initParserParams(parentParserMode Mode) *ParserParams {
	// parserModeRef := flag.String(parserModeArg, string(DefaultParserMode), parserModeArg__description)

	urlToParseShortRef := flag.String(urlToParseArg__short, "", urlToParseArg__description)
	urlToParseLongRef := flag.String(urlToParseArg, "", urlToParseArg__description)

	urlsToParsePathLongRef := flag.String(urlsToParsePathArg, "", urlsToParsePathArg__description)
	urlsToParsePathShortRef := flag.String(urlsToParsePathArg__short, "", urlsToParsePathArg__description)

	dataFilePathLongRef := flag.String(dataFilePathArg, DefaultDataFilePath, dataFilePathArg__description)
	dataFilePathShortRef := flag.String(dataFilePathArg__short, DefaultDataFilePath, dataFilePathArg__description)

	imagesFolderPathLongRef := flag.String(imagesFolderPathArg, DefaultImagesFolderPath, imagesFolderPathArg__description)
	imagesFolderPathShortRef := flag.String(imagesFolderPathArg__short, DefaultImagesFolderPath, imagesFolderPathArg__description)

	withoutImageLongRef := flag.Bool(withoutImagesArg, DefaultWithoutImages, withoutImagesArg__description)
	withoutImageShortRef := flag.Bool(withoutImagesArg__short, DefaultWithoutImages, withoutImagesArg__description)

	flag.Parse()

	params := NewParserParams()

	// parserMode := ParserMode(*parserModeRef)
	if parentParserMode != "" {
		params.ParserMode = parentParserMode
	}
	// if parserMode == InternalParserMode || parserMode == ExternalParserMode {
	// 	params.ParserMode = parserMode
	// }

	if *urlToParseShortRef != "" {
		params.UrlToParse = *urlToParseShortRef
	}
	if *urlToParseLongRef != "" {
		params.UrlToParse = *urlToParseLongRef
	}

	if *urlsToParsePathShortRef != "" {
		params.UrlsToParsePath = *urlsToParsePathShortRef
	}
	if *urlsToParsePathLongRef != "" {
		params.UrlsToParsePath = *urlsToParsePathLongRef
	}

	if *dataFilePathLongRef != "" {
		params.DataFilePath = *dataFilePathLongRef
	}
	if *dataFilePathShortRef != "" {
		params.DataFilePath = *dataFilePathShortRef
	}

	if *imagesFolderPathLongRef != "" {
		params.ImagesFolderPath = *imagesFolderPathLongRef
	}
	if *imagesFolderPathShortRef != "" {
		params.ImagesFolderPath = *imagesFolderPathShortRef
	}

	if *withoutImageShortRef != *withoutImageLongRef {
		if params.WithoutImages != *withoutImageShortRef {
			params.WithoutImages = *withoutImageShortRef
		} else {
			params.WithoutImages = *withoutImageLongRef
		}
	} else {
		params.WithoutImages = *withoutImageLongRef
	}

	return params
}

func initParser(params *ParserParams) error {
	if params.UrlToParse == "" && params.UrlsToParsePath == "" {
		return fmt.Errorf("Не заданы ссылки для скачивания. (за информацией запустите программу с флагом -h. и прочитайте про флаги  -url и -utp). ")
	}

	if _, err := os.Stat(BaseFolderToSave); os.IsNotExist(err) {
		err = os.Mkdir(BaseFolderToSave, 0777)
		if err != nil {
			return fmt.Errorf("неудалось создать базовую папку `%s`: %s", BaseFolderToSave, err)
		}
	}

	folderPath := GetValidPath(params.ImagesFolderPath)

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		err = os.Mkdir(folderPath, 0777)
		if err != nil {
			return fmt.Errorf("неудалось создать папку с картинками `%s`: %s", folderPath, err)
		}
	}

	dataFilePath := GetValidPath(params.DataFilePath)

	if _, err := os.Stat(dataFilePath); os.IsNotExist(err) {
		_, err = os.Create(dataFilePath)
		if err != nil {
			return fmt.Errorf("неудалось создать файл с данными `%s`: %s", dataFilePath, err)
		}
	}

	return nil
}
