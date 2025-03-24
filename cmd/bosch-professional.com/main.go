package main

import parser "sitesParsers"

func main() {
	parser.StartParser(parser.ExternalParserMode, "bosch-professional.com")
}
