package Ebay

import (
	"Scraper/EbayBookBuilder/PriceConversion"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
)

func SearchMinCost(isbn string) float64 {
	auth := getAccessToken()
	url := fmt.Sprintf("https://api.ebay.com/buy/browse/v1/item_summary/search?q=%s&filter=buyingOptions:{FIXED_PRICE},conditions:{NEW}", isbn)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Language", "it-IT")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "it-IT")
	req.Header.Set("X-EBAY-C-MARKETPLACE-ID", "EBAY_IT")
	req.Header.Set("X-EBAY-C-ENDUSERCTX", "contextualLocation=country=IT,zip=80143")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	minCost := 1000.0
	if itemSummaries, ok := result["itemSummaries"].([]interface{}); ok {
		for _, itemSummary := range itemSummaries {
			item := itemSummary.(map[string]interface{})
			seller := item["seller"].(map[string]interface{})
			if seller["username"] == "ri-manga" {
				continue
			}

			price, err := strconv.ParseFloat(item["price"].(map[string]interface{})["value"].(string), 64)
			if err != nil {
				log.Printf("Error parsing price: %v", err)
				continue
			}
			itemShippingOptions, okShippingOptions := item["shippingOptions"]
			if !okShippingOptions {
				continue
			}
			shippingOptions := itemShippingOptions.([]interface{})
			if len(shippingOptions) == 0 {
				continue
			}

			shippingCost := shippingOptions[0].(map[string]interface{})["shippingCost"].(map[string]interface{})
			shipping := PriceConversion.ConvertPriceToFloat(shippingCost["value"].(string))

			minCost = math.Min(price+shipping, minCost)
		}
	}

	return math.Round(minCost*100) / 100
}
