package parser

type Mode string

type Item interface {
	GetLink() string
	GetId() string
}

type ParserParams struct {
	ParserMode       Mode
	UrlsToParsePath  string
	UrlToParse       string
	ImagesFolderPath string
	DataFilePath     string
	WithoutImages    bool
}

type InternalItem struct {
	Artikul   string `json:"artikul"`
	Image     string `json:"image"`
	ItemTitle string `json:"itemTitle"`
	LinkTo    string `json:"linkTo"`
	Price     string `json:"price"`
}

func (ii *InternalItem) GetLink() string {
	return ii.LinkTo
}

func (ii *InternalItem) GetId() string {
	return ii.Artikul
}
