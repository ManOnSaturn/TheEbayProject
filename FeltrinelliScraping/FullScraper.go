package FeltrinelliScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func downloadAndParseXML(url string) (*DataTypes.UrlSet, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

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
	var urlSet DataTypes.UrlSet
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

func scrapeAllXMLs() {
	baseURL := "https://www.lafeltrinelli.it/sitemap_itbook_"
	urlSets := make([]DataTypes.UrlSet, 0)

	for i := 1; i <= 70; i++ {
		// Construct the URL
		url := baseURL + strconv.Itoa(i) + ".xml"

		// Download and parse the XML file
		urlSet, err := downloadAndParseXML(url)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}
		urlSets = append(urlSets, *urlSet)
	}
	var models []mongo.WriteModel

	lastSeen := time.Now()

	for urlSetsIndex, urlSet := range urlSets {
		for _, entry := range urlSet.URLs {
			filter := bson.M{"URL": entry.Loc}

			// Create the update document
			update := bson.M{
				"$set": bson.M{
					"URL":      entry.Loc,
					"EAN":      getEAN(entry.Loc),
					"LastSeen": lastSeen,
				},
			}

			// Create an UpdateOneModel with upsert option
			model := mongo.NewUpdateOneModel().
				SetFilter(filter).
				SetUpdate(update).
				SetUpsert(true)

			models = append(models, model)

			if len(models) >= 10000 {
				MongoDBInteractions.BulkWriteFeltrinelliProducts(models)
				models = make([]mongo.WriteModel, 0)
				fmt.Println("Processed 10000 XML entries into the DB.")
				fmt.Println("Working URLSet index", urlSetsIndex, "out of", len(urlSets))
			}
		}
	}

	// Execute remaining models in bulk
	if len(models) > 0 {
		fmt.Println("Processing last", len(models), " into the DB.")
		MongoDBInteractions.BulkWriteFeltrinelliProducts(models)
	}

	MongoDBInteractions.RemoveAllUnseenProductsAndBooks(lastSeen)
}

func getNewProducts(urlsChan chan string) {
	// Filter for documents where the field 'IsBook' does not exist
	filter := bson.M{"IsBook": bson.M{"$exists": false}}
	todoContext := context.TODO()

	documentsCount, _ := MongoDBInteractions.FeltrinelliProductsCollection.CountDocuments(todoContext, filter)
	fmt.Println("Number of new products: ", documentsCount)

	opts := options.Find().SetBatchSize(1000)
	cursor, err := MongoDBInteractions.FeltrinelliProductsCollection.Find(todoContext, filter, opts)
	if err != nil {
		log.Fatal(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			panic(err)
		}
	}(cursor, todoContext)

	startTime := time.Now()
	count := 0
	// Iterate through the cursor and send documents to the channel
	for cursor.Next(todoContext) {
		var document bson.M
		if err := cursor.Decode(&document); err != nil {
			// Log the error but continue processing other documents
			log.Printf("Error decoding document: %v\n", err)
			continue
		}

		// Safely extract URL and EAN from the document
		url, okURL := document["URL"].(string)
		ean := url[len(url)-13:]

		if !okURL {
			log.Fatalf("Missing or invalid URL in document: %v\n", document)
		}
		if !strings.HasPrefix(ean, "978") && !strings.HasPrefix(ean, "979") {
			MongoDBInteractions.SetProductIsBook(url, false)
			continue
		}
		urlsChan <- url
		count++
		if count%100 == 0 {
			newNow := time.Now()
			fmt.Println(count, "products processed. 100 done in", newNow.Sub(startTime).Seconds())
			startTime = newNow
		}
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
	close(urlsChan) // Close the channel when done
}

func UnmarshalJSON[T any](data []byte, target *T) error {
	err := json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("error unmarshaling JSON into %T: %w", target, err)
	}
	return nil
}

