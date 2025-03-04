package DataTypes

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BookPartial struct {
	ISBN      string
	Price     string
	Available string
}

type BookToUpdate struct {
	ISBN                string
	Price               string
	PriceChanged        bool
	Available           string
	AvailabilityChanged bool
}

type MondadoriBookDocument struct {
	ISBN           string             `bson:"ISBN"`
	Published      bool               `bson:"Published"`      // TODO remove
	PublishedPrice string             `bson:"PublishedPrice"` // TODO remove
	Title          string             `bson:"Title"`
	Available      string             `bson:"Available"`
	Price          string             `bson:"Price"`
	URL            string             `bson:"URL"` // TODO remove
	ImageURL       string             `bson:"ImageURL"`
	Author         string             `bson:"Author"`
	Category       string             `bson:"Category"` // TODO remove
	Editor         string             `bson:"Editor"`
	Variant        string             `bson:"Variant"`
	Language       string             `bson:"Language"`
	UpdatedAt      primitive.DateTime `bson:"UpdatedAt"`    // TODO remove
	ListingId      string             `bson:"ListingId"`    // TODO remove
	OfferId        string             `bson:"OfferId"`      // TODO remove
	EbayImageUrl   string             `bson:"EbayImageUrl"` // TODO remove
	PublishedFrom  string             `bson:"PublishedFrom"`
	Pages          int                `bson:"Pages"`
	Description    string             `bson:"Description"`
	Series         string             `bson:"Series"`
	Categories     []string           `bson:"Categories"`
}

type MondadoriBook struct {
	ISBN           string
	Published      bool // TODO remove
	Title          string
	Available      string
	Price          string
	URL            string // TODO remove
	ImageURL       string
	Author         string
	Category       string // TODO remove
	PublishedPrice string // TODO remove
	Variant        string
	Editor         string
	Language       string
	PublishedFrom  string
	Pages          int
	Description    string
	Series         string
	Categories     []string
}

type UrlSet struct {
	URLs []URL `xml:"url"`
}

type URL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type MondadoriSitemapItemFile struct {
	MondadoriSitemapItem []MondadoriSitemapItem `xml:"url"`
}

type MondadoriSitemapItem struct {
	Loc string `xml:"loc"`
}

// Sitemap represents a single <sitemap> element in the XML.
type Sitemap struct {
	Loc string `xml:"loc"`
}

// SitemapIndex represents the root <sitemapindex> element in the XML.
type SitemapIndex struct {
	Sitemaps []Sitemap `xml:"sitemap"`
}

