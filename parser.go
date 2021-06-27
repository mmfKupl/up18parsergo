package parser

import (
	"fmt"
	"os"
)

func StartParser(parserMode Mode) {
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
		StartDewaltParser(parserParams)
	}
}
