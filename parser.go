package parser

import (
	"fmt"
	"os"

	"github.com/microcosm-cc/bluemonday"
)

var sanitizer = bluemonday.UGCPolicy().SkipElementsContent("a")

func StartParser(parserMode Mode, externalType string) {
	defer fmt.Println("Конец.")
	parserParams := initParserParams(parserMode)
	err := initParser(parserParams)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// TODO: make other parsers in other folders
	switch parserParams.ParserMode {
	case InternalParserMode:
		StartUp18Parser(parserParams)
	case ExternalParserMode:
		switch externalType {
		case "makita.ru":
			StartMakitaParser(parserParams)
		case "bashmaistora.bg":
			StartBashmaistoraParser(parserParams)
		case "japan-ukraine.com":
			JapanUkraineParser(parserParams)
		case "tpro.by":
			StartTproParser(parserParams)
		case "dw_st_bad.ru":
			StartDW_ST_BADParser(parserParams)
		case "dw4you.ru":
			StartDW4YouParser(parserParams)
		default:
			fmt.Printf("Такой сайт пока еще не распарсить")
			os.Exit(1)
		}
	}
}
