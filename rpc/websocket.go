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

/*
Example HTTP initial request headers
	GET ws://websocket.example.com/ HTTP/1.1
	Origin: http://example.com
	Connection: Upgrade
	Host: websocket.example.com
	Upgrade: websocket

Origin = https:// + host_name OR http:// + host_name
In other words, origin must be using http protocol because the websocket protocol is initialized
through HTTP connection
*/
package rpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/DxChainNetwork/godx/log"
	mapset "github.com/deckarep/golang-set"
	"golang.org/x/net/websocket"
)

// websocketJSONCodec is a custom JSON codec with payload size enforcement and
// special number parsing.
var websocketJSONCodec = websocket.Codec{
	// Marshal is the stock JSON marshaller used by the websocket library too.
	// convert the message into json format
	Marshal: func(v interface{}) ([]byte, byte, error) {
		msg, err := json.Marshal(v)
		return msg, websocket.TextFrame, err
	},
	// Unmarshal is a specialized unmarshaller to properly convert numbers.
	Unmarshal: func(msg []byte, payloadType byte, v interface{}) error {
		dec := json.NewDecoder(bytes.NewReader(msg))
		// it means unmarshall the number as interface{}
		dec.UseNumber()

		return dec.Decode(v)
	},
}

// WebsocketHandler returns a handler that serves JSON-RPC to WebSocket connections.
// WHY USE http.Server
// By default, golang does not support WebSocket connection. Therefore, the external package
// used HTTP server connection with Websocket handler
//
// allowedOrigins should be a comma-separated list of allowed origin URLs.
// To allow connections with any origin, pass "*".
func (srv *Server) WebsocketHandler(allowedOrigins []string) http.Handler {
	// even though it returns http.Handler
	// websocket.Server calls serveHTTP, which calls serveWebsocket
	return websocket.Server{
		// returns a function that is used to check if the request
		// origin is allowed to connect to the server
		Handshake: wsHandshakeValidator(allowedOrigins),

		// defines handler used for websocket connection
		// which is basically ServeCodec that allow both regular method call
		// and subscription method call
		Handler: func(conn *websocket.Conn) {
			// Create a custom encode/decode pair to enforce payload size and number encoding
			conn.MaxPayloadBytes = maxRequestContentLength

			// encoder & decoder are functions used to send/receive message through connection
			encoder := func(v interface{}) error {
				// send marshalled v (json format) through connection
				return websocketJSONCodec.Send(conn, v)
			}
			// read json from the connection
			decoder := func(v interface{}) error {
				return websocketJSONCodec.Receive(conn, v)
			}
			srv.ServeCodec(NewCodec(conn, encoder, decoder), OptionMethodInvocation|OptionSubscriptions)
		},
	}
}

// NewWSServer creates a new websocket RPC server around an API provider.
// Created a new HTTP server, with WebsocketHandler
// NOTE: WS originally is HTTP
//
// Deprecated: use Server.WebsocketHandler
func NewWSServer(allowedOrigins []string, srv *Server) *http.Server {
	return &http.Server{Handler: srv.WebsocketHandler(allowedOrigins)}
}

// wsHandshakeValidator returns a handler that verifies the origin during the
// websocket upgrade process. When a '*' is specified as an allowed origins all
// connections are accepted.
//
// allowedOrigins specified domains that is allowed to connect to the server
// similar to HTTP CORS
func wsHandshakeValidator(allowedOrigins []string) func(*websocket.Config, *http.Request) error {
	origins := mapset.NewSet()
	allowAllOrigins := false

	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAllOrigins = true
		}
		if origin != "" {
			origins.Add(strings.ToLower(origin))
		}
	}

	// allow localhost if no allowedOrigins are specified.
	//
	// add localhost and os http://hostname
	if len(origins.ToSlice()) == 0 {
		origins.Add("http://localhost")
		if hostname, err := os.Hostname(); err == nil {
			origins.Add("http://" + strings.ToLower(hostname))
		}
	}

	log.Debug(fmt.Sprintf("Allowed origin(s) for WS RPC interface %v\n", origins.ToSlice()))

	// handshake function returned
	// used to check if the request's origin is allowed to be connected
	// to the server
	f := func(cfg *websocket.Config, req *http.Request) error {
		origin := strings.ToLower(req.Header.Get("Origin"))
		if allowAllOrigins || origins.Contains(origin) {
			return nil
		}
		log.Warn(fmt.Sprintf("origin '%s' not allowed on WS-RPC interface\n", origin))
		return fmt.Errorf("origin %s not allowed", origin)
	}

	return f
}

