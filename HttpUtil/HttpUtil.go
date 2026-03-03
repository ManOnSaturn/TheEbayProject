package HttpUtil

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func CloseBody(body io.ReadCloser) {
	err := body.Close()
	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "Error closing body: %v\n", err)
	}
}

func GetRequestWithHeader(URL string, proxy *string) (*http.Response, error) {
	// Create a new HTTP request
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v\n", err)
	}

	// Set a custom User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")

	// Create HTTP client with proxy if provided
	client := &http.Client{}

	if proxy != nil && *proxy != "" {
		proxyUrl, err := url.Parse(*proxy)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy URL: %v\n", err)
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
		if resp != nil {
			CloseBody(resp.Body)
		}
		return resp, fmt.Errorf("error making request: %v", err)
	}

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		CloseBody(resp.Body)
		return nil, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	return resp, err
}
