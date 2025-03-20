package FeltrinelliScraping

import (
	"Scraper/ChromeClient"
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"context"
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

func FullScrape() {
	startTime := time.Now()

	//scrapeAllXMLs()

	urlsChan := make(chan string)
	go getNewProducts(urlsChan)
	//go testSendingProducts(urlsChan)

	fullBooksChan := make(chan *DataTypes.FeltrinelliScrapedBook)

	proxies := Proxy.GetProxies()
	wg := sync.WaitGroup{}
	wg.Add(len(proxies))

	fakeChrome := ChromeClient.GetChromeClient()

	for _, proxy := range proxies {
		go func(proxy string) {
			defer wg.Done()
			getProductInfos(urlsChan, fullBooksChan, proxy, fakeChrome)
		}(proxy)
	}

	go handleScrapedBooks(fullBooksChan)

	wg.Wait()
	close(fullBooksChan)

	fmt.Println("Finished Feltrinelli full scraping in ", time.Since(startTime).Seconds(), "seconds.")
}

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

	startTime := time.Now() // This took about 103 seconds
	for i := 1; i <= fetchNumberOfSitemaps(); i++ {
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
	fmt.Printf("Finished getting all feltrinelli book's URLs from sitemaps in %g\n", time.Since(startTime).Seconds())

	var models []mongo.WriteModel

	lastSeen := time.Now()

	startTime = time.Now() // This takes about 236 seconds
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

				fmt.Println("Working URLSet index", urlSetsIndex, "out of", len(urlSets))
			}
		}
	}

	// Execute remaining models in bulk
	if len(models) > 0 {
		fmt.Println("Processing last", len(models), " into the DB.")
		MongoDBInteractions.BulkWriteFeltrinelliProducts(models)
	}
	fmt.Printf("Finished processing all URLs in %g\n", time.Since(startTime).Seconds())

	MongoDBInteractions.RemoveAllUnseenProductsAndBooksFeltrinelli(lastSeen)
}

func fetchNumberOfSitemaps() int {
	// Fetch the XML content from the URL
	err, resp := getRequestWithHeader("https://www.lafeltrinelli.it/sitemap_itbook_index.xml")
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("error closing body", err)
		}
	}(resp.Body)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return -1
	}

	// Parse the XML
	var sitemapIndex DataTypes.SitemapIndex
	err = xml.Unmarshal(body, &sitemapIndex)
	if err != nil {
		fmt.Println("Error parsing XML:", err)
		return -1
	}

	// Define the regex pattern to match the links
	pattern := regexp.MustCompile(`https://www\.lafeltrinelli\.it/sitemap_itbook_(\d+)\.xml`)

	maxNumber := 0

	// Iterate over the URLs and find the one with the highest number
	for _, sitemapObject := range sitemapIndex.Sitemaps {
		matches := pattern.FindStringSubmatch(sitemapObject.Loc)
		if len(matches) == 2 {
			number, err := strconv.Atoi(matches[1])
			if err != nil {
				fmt.Println("Error converting number:", err)
				continue
			}
			if number > maxNumber {
				maxNumber = number
			}
		}
	}

	return maxNumber
}

func getRequestWithHeader(url string) (error, *http.Response) {
	// Create a new HTTP request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err), nil
	}

	// Set a custom User-Agent
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("error making request: %v", err), resp
	}

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: Received non-200 response code: %d", resp.StatusCode), nil
	}

	return err, resp
}

func getNewProducts(urlsChan chan string) {
	// Filter for documents where the field 'IsBook' does not exist
	//filter := bson.M{"IsBook": bson.M{"$exists": false}}
	filter := bson.M{
		"$or": []bson.M{
			{"IsBook": false},
			{"IsBook": bson.M{"$exists": false}},
		},
	}
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

func cleanDescription(input string) string {
	// Remove leading spaces and newlines
	re := regexp.MustCompile(`^[\s\r\n]+`)
	trimmed := re.ReplaceAllString(input, "")

	// Replace multiple spaces or newlines with a single space
	re = regexp.MustCompile(`[\s\r\n]{2,}`)
	return re.ReplaceAllString(trimmed, " ")
}

func getProductInfos(urlsChan <-chan string, fullBooksChan chan<- *DataTypes.FeltrinelliScrapedBook, proxy string, fakeChrome *req.Client) {
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			originalReqURL := via[len(via)-1].URL.String()
			MongoDBInteractions.SetProductIsBook(originalReqURL, false)
			return fmt.Errorf("redirects are not allowed")
		},
	})

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

	err := c.Limit(&colly.LimitRule{DomainGlob: "*"})
	if err != nil {
		panic(err)
	}
	err = c.SetProxy(proxy)
	if err != nil {
		panic(err)
	}
	c.SetRequestTimeout(30 * time.Second)

	fullBook := &DataTypes.FeltrinelliScrapedBook{}

	c.OnHTML("pdp-physical-buy-info", func(e *colly.HTMLElement) {
		if e.Attr(":is-ebook") == "true" {
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

		availabilityText := availabilityJSON.Text
		if e.Attr(":is-marketplace") == "true" {
			availabilityText = "Marketplace only"
		}

		title := e.Attr(":product-title")
		title = title[1 : len(title)-1]
		buyInfos := DataTypes.BuyInfos{Price: inventoryJSON.Price,
			Title:        title,
			Availability: availabilityText,
			URL:          e.Request.URL.String(),
			ISBN:         EAN}

		fullBook.BuyInfos = buyInfos
	})

	c.OnHTML("ul.cc-breadcrumbs-list", func(e *colly.HTMLElement) {
		if e.Request.Ctx.GetAny("Skip") == true {
			return
		}

		var breadcrumbTexts []string
		e.ForEach("span", func(i int, element *colly.HTMLElement) {
			breadcrumbTexts = append(breadcrumbTexts, element.Text)
		})

		fullBook.Category = strings.Join(breadcrumbTexts[2:], " > ")
	})

	c.OnHTML("div.cc-content-text.cc-clamp.cc-clamp--7", func(e *colly.HTMLElement) {
		if e.Request.Ctx.GetAny("Skip") == true {
			return
		}

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

		fullBook.DescriptionData = descriptionData
	})

	c.OnHTML("div#pdp-dettagli", func(e *colly.HTMLElement) {
		if e.Request.Ctx.GetAny("Skip") == true {
			return
		}

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

		fullBook.Details = details
	})

	c.OnScraped(func(response *colly.Response) {
		if response.Request.Ctx.GetAny("Skip") == true {
			return
		}
		fullBooksChan <- fullBook
		fullBook = &DataTypes.FeltrinelliScrapedBook{}
	})

	c.OnError(func(response *colly.Response, err error) {
		_, err2 := fmt.Fprintf(os.Stderr, "error for request:%s, %v\n", response.Request.URL, err)
		if err2 != nil {
			panic(err2)
		}
	})

	for url := range urlsChan {
		err := c.Visit(url)
		if err != nil && !strings.Contains(err.Error(), "redirects are not allowed") {
			_, errFmt := fmt.Fprintf(os.Stderr, "Failed to visit:%s\n", err)
			if errFmt != nil {
				panic(errFmt)
			}
		}
	}
	c.Wait()
}

func testSendingProducts(urlsChan chan<- string) {
	urlsChan <- "https://www.lafeltrinelli.it/siti-sacri-segreti-ediz-illustrata-libro-martin-gray/e/9782361956875"
	close(urlsChan)
}

func handleScrapedBooks(fullBooksChan <-chan *DataTypes.FeltrinelliScrapedBook) {
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
