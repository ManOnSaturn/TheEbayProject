package ChromeClient

import (
	"sync"

	"github.com/imroc/req/v3"
)

var ChromeClient *req.Client
var chromeClientLock sync.Mutex

func GetChromeClient() *req.Client {
	chromeClientLock.Lock()
	if ChromeClient == nil {
		ChromeClient = req.DefaultClient().ImpersonateChrome().DisableKeepAlives()
	}
	chromeClientLock.Unlock()
	return ChromeClient
}
