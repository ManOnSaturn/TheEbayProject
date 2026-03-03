package Ebay

import (
	"Scraper/HttpUtil"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	appSettings = map[string]string{
		"client_id":     "MattiaRi-Darkanne-PRD-d4d994de1-c57e2e21",
		"client_secret": "PRD-4d994de1550a-49cd-4e94-a961-d07c",
		"ruName":        "Mattia_Ripoli-MattiaRi-Darkan-xwbwvpae",
	}
	refreshToken = "v^1.1#i^1#I^3#r^1#f^0#p^3#t^Ul4xMF8xMDowNEQyNzY5NDMxMDM1NUE0Mzg1NEUyMUREN0Y1RDMzMV8wXzEjRV4yNjA="
)

var expirationTime time.Time
var lock sync.Mutex
var accessToken string

func getAccessToken() string {
	lock.Lock()
	if accessToken == "" || time.Now().After(expirationTime) {
		tokenResponse, err := getAuthTokenRefreshed()
		if err != nil {
			panic(err)
		}
		accessToken = tokenResponse["access_token"].(string)
		expiresIn := tokenResponse["expires_in"].(float64)
		seconds := time.Duration(expiresIn) * time.Second
		expirationTime = time.Now().Add(seconds)
	}
	lock.Unlock()
	return accessToken
}

func getAuthTokenRefreshed() (map[string]interface{}, error) {
	// Create the auth header
	authHeaderData := appSettings["client_id"] + ":" + appSettings["client_secret"]
	encodedAuthHeader := base64.StdEncoding.EncodeToString([]byte(authHeaderData))

	// Prepare headers
	headers := map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": "Basic " + encodedAuthHeader,
	}

	// Prepare body data
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", refreshToken)
	body.Set("redirect_uri", appSettings["ruName"])
	body.Set("scope", "https://api.ebay.com/oauth/api_scope https://api.ebay.com/oauth/api_scope/sell.marketing.readonly https://api.ebay.com/oauth/api_scope/sell.marketing https://api.ebay.com/oauth/api_scope/sell.inventory.readonly https://api.ebay.com/oauth/api_scope/sell.inventory https://api.ebay.com/oauth/api_scope/sell.account.readonly https://api.ebay.com/oauth/api_scope/sell.account https://api.ebay.com/oauth/api_scope/sell.fulfillment.readonly https://api.ebay.com/oauth/api_scope/sell.fulfillment https://api.ebay.com/oauth/api_scope/sell.analytics.readonly https://api.ebay.com/oauth/api_scope/sell.finances https://api.ebay.com/oauth/api_scope/sell.payment.dispute https://api.ebay.com/oauth/api_scope/commerce.identity.readonly https://api.ebay.com/oauth/api_scope/sell.reputation https://api.ebay.com/oauth/api_scope/sell.reputation.readonly https://api.ebay.com/oauth/api_scope/commerce.notification.subscription https://api.ebay.com/oauth/api_scope/commerce.notification.subscription.readonly https://api.ebay.com/oauth/api_scope/sell.stores https://api.ebay.com/oauth/api_scope/sell.stores.readonly")

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.ebay.com/identity/v1/oauth2/token", bytes.NewBufferString(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer HttpUtil.CloseBody(resp.Body)

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return result, nil
}
