package AmazonScraping

import (
	"Scraper/ChromeClient"
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

// https://www.amazon.it/gp/bestsellers/books

type Item struct {
	ID string `json:"id"`
}

func getUnknownASINsFromList(ASINs []string, ASINsChannel chan<- string) {
	for _, ASIN := range ASINs {
		if !MongoDBInteractions.IsASINStored(ASIN) {
			ASINsChannel <- ASIN
		}
	}
	close(ASINsChannel)
}

func ScrapeBestsellers() {
	asins := scrapeASINsFromAmazon()

	scrapeISBNsFromAmazon(asins)
}

func scrapeASINsFromAmazon() []string {
	links := getLinksFromBestsellerPages()
	asins := extractASINsFromLinks(links)
	return asins
}

func scrapeISBNsFromAmazon(asins []string) {
	ASINsChannel := make(chan string, 100)

	go getUnknownASINsFromList(asins, ASINsChannel)

	ASINISBNPairsChannel := make(chan DataTypes.ASINISBNPair)
	kindleASINsChannel := make(chan string)

	insertWaitingGroup := sync.WaitGroup{}
	insertWaitingGroup.Add(2)

	go func() {
		defer insertWaitingGroup.Done()
		insertISBNs(ASINISBNPairsChannel)
	}()
	go func() {
		defer insertWaitingGroup.Done()
		insertASINsKindle(kindleASINsChannel)
	}()

	fakeChrome := ChromeClient.GetChromeClient()

	proxies := Proxy.GetProxies()
	scrapingWaitingGroup := sync.WaitGroup{}
	scrapingWaitingGroup.Add(len(proxies))

	for _, proxy := range proxies {
		go func(proxy string) {
			defer scrapingWaitingGroup.Done()
			getISBNsFromASINs(ASINsChannel, fakeChrome, proxy, ASINISBNPairsChannel, kindleASINsChannel)
		}(proxy)
	}

	scrapingWaitingGroup.Wait()

	close(ASINISBNPairsChannel)
	close(kindleASINsChannel)

	insertWaitingGroup.Wait()
}

func insertASINsKindle(asinsKindle chan string) {
	var bulkOps []mongo.WriteModel
	for ASIN := range asinsKindle {
		model := MongoDBInteractions.BuildAmazonBestsellerIsKindleUpdateModel(ASIN)
		bulkOps = append(bulkOps, model)
	}

	result := MongoDBInteractions.BulkWriteBestsellers(bulkOps)
	fmt.Printf("(Kindle books) Inserted %d, Upserted %d and modified %d documents.\n", result.InsertedCount, result.UpsertedCount, result.ModifiedCount)
}

func insertISBNs(ASINISBNPairs chan DataTypes.ASINISBNPair) {
	var bulkOps []mongo.WriteModel
	for pair := range ASINISBNPairs {
		model := MongoDBInteractions.BuildAmazonBestsellerISBNUpdateModel(pair)
		bulkOps = append(bulkOps, model)
	}

	result := MongoDBInteractions.BulkWriteBestsellers(bulkOps)
	fmt.Printf("(Normal books) Inserted %d, Upserted %d and modified %d documents.\n", result.InsertedCount, result.UpsertedCount, result.ModifiedCount)
}

func getISBNsFromASINs(ASINsChannel <-chan string, fakeChrome *req.Client, proxy string, ASINISBNPairsChannel chan<- DataTypes.ASINISBNPair, kindleASINsChannel chan<- string) {
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})

	err := c.Limit(&colly.LimitRule{DomainGlob: "*", Delay: 2 * time.Second})
	if err != nil {
		panic(err)
	}

	err = c.SetProxy(proxy)
	if err != nil {
		panic(err)
	}

	c.SetRequestTimeout(30 * time.Second)

	isbnRegex := regexp.MustCompile(`\d{3}-\d{10}`)

	c.OnHTML("body", func(element *colly.HTMLElement) {
		isbn := isbnRegex.FindString(element.Text)
		asin := element.Request.URL.Path[4:]
		if len(isbn) < 13 {
			element.ForEach("span#productSubtitle", func(i int, element *colly.HTMLElement) {
				if i > 0 {
					panic("Found i>0")
				}
				if strings.Contains(element.Text, "Formato Kindle") {
					kindleASINsChannel <- asin
				}
			})
			return
		}
		pair := DataTypes.ASINISBNPair{ASIN: asin, ISBN: strings.Replace(isbn, "-", "", 1)}
		ASINISBNPairsChannel <- pair
	})

	c.OnError(func(response *colly.Response, err error) {
		fmt.Println(response.Request.URL)
		fmt.Println(err.Error())
	})

	for ASIN := range ASINsChannel {
		URL := "https://www.amazon.it/dp/" + ASIN
		err := c.Visit(URL)
		if err != nil {
			fmt.Println(err)
		}
	}

	c.Wait()
}

