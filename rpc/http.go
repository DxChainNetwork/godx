// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/log"
	"github.com/rs/cors"
)

const (
	maxRequestContentLength = 1024 * 512
)

var (
	// https://www.jsonrpc.org/historical/json-rpc-over-http.html#id13
	acceptedContentTypes = []string{"application/json", "application/json-rpc", "application/jsonrequest"}
	contentType          = acceptedContentTypes[0]
	nullAddr, _          = net.ResolveTCPAddr("tcp", "127.0.0.1:0")
)

type httpConn struct {
	// HTTP Client created using new(http.Client), used to execute the http request
	// ex: http.Client.Do(req)
	client *http.Client
	// Newly Created HTTP Request, POST Method to the rawurl passed in (endpoint)
	req *http.Request
	// the purpose of closeOnce is used to close the httpConn.closed channel
	// sync.Once -> object that will perform exactly one action
	// it has only one method called Do(f func())
	closeOnce sync.Once
	closed    chan struct{}
}

// httpConn is treated specially by Client.
func (hc *httpConn) LocalAddr() net.Addr              { return nullAddr }
func (hc *httpConn) RemoteAddr() net.Addr             { return nullAddr }
func (hc *httpConn) SetReadDeadline(time.Time) error  { return nil }
func (hc *httpConn) SetWriteDeadline(time.Time) error { return nil }
func (hc *httpConn) SetDeadline(time.Time) error      { return nil }
func (hc *httpConn) Write([]byte) (int, error)        { panic("Write called") }

func (hc *httpConn) Read(b []byte) (int, error) {
	<-hc.closed
	return 0, io.EOF
}

// close the hc.closed channel
// meaning nothing can be sent to hc.closed channel
func (hc *httpConn) Close() error {
	// for one httpConn object, close(hc.closed) can only be performed once
	// future calls of this function Do will still return but without performing the
	// function contained inside
	hc.closeOnce.Do(func() { close(hc.closed) })
	return nil
}

// HTTPTimeouts represents the configuration params for the HTTP RPC server.
type HTTPTimeouts struct {
	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body.
	//
	// Because ReadTimeout does not let Handlers make per-request
	// decisions on each request body's acceptable deadline or
	// upload rate, most users will prefer to use
	// ReadHeaderTimeout. It is valid to use them both.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, ReadHeaderTimeout is used.
	IdleTimeout time.Duration
}

// DefaultHTTPTimeouts represents the default timeout values used if further
// configuration is not provided.
var DefaultHTTPTimeouts = HTTPTimeouts{
	ReadTimeout:  30 * time.Second,
	WriteTimeout: 30 * time.Second,
	IdleTimeout:  120 * time.Second,
}

// DialHTTPWithClient creates a new RPC client that connects to an RPC server over HTTP
// using the provided HTTP Client.
func DialHTTPWithClient(endpoint string, client *http.Client) (*Client, error) {
	// Create a post request to the endpoint (rawurl)
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// set request header: contentType == application/json
	// contentType -> indicates the request body is json encoded
	// accept -> sets the output type to be json
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)

	// root context
	initctx := context.Background()

	// defines http connection function
	httpConnectFunc := func(context.Context) (net.Conn, error) {
		return &httpConn{client: client, req: req, closed: make(chan struct{})}, nil
	}

	// create and return new Client
	return newClient(initctx, httpConnectFunc)
}

// DialHTTP creates a new RPC client that connects to an RPC server over HTTP.
// where endpoint is just the rawurl passed in
//
// The purpose of this function is just to create an Ethereum Client (defined in client.go)
// where the connectFunction returns a httpConn object, which included http client, post request
// to the rawurl, and a channel
func DialHTTP(endpoint string) (*Client, error) {
	return DialHTTPWithClient(endpoint, new(http.Client))
}

// call doRequest to send http request
// then send the response message address through channel (op.resp)
func (c *Client) sendHTTP(ctx context.Context, op *requestOp, msg interface{}) error {
	// hc = *httpConn object
	hc := c.writeConn.(*httpConn)

	// start the http request and get the response body
	respBody, err := hc.doRequest(ctx, msg)
	if respBody != nil {
		// close the connection after function return
		defer respBody.Close()
	}

	if err != nil {
		// respBody may contain specific connection fail description
		if respBody != nil {
			buf := new(bytes.Buffer)
			if _, err2 := buf.ReadFrom(respBody); err2 == nil {
				return fmt.Errorf("%v %v", err, buf.String())
			}
		}
		return err
	}
	var respmsg jsonrpcMessage
	// NOTE respBody will be json format
	// reads Json encoded data, and set it to respmsg (directly change the value in the address)
	if err := json.NewDecoder(respBody).Decode(&respmsg); err != nil {
		return err
	}

	// send the response message address through the channel
	op.resp <- &respmsg
	return nil
}

