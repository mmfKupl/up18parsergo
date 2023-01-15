package main

import parser "sitesParsers"

func main() {
	parser.StartParser(parser.ExternalParserMode, "japan-ukraine.com")
}
