package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var fullEndPoint = "https://dxchain.com/"
var emptyEndPoint = ""
var partialEndPoint = "google.com"

/*
	Test DialHTTPWithClient function to see if it can return the RPC Client object with
	desired properties to show it is created through HTTP method.

	Properties to check in RPC Client:
		1. writeConn -- *httpConn object returned from httpConnectFunc
		2. isHTTP   -- must be true since it is created through HTTP method
		3. connectFunc -- httpConnectFunc, can be checked by checking writeConn

	Properties to check in httpConn Object returned from httpConnectFunc
		1. req -- HTTP headers

	Properties to check in req created using the endpoint passed in
		1. Header: Content-Type -- should be application/json
		2. Header: Accept -- should be application/json

	Cases (in theory, those three cases should make no difference):
		1. end point with full structure (scheme, host, etc.)
		2. end point with nothing inside
		3. end point with host name only
*/
func TestDialHTTPWithClient(t *testing.T) {
	tables := []string{fullEndPoint, emptyEndPoint, partialEndPoint}
	var httpHeaderContentAccept = "application/json"

	for _, table := range tables {
		client, err := DialHTTPWithClient(table, new(http.Client))
		httpconnObj := client.writeConn
		isHTTP := client.isHTTP

		if err != nil {
			t.Errorf("Error: %s", err)
		}

		// Check if the RPC Client isHTTP
		if !isHTTP {
			t.Errorf("Calling HTTP Method: got isHttp: %t, expected isHTTP: %t", isHTTP, true)
		}

		// Check if writeConn is *httpConn object
		hc, err1 := httpconnObj.(*httpConn)
		if !err1 {
			t.Errorf("Calling HTTP Method: the client.writeConn supposed to be *httpConn object")
		}

		// Check request header
		req := hc.req
		contentType := req.Header.Get("Content-Type")
		accept := req.Header.Get("Accept")
		if contentType != httpHeaderContentAccept {
			t.Errorf("HTTP Request Header - got content type: %s, expected content type: %s",
				contentType, httpHeaderContentAccept)
		}

		if accept != "application/json" {
			t.Errorf("HTTP Request Header - got accept: %s, expected accept: %s",
				contentType, httpHeaderContentAccept)
		}
	}
}

/*
	Test DialHTTP Function to check if it correctly called DialHTTPWithClient function
	The test cases are exactly same as testing DialHTTPWithClient
*/
func TestDialHTTP(t *testing.T) {
	tables := []string{fullEndPoint, emptyEndPoint, partialEndPoint}
	var httpHeaderContentAccept = "application/json"

	for _, table := range tables {
		client, err := DialHTTP(table)
		httpconnObj := client.writeConn
		isHTTP := client.isHTTP

		if err != nil {
			t.Errorf("Error: %s", err)
		}

		// Check if the RPC Client isHTTP
		if !isHTTP {
			t.Errorf("Calling HTTP Method: got isHttp: %t, expected isHTTP: %t", isHTTP, true)
		}

		// Check if writeConn is *httpConn object
		hc, err1 := httpconnObj.(*httpConn)
		if !err1 {
			t.Errorf("Calling HTTP Method: the client.writeConn supposed to be *httpConn object")
		}

		// Check request header
		req := hc.req
		contentType := req.Header.Get("Content-Type")
		accept := req.Header.Get("Accept")
		if contentType != httpHeaderContentAccept {
			t.Errorf("HTTP Request Header - got content type: %s, expected content type: %s",
				contentType, httpHeaderContentAccept)
		}

		if accept != "application/json" {
			t.Errorf("HTTP Request Header - got accept: %s, expected accept: %s",
				contentType, httpHeaderContentAccept)
		}
	}
}

/*


  ____  _____  _____ _____ _____ _   _          _        _______ ______  _____ _______ _____           _____ ______  _____
 / __ \|  __ \|_   _/ ____|_   _| \ | |   /\   | |      |__   __|  ____|/ ____|__   __/ ____|   /\    / ____|  ____|/ ____|
| |  | | |__) | | || |  __  | | |  \| |  /  \  | |         | |  | |__  | (___    | | | |       /  \  | (___ | |__  | (___
| |  | |  _  /  | || | |_ | | | | . ` | / /\ \ | |         | |  |  __|  \___ \   | | | |      / /\ \  \___ \|  __|  \___ \
| |__| | | \ \ _| || |__| |_| |_| |\  |/ ____ \| |____     | |  | |____ ____) |  | | | |____ / ____ \ ____) | |____ ____) |
 \____/|_|  \_\_____\_____|_____|_| \_/_/    \_\______|    |_|  |______|_____/   |_|  \_____/_/    \_\_____/|______|_____/



*/

func TestHTTPErrorResponseWithDelete(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodDelete, contentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithPut(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodPut, contentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithMaxContentLength(t *testing.T) {
	body := make([]rune, maxRequestContentLength+1)
	testHTTPErrorResponse(t,
		http.MethodPost, contentType, string(body), http.StatusRequestEntityTooLarge)
}

func TestHTTPErrorResponseWithEmptyContentType(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodPost, "", "", http.StatusUnsupportedMediaType)
}

func TestHTTPErrorResponseWithValidRequest(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodPost, contentType, "", 0)
}

func testHTTPErrorResponse(t *testing.T, method, contentType, body string, expected int) {
	request := httptest.NewRequest(method, "http://url.com", strings.NewReader(body))
	request.Header.Set("content-type", contentType)
	if code, _ := validateRequest(request); code != expected {
		t.Fatalf("response code should be %d not %d", expected, code)
	}
}
