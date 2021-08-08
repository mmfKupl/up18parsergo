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
		default:
			StartDW_ST_BADParser(parserParams)
		}
	}
}