func GetEANFromPath(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func cleanDescription(input string) string {
	// Remove leading spaces and newlines
	re := regexp.MustCompile(`^[\s\r\n]+`)
	trimmed := re.ReplaceAllString(input, "")

	// Replace multiple spaces or newlines with a single space
	re = regexp.MustCompile(`[\s\r\n]{2,}`)
	return re.ReplaceAllString(trimmed, " ")
}

func areURLsForSameBook(URL1 string, URL2 string) bool {
	if len(URL1) >= 13 && len(URL2) >= 13 {
		if URL1[len(URL1)-13:] == URL2[len(URL2)-13:] {
			return true
		}
	}
	return false
}

func getProductInfos(urlsChan <-chan string, fullBooksChan chan<- *DataTypes.FeltrinelliScrapedBook) {
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")), colly.Async(true))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			reqURL := req.URL.String()
			originalReqURL := via[len(via)-1].URL.String()
			if areURLsForSameBook(originalReqURL, reqURL) {
				MongoDBInteractions.SetNewURLAndIsBook(originalReqURL, reqURL, true)
				return nil
			}
			MongoDBInteractions.SetProductIsBook(originalReqURL, false)
			return fmt.Errorf("redirects are not allowed")
		},
	})

	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 10,
	})
	if err != nil {
		panic(err)
	}
	c.SetRequestTimeout(30 * time.Second)

	semaphore := DataTypes.NewSemaphore(50)
	booksMap := make(map[string]*DataTypes.FeltrinelliScrapedBook)
	booksMapLock := sync.Mutex{}

	c.OnHTML("pdp-physical-buy-info", func(e *colly.HTMLElement) {
		if e.Attr(":is-ebook") == "true" || e.Attr(":is-marketplace") == "true" {
			MongoDBInteractions.SetProductIsBook(e.Request.URL.String(), false)
			e.Request.Ctx.Put("Skip", true)
			return
		}
		EAN := GetEANFromPath(e.Request.URL.Path)
		var inventoryJSON DataTypes.InventoryJSON
		if UnmarshalJSON([]byte(e.Attr(":inventory")), &inventoryJSON) != nil {
			return
		}
		var availabilityJSON DataTypes.AvailabilityJSON
		if UnmarshalJSON([]byte(e.Attr(":availability")), &availabilityJSON) != nil {
			return
		}

		title := e.Attr(":product-title")
		title = title[1 : len(title)-1]
		buyInfos := DataTypes.BuyInfos{Price: inventoryJSON.Price,
			Title:        title,
			Availability: availabilityJSON.Text,
			URL:          e.Request.URL.String(),
			ISBN:         EAN}
		//ImageURL: "https://www.lafeltrinelli.it/images/" + EAN + "_0_536_0_75.jpg"
		booksMapLock.Lock()
		booksMap[EAN] = &DataTypes.FeltrinelliScrapedBook{}
		booksMap[EAN].BuyInfos = buyInfos
		booksMapLock.Unlock()
	})

	c.OnHTML("ul.cc-breadcrumbs-list", func(e *colly.HTMLElement) {
		if e.Request.Ctx.GetAny("Skip") == true {
			return
		}
		var breadcrumbTexts []string
		e.ForEach("span", func(i int, element *colly.HTMLElement) {
			breadcrumbTexts = append(breadcrumbTexts, element.Text)
		})
		EAN := GetEANFromPath(e.Request.URL.Path)
		booksMapLock.Lock()
		if _, ok := booksMap[EAN]; ok {
			booksMap[EAN].Category = strings.Join(breadcrumbTexts[2:], " > ")
		} else {
			MongoDBInteractions.SetProductProblematic(e.Request.URL.String(), true)
			e.Request.Ctx.Put("Skip", true)
		}
		booksMapLock.Unlock()
	})

	c.OnHTML("div.cc-content-text.cc-clamp.cc-clamp--7", func(e *colly.HTMLElement) {
		if e.Request.Ctx.GetAny("Skip") == true {
			return
		}
		EAN := GetEANFromPath(e.Request.URL.Path)
		var descriptionParagraphs []string
		e.ForEach("p", func(i int, element *colly.HTMLElement) {
			descriptionParagraphs = append(descriptionParagraphs, element.Text)
		})
		var descriptionData DataTypes.DescriptionData
		if descriptionParagraphs != nil && len(descriptionParagraphs) > 0 {
			descriptionData = DataTypes.DescriptionData{ShortDescription: descriptionParagraphs[0], LongDescription: strings.Join(descriptionParagraphs[1:], "\n")}
		} else {
			descriptionData = DataTypes.DescriptionData{ShortDescription: cleanDescription(e.Text)}
		}
		booksMapLock.Lock()
		booksMap[EAN].DescriptionData = descriptionData
		booksMapLock.Unlock()
	})

	c.OnHTML("div#pdp-dettagli", func(e *colly.HTMLElement) {
		if e.Request.Ctx.GetAny("Skip") == true {
			return
		}
		EAN := GetEANFromPath(e.Request.URL.Path)
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

		booksMapLock.Lock()
		booksMap[EAN].Details = details
		booksMapLock.Unlock()
	})

	c.OnScraped(func(response *colly.Response) {
		semaphore.Release()
		if response.Request.Ctx.GetAny("Skip") == true {
			return
		}
		EAN := GetEANFromPath(response.Request.URL.Path)
		booksMapLock.Lock()
		if fullBook, ok := booksMap[EAN]; !ok {
			log.Fatalf("Failed to retrieve EAN in booksMap: %s", EAN)
		} else {
			fullBooksChan <- fullBook
			delete(booksMap, EAN)
		}
		booksMapLock.Unlock()
	})

	c.OnError(func(response *colly.Response, err error) {
		semaphore.Release()
		_, err2 := fmt.Fprintf(os.Stderr, "error for request:%s, %v\n", response.Request.URL, err)
		if err2 != nil {
			panic(err2)
		}
	})

	for url := range urlsChan {
		semaphore.Acquire()
		err := c.Visit(url)
		if err != nil {
			log.Fatalf("Failed to visit:%s", err)
		}
	}
	c.Wait()
	close(fullBooksChan)
}

func FullScrapeFeltrinelli() {
	startTime := time.Now()

	scrapeAllXMLs()

	urlsChan := make(chan string)
	go getNewProducts(urlsChan)
	//go testSendingProducts(urlsChan)

	fullBooksChan := make(chan *DataTypes.FeltrinelliScrapedBook)

	go getProductInfos(urlsChan, fullBooksChan)
	handleFeltrinelliScrapedBooks(fullBooksChan)

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")
}

func testSendingProducts(urlsChan chan<- string) {
	urlsChan <- "https://www.lafeltrinelli.it/siti-sacri-segreti-ediz-illustrata-libro-martin-gray/e/9782361956875"
	close(urlsChan)
}

func handleFeltrinelliScrapedBooks(fullBooksChan <-chan *DataTypes.FeltrinelliScrapedBook) {
	startTime := time.Now()
	lastTime := startTime
	count := 0

	for fullBook := range fullBooksChan {
		MongoDBInteractions.InsertFeltrinelliScrapedBook(fullBook)
		count++
		if count%100 == 0 {
			elapsed := time.Since(startTime).Seconds()
			diff := time.Since(lastTime).Seconds()
			fmt.Printf("Inserted %d books. Time elapsed: %.2f. Since last: %.2f\n", count, elapsed, diff)
			lastTime = time.Now()
		}
	}
}
