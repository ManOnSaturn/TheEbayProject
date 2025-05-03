package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"
)

func scrapeXMLs() {
	numOfSitemaps := fetchNumberOfSitemaps()
	lastSeen := time.Now()
	var models []mongo.WriteModel
	mutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(numOfSitemaps)
	proxies := Proxy.GetProxies()

	for i := 1; i <= numOfSitemaps; i++ {
		go func(index int) {
			mondadoriSitemapItemFile, err := downloadAndUncompressGzToXML(index, proxies[index%len(proxies)])
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			for _, entry := range mondadoriSitemapItemFile.MondadoriSitemapItem {
				model := MongoDBInteractions.BuildMondadoriProductUpsertModel(entry, lastSeen)

				mutex.Lock()
				models = append(models, model)

				if len(models) >= 10000 {
					MongoDBInteractions.BulkWriteMondadoriProducts(models)
					models = make([]mongo.WriteModel, 0)
					fmt.Println("Processed 10000 XML entries into the DB.")
				}

				mutex.Unlock()
			}

			wg.Done()
		}(i)
	}

	wg.Wait()

	// Execute remaining models in bulk
	if len(models) > 0 {
		fmt.Println("Processing last", len(models), "into the DB.")
		MongoDBInteractions.BulkWriteMondadoriProducts(models)
	}

	MongoDBInteractions.RemoveAllUnseenProductsAndBooksMondadori(lastSeen)
}

func fetchNumberOfSitemaps() int {
	// Fetch the XML content from the URL
	err, resp := getRequestWithHeader("https://www.mondadoristore.it/sitemap.xml", nil)
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
	pattern := regexp.MustCompile(`https://www\.mondadoristore\.it/sitemap-libri-(\d+)\.xml\.gz`)

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

func downloadAndUncompressGzToXML(index int, proxy string) (*DataTypes.MondadoriSitemapItemFile, error) {
	URL := "https://www.mondadoristore.it/sitemap-libri-" + strconv.Itoa(index) + ".xml.gz"
	err, resp := getRequestWithHeader(URL, &proxy)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("error closing body", err)
		}
	}(resp.Body)
	if err != nil {
		return nil, err
	}

	// Create a gzip reader
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		fmt.Println("Error creating gzip reader:", err)
		return nil, err
	}
	defer func(gzReader *gzip.Reader) {
		err := gzReader.Close()
		if err != nil {
			fmt.Println("Error closing gzip reader:", err)
		}
	}(gzReader)

	// Read the decompressed data into a buffer
	var buf bytes.Buffer
	_, err = io.Copy(&buf, gzReader)
	if err != nil {
		fmt.Println("Error reading decompressed data:", err)
		return nil, err
	}

	// Parse the XML data
	var mondadoriSitemapItemFile DataTypes.MondadoriSitemapItemFile
	err = xml.Unmarshal(buf.Bytes(), &mondadoriSitemapItemFile)
	if err != nil {
		fmt.Println("Error parsing XML:", err)
		return nil, err
	}

	return &mondadoriSitemapItemFile, nil
}

func getRequestWithHeader(URL string, proxy *string) (error, *http.Response) {
	// Create a new HTTP request
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err), nil
	}

	// Set a custom User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")

	// Create HTTP client with proxy if provided
	client := &http.Client{}

	if proxy != nil && *proxy != "" {
		proxyUrl, err := url.Parse(*proxy)
		if err != nil {
			return fmt.Errorf("error parsing proxy URL: %v", err), nil
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}

		client = &http.Client{
			Transport: transport,
		}
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err), resp
	}

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: Received non-200 response code: %d", resp.StatusCode), nil
	}

	return err, resp
}
