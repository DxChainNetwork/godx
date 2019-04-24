package rpc

import (
	"net/url"
	"testing"
)

/*
	Test to see if the correct port number is added to the host
	Cases:
		1. HTTP URL   -- return original host
		2. HTTPS URL  -- return original host
		3. WS URL with Port -- return original host
		4. WSS URL with Port -- return original host
		5. WS URL without port -- return host + 80
		6. WSS URL without port -- return host + 443
		7. Empty URL -- return empty
*/
func TestWsDialAddress(t *testing.T) {
	tables := []struct {
		rawurl     string
		resultHost string
	}{
		{"https://dxchain.com/", "dxchain.com"},
		{"http://explorer.dxchain.com/", "explorer.dxchain.com"},
		{"wss://dxchain.com:8080/", "dxchain.com:8080"},
		{"ws://explorer.dxchain.com:1688/", "explorer.dxchain.com:1688"},
		{"wss://dxchain.com/", "dxchain.com:443"},
		{"ws://explorer.dxchain.com/", "explorer.dxchain.com:80"},
	}

	for _, table := range tables {
		formatUrl, err := url.Parse(table.rawurl)
		if err != nil {
			t.Errorf("Error while paring the rawurl: %s", err)
		} else {
			host := wsDialAddress(formatUrl)
			if host != table.resultHost {
				t.Errorf("Input rawurl %s, got host %s, expecte host %s",
					table.rawurl, host, table.resultHost)
			}
		}
	}
}