// Similar to sendHTTP, both batch calls and single calls uses doRequest
func (c *Client) sendBatchHTTP(ctx context.Context, op *requestOp, msgs []*jsonrpcMessage) error {
	// hc = *httpConn
	hc := c.writeConn.(*httpConn)
	respBody, err := hc.doRequest(ctx, msgs)
	if err != nil {
		return err
	}
	defer respBody.Close()
	var respmsgs []jsonrpcMessage
	if err := json.NewDecoder(respBody).Decode(&respmsgs); err != nil {
		return err
	}
	// send each response to channel
	for i := 0; i < len(respmsgs); i++ {
		op.resp <- &respmsgs[i]
	}
	return nil
}

// set request body, ContentLength and start the request
func (hc *httpConn) doRequest(ctx context.Context, msg interface{}) (io.ReadCloser, error) {
	// transfer the message into json format
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	// created a shadow request using the context manually created
	// http package has its' default context
	// if do not want to use the default context
	// req.WithContext function can be used to create a copy of the request, but with
	// user defined context
	req := hc.req.WithContext(ctx)

	// set the message sent to the endpoint and define content length
	// req.Body is io.ReadCloser object, which contains reader and closer
	// NopCloser only wraps io.Reader, and returned ReadCloser object
	req.Body = ioutil.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))

	// start HTTP request
	resp, err := hc.client.Do(req)
	if err != nil {
		return nil, err
	}

	// Check the response status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Body, errors.New(resp.Status)
	}
	return resp.Body, nil
}

// httpReadWriteNopCloser wraps a io.Reader and io.Writer with a NOP Close method.
type httpReadWriteNopCloser struct {
	io.Reader
	io.Writer
}

// Close does nothing and returns always nil
func (t *httpReadWriteNopCloser) Close() error {
	return nil
}

// NewHTTPServer creates a new HTTP RPC server around an API provider.
// Created and returned new http.Server Object
// defined timeouts, CORS settings, and allowedHosts
//
// Deprecated: Server implements http.Handler
func NewHTTPServer(cors []string, vhosts []string, timeouts HTTPTimeouts, srv *Server) *http.Server {
	// Wrap the CORS-handler within a host-handler
	//
	// first defines a list of allowed origins in HTTP handler
	// then defined a list of allowed host in HTTP handler
	handler := newCorsHandler(srv, cors)
	handler = newVHostHandler(vhosts, handler)

	// Make sure timeout values are meaningful
	if timeouts.ReadTimeout < time.Second {
		log.Warn("Sanitizing invalid HTTP read timeout", "provided", timeouts.ReadTimeout, "updated", DefaultHTTPTimeouts.ReadTimeout)
		timeouts.ReadTimeout = DefaultHTTPTimeouts.ReadTimeout
	}
	if timeouts.WriteTimeout < time.Second {
		log.Warn("Sanitizing invalid HTTP write timeout", "provided", timeouts.WriteTimeout, "updated", DefaultHTTPTimeouts.WriteTimeout)
		timeouts.WriteTimeout = DefaultHTTPTimeouts.WriteTimeout
	}
	if timeouts.IdleTimeout < time.Second {
		log.Warn("Sanitizing invalid HTTP idle timeout", "provided", timeouts.IdleTimeout, "updated", DefaultHTTPTimeouts.IdleTimeout)
		timeouts.IdleTimeout = DefaultHTTPTimeouts.IdleTimeout
	}
	// Bundle and start the HTTP server
	return &http.Server{
		Handler:      handler,
		ReadTimeout:  timeouts.ReadTimeout,
		WriteTimeout: timeouts.WriteTimeout,
		IdleTimeout:  timeouts.IdleTimeout,
	}
}

// ServeHTTP serves JSON-RPC requests over HTTP.
//
// The Server Object Implemented ServeHTTP method
// which means Server can be used as HTTP handler
//
// http.ResponseWriter assembles the HTTP server's response
// by writing to it, data will be sent to HTTP client
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Permit dumb empty requests for remote health-checks (AWS)
	if r.Method == http.MethodGet && r.ContentLength == 0 && r.URL.RawQuery == "" {
		return
	}

	// Check if the request is valid
	if code, err := validateRequest(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	// All checks passed, create a codec that reads direct from the request body
	// untilEOF and writes the response to w and order the server to process a
	// single request.
	//
	// Since it only support regular method call
	// Therefore, the only place those value will be used is when the method call
	// has context.Context as parameter, those values will be passed in
	ctx := r.Context()
	ctx = context.WithValue(ctx, "remote", r.RemoteAddr)
	ctx = context.WithValue(ctx, "scheme", r.Proto)
	ctx = context.WithValue(ctx, "local", r.Host)
	if ua := r.Header.Get("User-Agent"); ua != "" {
		ctx = context.WithValue(ctx, "User-Agent", ua)
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		ctx = context.WithValue(ctx, "Origin", origin)
	}

	// set up reading limit be 512 bytes because the max request content length
	// allowed to be passed into a single request is 512 bytes
	body := io.LimitReader(r.Body, maxRequestContentLength)

	// defines the JsonCodec
	// where body is the reader contains HTTP request
	// w is the writer, if write to it, the response will be sent to client
	codec := NewJSONCodec(&httpReadWriteNopCloser{body, w})
	defer codec.Close()

	w.Header().Set("content-type", contentType)
	srv.ServeSingleRequest(ctx, codec, OptionMethodInvocation)
}

