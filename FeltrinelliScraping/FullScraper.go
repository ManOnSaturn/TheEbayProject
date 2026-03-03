package FeltrinelliScraping

import (
	"Scraper/ChromeClient"
	"Scraper/DataTypes"
	"Scraper/HttpUtil"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

func FullScrape() {
	startTime := time.Now()

	scrapeAllXMLs()

	urlsChan := make(chan string)
	go MongoDBInteractions.GetAllBookProductsAndNewProductsURLsIntoChannel(urlsChan)

	fullBooksChan := make(chan *DataTypes.FeltrinelliScrapedBook)

	proxies := Proxy.GetProxies()
	scrapersWaitGroup := sync.WaitGroup{}
	scrapersWaitGroup.Add(len(proxies))

	fakeChrome := ChromeClient.GetChromeClient()

	for _, proxy := range proxies {
		go func(proxy string) {
			defer scrapersWaitGroup.Done()
			getProductInfos(urlsChan, fullBooksChan, proxy, fakeChrome)
		}(proxy)
	}

	handlerWaitGroup := sync.WaitGroup{}
	handlerWaitGroup.Add(1)
	go func() {
		defer handlerWaitGroup.Done()
		handleScrapedBooks(fullBooksChan)
	}()

	scrapersWaitGroup.Wait()
	close(fullBooksChan)
	handlerWaitGroup.Wait()

	fmt.Println("Finished Feltrinelli full scraping in", time.Since(startTime).Seconds(), "seconds.")
}

func downloadAndParseXML(index int, proxy string) (*DataTypes.UrlSet, error) {
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		log.Fatal(err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
	}

	fullURL := "https://www.lafeltrinelli.it/sitemap_itbook_" + strconv.Itoa(index) + ".xml"
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %v", err)
	}
	defer HttpUtil.CloseBody(resp.Body)

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download: %s (status code: %d)", fullURL, resp.StatusCode)
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
	fmt.Println("Successfully parsed", fullURL)
	return &urlSet, nil
}

func scrapeAllXMLs() {
	urlSets := make([]DataTypes.UrlSet, 0)

	startTime := time.Now()

	numOfSitemaps := fetchNumberOfSitemaps()
	if numOfSitemaps <= 0 {
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(numOfSitemaps)
	proxies := Proxy.GetProxies()
	mutex := sync.Mutex{}

	for i := 1; i <= numOfSitemaps; i++ {
		go func(index int) {
			urlSet, err := downloadAndParseXML(index, proxies[index%len(proxies)])
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			mutex.Lock()
			urlSets = append(urlSets, *urlSet)
			mutex.Unlock()

			wg.Done()
		}(i)
	}

	wg.Wait()
	fmt.Printf("Finished getting all feltrinelli book's URLs from sitemaps in %g\n", time.Since(startTime).Seconds())

	var models []mongo.WriteModel

	lastSeen := time.Now()

	startTime = time.Now()
	for urlSetsIndex, urlSet := range urlSets {
		for _, entry := range urlSet.URLs {
			model := MongoDBInteractions.BuildFeltrinelliProductUpsertModel(entry, lastSeen)

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

	MongoDBInteractions.RemoveAllUnseenProductsAndBooks(lastSeen, MongoDBInteractions.Feltrinelli)
}

func fetchNumberOfSitemaps() int {
	// Fetch the XML content from the URL
	resp, err := HttpUtil.GetRequestWithHeader("https://www.lafeltrinelli.it/sitemap_itbook_index.xml", nil)
	if err != nil || resp == nil {
		fmt.Println("Error fetching sitemap index:", err)
		return -1
	}
	defer HttpUtil.CloseBody(resp.Body)

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
			MongoDBInteractions.SetBookIsRedirected(originalReqURL)
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

	for URL := range urlsChan {
		err := c.Visit(URL)
		if err != nil && !strings.Contains(err.Error(), "redirects are not allowed") {
			_, errFmt := fmt.Fprintf(os.Stderr, "Failed to visit:%s\n", err)
			if errFmt != nil {
				panic(errFmt)
			}
		}
	}
	c.Wait()
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
