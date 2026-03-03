package Ebay

import (
	"Scraper/EbayBookBuilder/PriceConversion"
	"Scraper/HttpUtil"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
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
	defer HttpUtil.CloseBody(resp.Body)

	body, err := io.ReadAll(resp.Body)
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

func DeleteOffer(offerID string, retrying bool) bool {
	auth := getAccessToken()
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", "https://api.ebay.com/sell/inventory/v1/offer/"+offerID, nil)
	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		if err != nil {
			panic(err)
		}
		return false
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Language", "it-IT")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "Error making request: %v\n", err)
		if err != nil {
			panic(err)
		}
		return false
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusNoContent { // 204
		_, err = fmt.Printf("Offer %s deleted successfully.\n", offerID)
		if err != nil {
			panic(err)
		}
		return true
	}

	if resp.StatusCode == http.StatusInternalServerError && !retrying { // 500
		fmt.Println("Internal Server Error(500) occurred. Retrying delete offer.")
		return DeleteOffer(offerID, true)
	}

	_, err = fmt.Fprintf(os.Stderr, "Offer %s deletion failed: %v\n", offerID, resp)
	if err != nil {
		panic(err)
	}
	return false
}

func DeleteInventoryItem(isbn string, retrying bool) bool {
	auth := getAccessToken()
	client := &http.Client{}
	req, err := http.NewRequest(
		"DELETE",
		"https://api.ebay.com/sell/inventory/v1/inventory_item/"+isbn,
		nil,
	)
	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "Error creating request for ISBN %s: %v\n", isbn, err)
		if err != nil {
			panic(err)
		}
		return false
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Language", "it-IT")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "Error deleting inventory item %s: %v\n", isbn, err)
		if err != nil {
			panic(err)
		}
		return false
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusNoContent { // 204
		_, err = fmt.Fprintf(os.Stderr, "Status code is %d when deleting %s, and we don't know why.\n", resp.StatusCode, isbn)
		if err != nil {
			panic(err)
		}
		return false
	}

	if resp.StatusCode == http.StatusInternalServerError && !retrying { // 500
		fmt.Println("Internal Server Error(500) occurred. Retrying delete offer.")
		return DeleteInventoryItem(isbn, true)
	}

	_, err = fmt.Printf("Item %s deleted successfully.\n", isbn)
	if err != nil {
		panic(err)
	}
	return true
}

type InventoryItem struct {
	SKU string `json:"sku"`
}

type InventoryResponse struct {
	InventoryItems []InventoryItem `json:"inventoryItems"`
	Total          int             `json:"total"`
}

func GetInventoryItems() ([]InventoryItem, error) {
	auth := getAccessToken()
	headers := map[string]string{
		"Authorization":    "Bearer " + auth,
		"Content-Type":     "application/json",
		"Content-Language": "it-IT",
		"Accept":           "application/json",
	}

	// Make the first request
	firstResp, err := makeRequest("https://api.ebay.com/sell/inventory/v1/inventory_item?limit=200&offset=0", headers)
	if err != nil {
		return nil, err
	}

	var firstResponse InventoryResponse
	err = json.Unmarshal(firstResp, &firstResponse)
	if err != nil {
		return nil, err
	}

	inventoryItems := make([]InventoryItem, len(firstResponse.InventoryItems))
	copy(inventoryItems, firstResponse.InventoryItems)
	numElements := firstResponse.Total

	// Continue fetching until we have all items
	for len(inventoryItems) < numElements {
		url := fmt.Sprintf("https://api.ebay.com/sell/inventory/v1/inventory_item?limit=200&offset=%d", len(inventoryItems))
		resp, err := makeRequest(url, headers)
		if err != nil {
			return nil, err
		}

		var response InventoryResponse
		err = json.Unmarshal(resp, &response)
		if err != nil {
			return nil, err
		}

		inventoryItems = append(inventoryItems, response.InventoryItems...)
	}

	return inventoryItems, nil
}

func makeRequest(url string, headers map[string]string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer HttpUtil.CloseBody(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
