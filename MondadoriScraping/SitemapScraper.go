package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/HttpUtil"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
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

	MongoDBInteractions.RemoveAllUnseenProductsAndBooks(lastSeen, MongoDBInteractions.Mondadori)
}

func fetchNumberOfSitemaps() int {
	// Fetch the XML content from the URL
	resp, err := HttpUtil.GetRequestWithHeader("https://www.mondadoristore.it/sitemap.xml", nil)
	if err != nil || resp == nil {
		fmt.Println("Error fetching sitemap:", err)
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
	resp, err := HttpUtil.GetRequestWithHeader(URL, &proxy)
	if err != nil || resp == nil {
		return nil, fmt.Errorf("error occurred while fetching mondadori sitemap-libri %d:%v\n", index, err)
	}
	defer HttpUtil.CloseBody(resp.Body)

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
