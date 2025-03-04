package Proxy

import (
	"Scraper/DataTypes"
	"encoding/json"
	"fmt"
	"net/http"
)

func GetProxies() []string {
	// Define the API URL
	url := "https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=1&page_size=100&valid=true&country_code__in=IT,GB,DE,EG"

	// Create a new HTTP request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil
	}

	// Set the Authorization header
	request.Header.Set("Authorization", "xfif3zqrn2ntnjrkaqhk34lu5n4khuq2plezy10j")

	// Send the request using the default HTTP client
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil
	}
	defer resp.Body.Close() // Ensure the response body is closed

	// Check if the response status code is OK (200)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Request failed with status code: %d\n", resp.StatusCode)
		return nil
	}

	// Decode the JSON response into a struct
	var proxyListResponse DataTypes.ProxyListResponse
	err = json.NewDecoder(resp.Body).Decode(&proxyListResponse)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return nil
	}

	var proxies []string
	for _, result := range proxyListResponse.Results {
		proxies = append(proxies, fmt.Sprintf("http://%s:%s@%s:%d", result.Username, result.Password, result.ProxyAddress, result.Port))
	}
	return proxies
}