// span#productSubtitle   Formato Kindle
func getLinksFromBestsellerPages() []string {
	URLs := []string{
		"https://www.amazon.it/gp/bestsellers/books",
		"https://www.amazon.it/gp/bestsellers/books/13077484031", // Adolescenti e ragazzi
		"https://www.amazon.it/gp/bestsellers/books/508714031",   // Biografie
		"https://www.amazon.it/gp/bestsellers/books/508864031",   // Dizionari e opere di consultazione
		"https://www.amazon.it/gp/bestsellers/books/508785031",   // Diritto
		"https://www.amazon.it/gp/bestsellers/books/508786031",   // Economia, affari e finanza
		"https://www.amazon.it/gp/bestsellers/books/508773031",   // Fantascienza
		"https://www.amazon.it/gp/bestsellers/books/508772031",   // Fantasy
		"https://www.amazon.it/gp/bestsellers/books/508784031",   // Fumetti e Manga
		"https://www.amazon.it/gp/bestsellers/books/508771031",   // Gialli e Thriller
		"https://www.amazon.it/gp/bestsellers/books/508821031",   // Tempo libero
		"https://www.amazon.it/gp/bestsellers/books/508774031",   // Horror
		"https://www.amazon.it/gp/bestsellers/books/508820031",   // Humour
		"https://www.amazon.it/gp/bestsellers/books/508733031",   // Informatica, Web e digital media
		"https://www.amazon.it/gp/bestsellers/books/13466598031", // Lettura erotica
		"https://www.amazon.it/gp/bestsellers/books/508770031",   // Letteratura e narrativa
		"https://www.amazon.it/gp/bestsellers/books/508811031",   // Politica
		"https://www.amazon.it/gp/bestsellers/books/508745031",   // Religione
		"https://www.amazon.it/gp/bestsellers/books/508775031",   // Romanzi rosa
		"https://www.amazon.it/gp/bestsellers/books/508792031",   // Famiglia salute e benessere
		"https://www.amazon.it/gp/bestsellers/books/508867031",   // Scienza, tecnologia e medicina
		"https://www.amazon.it/gp/bestsellers/books/508794031",   // Self-help
		"https://www.amazon.it/gp/bestsellers/books/508879031",   // Società e scienze sociali
		"https://www.amazon.it/gp/bestsellers/books/508835031",   // Sport
		"https://www.amazon.it/gp/bestsellers/books/508796031",   // Storia
		"https://www.amazon.it/gp/bestsellers/books/508753031",   // Viaggi
	}
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)
	links := make([]string, 0)

	c.OnHTML("div.p13n-desktop-grid", func(element *colly.HTMLElement) {
		links = append(links, element.Attr("data-client-recs-list"))
	})

	c.OnHTML("ul.a-pagination li.a-normal a", func(element *colly.HTMLElement) {
		secondPageLink := element.Attr("href")
		URLs = URLs[1:]
		if !strings.HasSuffix(secondPageLink, "pg=1") {
			URLs = append(URLs, "https://www.amazon.it"+secondPageLink)
		}
	})

	c.OnError(func(response *colly.Response, err error) {
		fmt.Println(response)
		fmt.Println(err)
	})

	for len(URLs) > 0 {
		fmt.Println("Remaining bestsellers pages: ", len(URLs))
		err := c.Visit(URLs[0])
		if err != nil {
			fmt.Println(err)
		}
	}

	c.Wait()

	return links
}

func extractASINsFromLinks(links []string) []string {
	var itemArrays [][]Item
	for _, link := range links {
		var items []Item
		err := json.Unmarshal([]byte(link), &items)
		if err != nil {
			log.Fatalf("Error unmarshalling JSON for link %s: %v", link, err)
		}
		itemArrays = append(itemArrays, items)
	}

	// Collect the IDs
	ASINs := make(map[string]struct{})
	for _, itemArray := range itemArrays {
		for _, item := range itemArray {
			ASINs[item.ID] = struct{}{}
		}
	}

	// Convert map keys to a slice
	ASINList := make([]string, 0, len(ASINs))
	for ASIN := range ASINs {
		ASINList = append(ASINList, ASIN)
	}
	return ASINList
}
