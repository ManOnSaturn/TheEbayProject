package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Structs to match the XML structure
type UrlSet struct {
	URLs []URL `xml:"url"`
}

type URL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type Product struct {
	URL string
	EAN string
}

type Promo struct {
	ID              int    `json:"id"`
	CatalogMessage  string `json:"catalog_message"`
	IsPriceHidden   string `json:"is_price_hidden"`
	OutputSmartlist int    `json:"output_smartlist"`
	PriceMessage    string `json:"price_message"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	IsPromoTime     bool   `json:"is_promo_time"`
}

type ProductJSON struct {
	IsCurrentlySellableOnIbs bool    `json:"IsCurrentlySellableOnIbs"`
	IsTooFarAvailable        bool    `json:"IsTooFarAvailable"`
	IsNextToTodayAvailable   bool    `json:"IsNextToTodayAvailable"`
	HasPublicationDate       bool    `json:"HasPublicationDate"`
	HasFuturePublicationDate bool    `json:"HasFuturePublicationDate"`
	HasInventoryPromotions   bool    `json:"HasInventoryPromotions"`
	HasInventoryDiscount     bool    `json:"HasInventoryDiscount"`
	IsDiscountAvarageVisible bool    `json:"IsDiscountAvarageVisible"`
	ShippingCharges          int     `json:"ShippingCharges"`
	InventoryDiscount        float64 `json:"InventoryDiscount"`
	Price                    int     `json:"Price"`
	IsGift                   bool    `json:"IsGift"`
	FidelityPoints           int     `json:"FidelityPoints"`
	SaleStartDate            string  `json:"sale_start_date"`
	PublicationDate          string  `json:"publication_date"`
	Promo                    []Promo `json:"promo"`
	Status                   int     `json:"status"`
	QuantityWarehouse        int     `json:"quantity_warehouse"`
	SmartListID              []int   `json:"smart_list_id"`
	IsAvailable              bool    `json:"IsAvailable"`
	MaxSellableQuantity      int     `json:"MaxSellableQuantity"`
}

func downloadAndParseXML(url string) (*UrlSet, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download: %s (status code: %d)", url, resp.StatusCode)
	}

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the XML
	var urlSet UrlSet
	err = xml.Unmarshal(data, &urlSet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}
	fmt.Println("Successfully parsed", url)
	return &urlSet, nil
}

func getEAN(s string) string {
	lastSlashIndex := strings.LastIndex(s, "/")

	if lastSlashIndex != -1 {
		return s[lastSlashIndex+1:]
	} else {
		panic("No '/' found in the string.")
	}
}

func scrapeAllXMLs(client *mongo.Client) {
	baseURL := "https://www.lafeltrinelli.it/sitemap_itbook_"
	urlSets := make([]UrlSet, 0)

	for i := 1; i <= 63; i++ {
		// Construct the URL
		url := baseURL + strconv.Itoa(i) + ".xml"

		// Download and parse the XML file
		urlSet, err := downloadAndParseXML(url)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		urlSets = append(urlSets, *urlSet)
	}
	var models []mongo.WriteModel

	for urlSetsIndex, urlSet := range urlSets {
		for _, entry := range urlSet.URLs {
			filter := bson.M{"URL": entry.Loc}

			// Parse the string into time.Time
			lastMod, err := time.Parse(time.RFC3339, entry.LastMod)
			if err != nil {
				log.Fatalf("Failed to parse date: %v", err)
			}

			// Create the update document
			update := bson.M{
				"$set": bson.M{
					"URL":     entry.Loc,
					"LastMod": lastMod,
					"EAN":     getEAN(entry.Loc),
				},
			}

			// Create an UpdateOneModel with upsert option
			model := mongo.NewUpdateOneModel().
				SetFilter(filter).
				SetUpdate(update).
				SetUpsert(true)

			models = append(models, model)

			if len(models) >= 10000 {
				bulkWriteToMongo(client, models)
				models = make([]mongo.WriteModel, 0)
				fmt.Println("Processed 10000 XML entries into the DB.")
				fmt.Println("Working URLSet index", urlSetsIndex, "out of", len(urlSets))
			}
		}
	}

	// Execute remaining models in bulk
	if len(models) > 0 {
		bulkWriteToMongo(client, models)
	}
}

func bulkWriteToMongo(client *mongo.Client, models []mongo.WriteModel) {
	collection := client.Database("Mondadori").Collection("FeltrinelliProducts")
	_, err := collection.BulkWrite(context.TODO(), models)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
}

func getNewProducts(client *mongo.Client, productsChan chan Product) {
	collection := client.Database("Mondadori").Collection("FeltrinelliProducts")

	// Filter for documents where the field 'IsBook' does not exist
	filter := bson.M{"IsBook": bson.M{"$exists": false}}

	cursor, err := collection.Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(context.TODO())

	// Iterate through the cursor and send documents to the channel
	for cursor.Next(context.TODO()) {
		var document bson.M
		if err := cursor.Decode(&document); err != nil {
			// Log the error but continue processing other documents
			log.Printf("Error decoding document: %v\n", err)
			continue
		}

		// Safely extract URL and EAN from the document
		url, okURL := document["URL"].(string)
		ean, okEAN := document["EAN"].(string)

		if !okURL || !okEAN {
			// Log if URL or EAN is missing or of incorrect type and continue
			log.Printf("Missing or invalid URL/EAN in document: %v\n", document)
			continue
		}

		// Send the decoded product to the channel
		productsChan <- Product{URL: url, EAN: ean}
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
	close(productsChan) // Close the channel when done
}

func getProductInfos(productsChan chan Product) {
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)

	// On every HTML element that matches the div with id "buy-box"
	//c.OnHTML("div#buy-box span.cc-price", func(e *colly.HTMLElement) {
	//	// Print the HTML content of the div
	//	fmt.Println("Buy Box Found:", e.Text, e.Request.URL.Host+e.Request.URL.Path)
	//})
	c.OnHTML("pdp-physical-buy-info", func(e *colly.HTMLElement) {
		// Print the HTML content of the div
		fmt.Println("Physical buy info found:", e.Attr(":availability"), e.Request.URL.Host+e.Request.URL.Path)
		var product ProductJSON
		err := json.Unmarshal([]byte(e.Attr(":inventory")), &product)
		if err != nil {
			fmt.Println("Error unmarshaling JSON:", err)
			return
		}
		fmt.Printf("%+v\n", product)
	})

	c.OnError(func(response *colly.Response, err error) {
		fmt.Println(response)
		fmt.Println(err)
	})

	for product := range productsChan {
		err := c.Visit(product.URL)
		if err != nil {
			fmt.Println(err)
		}
	}

	c.Wait()
}

func fullScrapeFeltrinelli() {
	client := connectToMongo()
	defer disconnectFromMongo(client)
	startTime := time.Now()

	//scrapeAllXMLs(client)

	productsChan := make(chan Product)
	go getNewProducts(client, productsChan)

	getProductInfos(productsChan)

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")
}
