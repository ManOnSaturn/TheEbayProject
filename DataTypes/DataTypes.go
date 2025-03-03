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
	ISBN          string             `bson:"ISBN"`
	Published     bool               `bson:"Published"` // TODO remove
	Title         string             `bson:"Title"`
	Available     string             `bson:"Available"`
	Price         string             `bson:"Price"`
	URL           string             `bson:"URL"` // TODO remove
	ImageURL      string             `bson:"ImageURL"`
	Author        string             `bson:"Author"`
	Category      string             `bson:"Category"` // TODO remove
	Editor        string             `bson:"Editor"`
	Variant       string             `bson:"Variant"`
	Language      string             `bson:"Language"`
	UpdatedAt     primitive.DateTime `bson:"UpdatedAt"` // TODO remove
	PublishedFrom string             `bson:"PublishedFrom"`
	Pages         int                `bson:"Pages"`
	Description   string             `bson:"Description"`
	Series        string             `bson:"Series"`
	Categories    []string           `bson:"Categories"`
}

type MondadoriBook struct {
	ISBN          string
	Published     bool // TODO remove
	Title         string
	Available     string
	Price         string
	URL           string // TODO remove
	ImageURL      string
	Author        string
	Category      string // TODO remove
	Variant       string
	Editor        string
	Language      string
	PublishedFrom string
	Pages         int
	Description   string
	Series        string
	Categories    []string
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
	ISBN           string `bson:"ISBN"`
	PublishedPrice string `bson:"PublishedPrice"`
	Published      bool   `bson:"Published"`
	ListingId      string `bson:"ListingId"`
	OfferId        string `bson:"OfferId"`
	EbayImageURL   string `bson:"EbayImageURL"`
	MarketIn       string `bson:"MarketIn"`
}

type Details struct {
	AnnoEdizione    string `json:"anno_edizione"`
	Autore          string `json:"autore"`
	Collana         string `json:"collana"`
	Curatore        string `json:"curatore"`
	Editore         string `json:"editore"`
	Edizione        string `json:"edizione"`
	EtaDiLettura    string `json:"eta_di_lettura"`
	Formato         string `json:"formato"`
	Illustratore    string `json:"illustratore"`
	InCommercioDal  string `json:"in_commercio_dal"`
	Pagine          string `json:"pagine"`
	Tipo            string `json:"tipo"`
	TitoloOriginale string `json:"titolo_originale"`
	Traduttore      string `json:"traduttore"`
}

type FeltrinelliBook struct {
	ISBN             string  `json:"isbn"`
	Availability     string  `json:"availability"`
	Details          Details `json:"details"`
	LongDescription  string  `json:"long_description"`
	ShortDescription string  `json:"short_description"`
	Price            string  `json:"price"`
	Title            string  `json:"title"`
	URL              string  `json:"url"`
	Category         string  `json:"category"`
}

type EbayDataWithFeltrinelliBook struct {
	EbayData        EbayData        `bson:"EbayData"`
	FeltrinelliBook FeltrinelliBook `bson:"FeltrinelliBook"`
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