// websocket configuration: origin + authorization header
func wsGetConfig(endpoint, origin string) (*websocket.Config, error) {
	// set origin if it is empty
	if origin == "" {
		var err error
		// returns host name reported by kernel (depend on the environment)
		// for example: mzhang.local, ip-172-xx-x-xx
		if origin, err = os.Hostname(); err != nil {
			return nil, err
		}

		// if the rawurl begins with wss, then add https to the Hostname; otherwise, add http
		// wss means secure connection, which is equivalent to https, where ws is equivalent to http
		if strings.HasPrefix(endpoint, "wss") {
			origin = "https://" + strings.ToLower(origin)
		} else {
			origin = "http://" + strings.ToLower(origin)
		}
	}

	// creates a new WebSocket config for client connection
	config, err := websocket.NewConfig(endpoint, origin)
	if err != nil {
		return nil, err
	}

	// config.Location --  websocket server address (struct URL)
	// config.Location.User -- user name and password
	//
	// Getting authentication information, add it to authorization header, and clear it
	if config.Location.User != nil {
		// the authorization is required to establish websocket handshake procedure
		b64auth := base64.StdEncoding.EncodeToString([]byte(config.Location.User.String()))
		config.Header.Add("Authorization", "Basic "+b64auth)
		// safety consideration ??
		config.Location.User = nil
	}
	return config, nil
}

// DialWebsocket creates a new RPC client that communicates with a JSON-RPC server
// that is listening on the given endpoint.
//
// The context is used for the initial connection establishment. It does not
// affect subsequent interactions with the client.
//
// origin is a header field defines the URL that originates a WebSocket request
// websocket is established through ordinary HTTP request and respond first
// origin is used to differentiate between websocket connections from different hosts
func DialWebsocket(ctx context.Context, endpoint, origin string) (*Client, error) {
	// getting websocket configuration
	// set origin and authorization header if user name and password is required
	config, err := wsGetConfig(endpoint, origin)
	if err != nil {
		return nil, err
	}

	wsConnectFunc := func(ctx context.Context) (net.Conn, error) {
		return wsDialContext(ctx, config)
	}

	// create an initilize new RPC client
	// with the connection function
	return newClient(ctx, wsConnectFunc)
}

// 1. Establish connection to server on TCP network that will last for 30 seconds
// 2. Based on this connection, create websocket connection to the address
//
// After called this function, the websocket connection is established
// and will be dead in 30 seconds
func wsDialContext(ctx context.Context, config *websocket.Config) (*websocket.Conn, error) {
	var conn net.Conn
	var err error
	switch config.Location.Scheme {
	case "ws":
		// connect to an address on TCP network (created a connection)
		conn, err = dialContext(ctx, "tcp", wsDialAddress(config.Location))
	case "wss":
		dialer := contextDialer(ctx)
		// tls is used to establish secure session
		// called dialer.Dial connect to the address on TCP network
		conn, err = tls.DialWithDialer(dialer, "tcp", wsDialAddress(config.Location), config.TlsConfig)
	default:
		err = websocket.ErrBadScheme
	}
	if err != nil {
		return nil, err
	}

	// creates a new WebSocket client connection over rwc
	// this step is to transfer the connection to the address
	// to websocket connection
	ws, err := websocket.NewClient(config, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ws, err
}

// specifies default port number for corresponded websocket scheme
var wsPortMap = map[string]string{"ws": "80", "wss": "443"}

// check if websocket server has port number included
// if not, add default post number to host
func wsDialAddress(location *url.URL) string {
	// if the url scheme is ws or wss
	if _, ok := wsPortMap[location.Scheme]; ok {
		// if the port was not found in location.Host, then add default port number
		// based on the url scheme
		if _, _, err := net.SplitHostPort(location.Host); err != nil {
			return net.JoinHostPort(location.Host, wsPortMap[location.Scheme])
		}
	}
	return location.Host
}

// when the server scheme is ws, non-secure connection
// connect to an address on TCP network, the connection will keepalive for 30 seconds
func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// dialer object contains options for connecting to an address
	// specified that the connection to an address will stay alive for 30 seconds
	d := &net.Dialer{KeepAlive: tcpKeepAliveInterval}
	// addr is host + port. ex: dxchain.com:8080
	// connect to an address on TCP network
	return d.DialContext(ctx, network, addr)
}

// create dialer object for wss server scheme with specified deadline
// dialer.Deadline specified the time that dial will be fail
func contextDialer(ctx context.Context) *net.Dialer {
	// tcpKeepAliveInterval is used for after Dial was successful
	dialer := &net.Dialer{Cancel: ctx.Done(), KeepAlive: tcpKeepAliveInterval}

	// Deadline specifies if the connection cannot be established within the time provided
	// then cancel
	if deadline, ok := ctx.Deadline(); ok {
		// if context passed in is Deadline context
		// the the dialer deadline to be context deadline
		dialer.Deadline = deadline
	} else {
		// default dialer Deadline is 10 seconds
		dialer.Deadline = time.Now().Add(defaultDialTimeout)
	}
	return dialer
}