type Promo struct {
	ID              int    `json:"id"`
	CatalogMessage  string `json:"catalog_message"`
	IsPriceHidden   bool   `json:"is_price_hidden"`
	OutputSmartlist int    `json:"output_smartlist"`
	PriceMessage    string `json:"price_message"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	IsPromoTime     bool   `json:"is_promo_time"`
}

type InventoryJSON struct {
	IsCurrentlySellableOnIbs bool        `json:"IsCurrentlySellableOnIbs"`
	IsTooFarAvailable        bool        `json:"IsTooFarAvailable"`
	IsNextToTodayAvailable   bool        `json:"IsNextToTodayAvailable"`
	HasPublicationDate       bool        `json:"HasPublicationDate"`
	HasFuturePublicationDate bool        `json:"HasFuturePublicationDate"`
	HasInventoryPromotions   bool        `json:"HasInventoryPromotions"`
	HasInventoryDiscount     bool        `json:"HasInventoryDiscount"`
	IsDiscountAvarageVisible bool        `json:"IsDiscountAvarageVisible"`
	ShippingCharges          json.Number `json:"ShippingCharges"`
	InventoryDiscount        float64     `json:"InventoryDiscount"`
	Price                    json.Number `json:"Price"`
	IsGift                   bool        `json:"IsGift"`
	FidelityPoints           int         `json:"FidelityPoints"`
	SaleStartDate            string      `json:"sale_start_date"`
	PublicationDate          string      `json:"publication_date"`
	Promo                    []Promo     `json:"promo"`
	Status                   int         `json:"status"`
	QuantityWarehouse        int         `json:"quantity_warehouse"`
	SmartListID              []int       `json:"smart_list_id"`
	IsAvailable              bool        `json:"IsAvailable"`
	MaxSellableQuantity      int         `json:"MaxSellableQuantity"`
}

type AvailabilityJSON struct {
	Text string `json:"Text"`
}

type BuyInfos struct {
	ISBN         string
	Price        json.Number `json:"Price"`
	Availability string      `json:"Text"`
	Title        string
	URL          string
}

type DescriptionData struct {
	ShortDescription string
	LongDescription  string
}

type FeltrinelliScrapedBook struct {
	BuyInfos        BuyInfos
	DescriptionData DescriptionData
	Category        string
	Details         map[string]string
}

type EbayData struct {
	ISBN         string `bson:"ISBN"`
	Published    bool   `bson:"Published"`
	ListingId    string `bson:"ListingId"`
	OfferId      string `bson:"OfferId"`
	EbayImageURL string `bson:"EbayImageURL"`
}

type Details struct {
	AnnoEdizione    string `bson:"Anno edizione"`
	Autore          string `bson:"Autore"`
	Collana         string `bson:"Collana"`
	Curatore        string `bson:"Curatore"`
	Editore         string `bson:"Editore"`
	Edizione        string `bson:"Edizione"`
	EtaDiLettura    string `bson:"Età di lettura"`
	Formato         string `bson:"Formato"`
	Illustratore    string `bson:"Illustratore"`
	InCommercioDal  string `bson:"In commercio dal"`
	Pagine          string `bson:"Pagine"`
	Tipo            string `bson:"Tipo"`
	TitoloOriginale string `bson:"Titolo originale"`
	Traduttore      string `bson:"Traduttore"`
}

type FeltrinelliBook struct {
	ISBN             string  `bson:"ISBN"`
	Availability     string  `bson:"Availability"`
	Details          Details `bson:"Details"`
	LongDescription  string  `bson:"LongDescription"`
	ShortDescription string  `bson:"ShortDescription"`
	Price            string  `bson:"Price"`
	Title            string  `bson:"Title"`
	URL              string  `bson:"URL"`
	Category         string  `bson:"Category"`
}

type EbayDataWithFeltrinelliBook struct {
	EbayData        EbayData        `bson:"EbayData"`
	FeltrinelliBook FeltrinelliBook `bson:"FeltrinelliBook"`
}

type EbayDataWithMondadoriBook struct {
	EbayData      EbayData      `bson:"EbayData"`
	MondadoriBook MondadoriBook `bson:"MondadoriBook"`
}

type URLDocument struct {
	URL string `bson:"URL"`
}

type MongoDBFeltrinelliProduct struct {
	URL string `bson:"URL"`
	EAN string `bson:"EAN"`
}

type MongoDBMondadoriProduct struct {
	URL  string `bson:"URL"`
	ISBN string `bson:"ISBN"`
}

type Results struct {
	ProxyAddress string `json:"proxy_address"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

type ProxyListResponse struct {
	Count   int       `json:"count"`
	Results []Results `json:"results"`
}

type Category int

const (
	Saggistica         Category = 171243
	Narrativa          Category = 171228
	LibriDiTesto       Category = 171223
	CucinaEGastronomia Category = 11104
	BambiniERagazzi    Category = 171219
	CorsiDiLingua      Category = 11442
)

type EbayBook struct {
	ISBN          string   `bson:"ISBN"`
	CategoryID    Category `bson:"CategoryID"`
	Available     bool     `bson:"Available"`
	Editor        string   `bson:"Editor"`
	Author        string   `bson:"Author"`
	Variant       string   `bson:"Variant"`
	Language      string   `bson:"Language"`
	Price         string   `bson:"Price"`
	Title         string   `bson:"Title"`
	PublishedFrom string   `bson:"PublishedFrom"`
	Pages         int      `bson:"Pages"`
	MarketIn      string   `bson:"MarketIn"`
}

func (b EbayBook) Equals(other EbayBook) bool {
	return b.ISBN == other.ISBN &&
		b.CategoryID == other.CategoryID &&
		b.Available == other.Available &&
		b.Editor == other.Editor &&
		b.Author == other.Author &&
		b.Variant == other.Variant &&
		b.Language == other.Language &&
		b.Price == other.Price &&
		b.Title == other.Title &&
		b.PublishedFrom == other.PublishedFrom &&
		b.Pages == other.Pages &&
		b.MarketIn == other.MarketIn
}
