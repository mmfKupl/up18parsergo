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

type ExternalItem struct {
	Articul       string `json:"articul"`
	Description   string `json:"description"`
	Image         string `json:"image"`
	LinkTo        string `json:"linkTo"`
	Name          string `json:"name"`
	TechnicalAttr string `json:"technical-attr"`
}

func (ii *ExternalItem) GetLink() string {
	return ii.LinkTo
}

func (ii *ExternalItem) GetId() string {
	return ii.Articul
}
