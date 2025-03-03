package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"go.mongodb.org/mongo-driver/mongo"
	"sync"
)

func Repricer() {
	urlsChan := make(chan string, 100)
	booksChan := make(chan *DataTypes.MondadoriBook, 100)
	proxies := Proxy.GetProxies()

	wg := sync.WaitGroup{}
	wg.Add(len(proxies))

	fakeChrome := getChromeClient()
	for _, proxy := range proxies {
		go func(proxy string) {
			defer wg.Done()
			scrapeBooks(urlsChan, booksChan, proxy, fakeChrome)
		}(proxy)
	}

	go MongoDBInteractions.GetAllMondadoriURLsOnEbay(urlsChan)

	go func() {
		wg.Wait()
		close(booksChan)
	}()

	var models []mongo.WriteModel
	for book := range booksChan {
		models = append(models, MongoDBInteractions.CreateUpsertModelFromMondadoriBook(*book))
		if len(models) == 1000 {
			MongoDBInteractions.UpsertMondadoriBooks(models)
			models = make([]mongo.WriteModel, 0)
		}
	}

	if len(models) > 0 {
		MongoDBInteractions.UpsertMondadoriBooks(models)
	}
}
