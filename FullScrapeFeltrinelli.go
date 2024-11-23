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
	"sync"
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
	Text              string `json:"Text"`
	StickyDesktopText string `json:"AvailabilityStickyText"`
}

type BuyInfos struct {
	ISBN                   string
	Price                  json.Number `json:"Price"`
	Availability           string      `json:"Text"`
	AvailabilityStickyText string      `json:"AvailabilityStickyText"`
	Title                  string
	URL                    string
	ImageURL               string
}

type DescriptionData struct {
	ShortDescription string
	LongDescription  string
}

type FeltrinelliScrapedBook struct {
	BuyInfos        BuyInfos
	DescriptionData DescriptionData
	Details         map[string]string
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

func unmarshalJSON[T any](data []byte, target *T) error {
	err := json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("error unmarshaling JSON into %T: %w", target, err)
	}
	return nil
}

func getEANFromRequest(e *colly.HTMLElement) string {
	parts := strings.Split(e.Request.URL.Path, "/")
	return parts[len(parts)-1]
}

func getProductInfos(productsChan chan Product) {
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)
	lock := sync.Mutex{}
	booksMap := make(map[string]*FeltrinelliScrapedBook)
	fullBooksChan := make(chan *FeltrinelliScrapedBook)

	c.OnHTML("pdp-physical-buy-info", func(e *colly.HTMLElement) {
		if e.Attr(":is-ebook") == "true" {
			fmt.Println("It's an ebook, skipping.")
			return
		}
		EAN := getEANFromRequest(e)
		if !strings.HasPrefix(EAN, "978") && !strings.HasPrefix(EAN, "979") {
			fmt.Println("It's not a book, skipping.")
			return
		}
		var availabilityJSON AvailabilityJSON
		if unmarshalJSON([]byte(e.Attr(":availability")), &availabilityJSON) != nil {
			return
		}
		var inventoryJSON InventoryJSON
		if unmarshalJSON([]byte(e.Attr(":inventory")), &inventoryJSON) != nil {
			return
		}

		buyInfos := BuyInfos{Price: inventoryJSON.Price,
			Title:                  e.Attr(":product-title"),
			Availability:           availabilityJSON.Text,
			AvailabilityStickyText: availabilityJSON.StickyDesktopText,
			URL:                    e.Request.URL.String(),
			ISBN:                   EAN,
			ImageURL:               "https://www.lafeltrinelli.it/images/" + EAN + "_0_536_0_75.jpg"}
		lock.Lock()
		booksMap[EAN] = &FeltrinelliScrapedBook{}
		booksMap[EAN].BuyInfos = buyInfos
		lock.Unlock()
	})

	c.OnHTML("div.cc-content-text.cc-clamp.cc-clamp--7", func(e *colly.HTMLElement) {
		EAN := getEANFromRequest(e)
		var descriptionParagraphs []string
		e.ForEach("p", func(i int, element *colly.HTMLElement) {
			descriptionParagraphs = append(descriptionParagraphs, element.Text)
		})
		descriptionData := DescriptionData{ShortDescription: descriptionParagraphs[0], LongDescription: strings.Join(descriptionParagraphs[1:], "\n")}
		lock.Lock()
		booksMap[EAN].DescriptionData = descriptionData
		lock.Unlock()
	})

	c.OnHTML("div#pdp-dettagli", func(e *colly.HTMLElement) {
		EAN := getEANFromRequest(e)
		details := map[string]string{}
		e.ForEach("div.cc-em-content-body", func(i int, e2 *colly.HTMLElement) {
			e2.ForEach("div.cc-item", func(i int, e3 *colly.HTMLElement) {
				var key string
				var value string
				e3.ForEach("span", func(i int, e4 *colly.HTMLElement) {
					if i == 0 {
						key = strings.Replace(e4.Text, ":", "", 1)
					} else {
						value = e4.ChildText("a")
						if value == "" {
							value = e4.Text
						}
					}
				})
				details[key] = value
			})
		})

		lock.Lock()
		booksMap[EAN].Details = details
		fullBooksChan <- booksMap[EAN]
		delete(booksMap, EAN)
		lock.Unlock()
	})

	c.OnError(func(response *colly.Response, err error) {
		fmt.Println(response)
		fmt.Println(err)
	})

	go insertFeltrinelliScrapedBooks(fullBooksChan)

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
	//go testSendingProducts(productsChan)

	getProductInfos(productsChan)

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")
}

func testSendingProducts(productsChan chan<- Product) {
	productsChan <- Product{URL: "https://www.lafeltrinelli.it/storia-del-nuovo-cognome-amica-libro-elena-ferrante/e/9788866321811", EAN: "9788866321811"}
}

func insertFeltrinelliScrapedBooks(fullBooksChan <-chan *FeltrinelliScrapedBook) {
	for fullBook := range fullBooksChan {
		fmt.Println(fullBook)
	}
}