// validateRequest returns a non-zero response code and error message if the
// request is invalid. 0 response code means no error
//
// 1. PUT and Delete Method is not allowed
// 2. Request ContentLength should be smaller or equal to 512 bytes
// 3. The content type must be allowed content type defined in acceptedContentTypes
func validateRequest(r *http.Request) (int, error) {
	// PUT and Delete methods are not allowed
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return http.StatusMethodNotAllowed, errors.New("method not allowed")
	}

	// request content should be smaller than 512 bytes
	if r.ContentLength > maxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, maxRequestContentLength)
		return http.StatusRequestEntityTooLarge, err
	}

	// Allow OPTIONS (regardless of content-type)
	// Options method is used to describe the communication options for the target resource
	// therefore, the content-type can be ignored because not trying to get information or post
	// information
	if r.Method == http.MethodOptions {
		return 0, nil
	}
	// Check content-type
	if mt, _, err := mime.ParseMediaType(r.Header.Get("content-type")); err == nil {
		for _, accepted := range acceptedContentTypes {
			if accepted == mt {
				return 0, nil
			}
		}
	}
	// Invalid content-type
	err := fmt.Errorf("invalid content type, only %s is supported", contentType)
	return http.StatusUnsupportedMediaType, err
}

// defines CORS settings for HTTP handler
// mainly defined a list of allowedOrigins that are allowed to make cross-domain request
func newCorsHandler(srv *Server, allowedOrigins []string) http.Handler {
	// disable CORS support if user has not specified a custom CORS configuration
	if len(allowedOrigins) == 0 {
		return srv
	}
	c := cors.New(cors.Options{
		// a list of origins a cross-domain request can be executed from
		//
		// RPC HOST HTTP Server: www.rpc.com
		// then, origins contained in allowedOrigins will be allowed to
		// send request to www.rpc.com
		AllowedOrigins: allowedOrigins,
		// methods client is allowed to use in cross-domain requests
		AllowedMethods: []string{http.MethodPost, http.MethodGet},
		MaxAge:         600,
		AllowedHeaders: []string{"*"},
	})
	return c.Handler(srv)
}

// virtualHostHandler is a handler which validates the Host-header of incoming requests.
// The virtualHostHandler can prevent DNS rebinding attacks, which do not utilize CORS-headers,
// since they do in-domain requests against the RPC api. Instead, we can see on the Host-header
// which domain was used, and validate that against a whitelist.
//
// virtualHostHandler is mainly used for error checking
// check if the host is allowed
type virtualHostHandler struct {
	vhosts map[string]struct{}
	next   http.Handler
}

// ServeHTTP serves JSON-RPC requests over HTTP, implements http.Handler
//
// if it is called through NewHTTPServer defined above,
// then it is just did some error checking and then called (src *Server) ServeHTTP()
func (h *virtualHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// if r.Host is not set, we can continue serving since a browser would set the Host header
	if r.Host == "" {
		h.next.ServeHTTP(w, r)
		return
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// Either invalid (too many colons) or no port specified
		host = r.Host
	}

	// if ip address can be obtained, calls serve
	if ipAddr := net.ParseIP(host); ipAddr != nil {
		// It's an IP address, we can serve that
		h.next.ServeHTTP(w, r)
		return

	}
	// Not an ip address, but a hostname. Need to validate
	// check if "*" existed, if it existed, then it means allowed all hosts
	if _, exist := h.vhosts["*"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	// check if the host contained in allowed hosts
	if _, exist := h.vhosts[host]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "invalid host specified", http.StatusForbidden)
}

// vhosts contained a list of allowedHost
// basically, this function stored each vhosts as a key that match to an empty structure
// the http handler is also stored in virtualHostHandler structure
func newVHostHandler(vhosts []string, next http.Handler) http.Handler {
	vhostMap := make(map[string]struct{})
	for _, allowedHost := range vhosts {
		vhostMap[strings.ToLower(allowedHost)] = struct{}{}
	}
	return &virtualHostHandler{vhostMap, next}
}
