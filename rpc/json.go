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
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/DxChainNetwork/godx/log"
)

const (
	jsonrpcVersion           = "2.0"
	serviceMethodSeparator   = "_"
	subscribeMethodSuffix    = "_subscribe"
	unsubscribeMethodSuffix  = "_unsubscribe"
	notificationMethodSuffix = "_subscription"
)

// convert jsonrpcMessage (request) int json format
//
// simplified version, respond and error are not included
// this is just simply a request
// which is in same format as jsonrpcMessage format request
type jsonRequest struct {
	Method  string          `json:"method"`
	Version string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Payload json.RawMessage `json:"params,omitempty"`
}

type jsonSuccessResponse struct {
	Version string      `json:"jsonrpc"`
	Id      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result"`
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type jsonErrResponse struct {
	Version string      `json:"jsonrpc"`
	Id      interface{} `json:"id,omitempty"`
	Error   jsonError   `json:"error"`
}

type jsonSubscription struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result,omitempty"`
}

type jsonNotification struct {
	Version string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  jsonSubscription `json:"params"`
}

// jsonCodec reads and writes JSON-RPC messages to the underlying connection. It
// also has support for parsing arguments and serializing (result) objects.
type jsonCodec struct {
	closer sync.Once                 // close closed channel once
	closed chan interface{}          // closed on Close
	decMu  sync.Mutex                // guards the decoder
	decode func(v interface{}) error // decoder to allow multiple transports
	encMu  sync.Mutex                // guards the encoder
	encode func(v interface{}) error // encoder to allow multiple transports
	rw     io.ReadWriteCloser        // read write connection
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	return err.Code
}

// NewCodec creates a new RPC server codec with support for JSON-RPC 2.0 based
// on explicitly given encoding and decoding methods.
func NewCodec(rwc io.ReadWriteCloser, encode, decode func(v interface{}) error) ServerCodec {
	return &jsonCodec{
		closed: make(chan interface{}),
		encode: encode,
		decode: decode,
		rw:     rwc,
	}
}

// NewJSONCodec creates a new RPC server codec with support for JSON-RPC 2.0.
//
// json.NewEncoder and json.NewDecoder are ones used in RPC mostly
func NewJSONCodec(rwc io.ReadWriteCloser) ServerCodec {
	enc := json.NewEncoder(rwc)
	dec := json.NewDecoder(rwc)
	dec.UseNumber()

	return &jsonCodec{
		closed: make(chan interface{}),
		encode: enc.Encode,
		decode: dec.Decode,
		rw:     rwc,
	}
}

// isBatch returns true when the first non-whitespace characters is '['
//
// check if it is a list of json message
func isBatch(msg json.RawMessage) bool {
	for _, c := range msg {
		// skip insignificant whitespace (http://www.ietf.org/rfc/rfc4627.txt)
		// 0x20 -> space  0x09 -> Tab  0x0a -> LF (newline \n) 0x0d -> CR (carriage return \r)
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			continue
		}
		return c == '['
	}
	return false
}

// ReadRequestHeaders will read new requests without parsing the arguments. It will
// return a collection of requests, an indication if these requests are in batch
// form or an error when the incoming message could not be read/parsed.
//
// parse the request into rpcRequest format
func (c *jsonCodec) ReadRequestHeaders() ([]rpcRequest, bool, Error) {
	c.decMu.Lock()
	defer c.decMu.Unlock()

	var incomingMsg json.RawMessage
	// decode incoming message and stores in the incomingMsg
	if err := c.decode(&incomingMsg); err != nil {
		return nil, false, &invalidRequestError{err.Error()}
	}
	if isBatch(incomingMsg) {
		return parseBatchRequest(incomingMsg)
	}
	return parseRequest(incomingMsg)
}

// checkReqId returns an error when the given reqId isn't valid for RPC method calls.
// valid id's are strings, numbers or null
//
// check if request id is valid
func checkReqId(reqId json.RawMessage) error {
	// requestID cannot be empty
	if len(reqId) == 0 {
		return fmt.Errorf("missing request id")
	}
	// check if the reqId can be converted to float
	if _, err := strconv.ParseFloat(string(reqId), 64); err == nil {
		return nil
	}
	var str string

	// check if the reqId can be converted to string
	if err := json.Unmarshal(reqId, &str); err == nil {
		return nil
	}

	// if the ID cannot be converted into numbers or string, id is invalid
	return fmt.Errorf("invalid request id")
}

// parseRequest will parse a single request from the given RawMessage. It will return
// the parsed request, an indication if the request was a batch or an error when
// the request could not be parsed.
//
// parse the request getting from the client (through the connection defined
// in the jsonCodec) into rpcRequest format
// which will contain service and method the client is requesting
// and also tell if the client is request for a Method or subscription
func parseRequest(incomingMsg json.RawMessage) ([]rpcRequest, bool, Error) {
	var in jsonRequest

	// transfer the incomingMsg into jsonRequest format (did not change much)
	if err := json.Unmarshal(incomingMsg, &in); err != nil {
		return nil, false, &invalidMessageError{err.Error()}
	}
	// check if the id is valid. String or numbers only
	if err := checkReqId(in.Id); err != nil {
		return nil, false, &invalidMessageError{err.Error()}
	}

	// subscribe are special, they will always use `subscribeMethod` as first param in the payload
	// if the passed in Method contains _subscribe at the end, then it is asking for subscription
	//
	// subscription must contain params to specify which method wants to subscribe
	// ex: eth_subscribe newHeads --> service: eth  method: newHeads
	if strings.HasSuffix(in.Method, subscribeMethodSuffix) {
		reqs := []rpcRequest{{id: &in.Id, isPubSub: true}}
		if len(in.Payload) > 0 {
			// first param must be subscription name
			var subscribeMethod [1]string
			// convert json format payload into string and store into subscribeMethod
			// NOTE: it is not convert to string, it is convert into string array
			if err := json.Unmarshal(in.Payload, &subscribeMethod); err != nil {
				log.Debug(fmt.Sprintf("Unable to parse subscription method: %v\n", err))
				return nil, false, &invalidRequestError{"Unable to parse subscription request"}
			}

			// for subscription rpcRequest
			// params is same as subscribeMethod
			// the difference is the type
			reqs[0].service, reqs[0].method = strings.TrimSuffix(in.Method, subscribeMethodSuffix), subscribeMethod[0]
			reqs[0].params = in.Payload
			return reqs, false, nil
		}
		return nil, false, &invalidRequestError{"Unable to parse subscription request"}
	}

	// if the passed in Method contains _unsubscribe, then it is asking for unsubscription
	// no further action is needed to modify passed in data
	if strings.HasSuffix(in.Method, unsubscribeMethodSuffix) {
		return []rpcRequest{{id: &in.Id, isPubSub: true,
			method: in.Method, params: in.Payload}}, false, nil
	}

	// separate service and method
	// typical request: service_method ex- rpc_modules
	elems := strings.Split(in.Method, serviceMethodSeparator)
	if len(elems) != 2 {
		return nil, false, &methodNotFoundError{in.Method, ""}
	}

	// regular RPC call (no parameters)
	if len(in.Payload) == 0 {
		return []rpcRequest{{service: elems[0], method: elems[1], id: &in.Id}}, false, nil
	}

	// regular RPC call with parameters
	return []rpcRequest{{service: elems[0], method: elems[1], id: &in.Id, params: in.Payload}}, false, nil
}

// parseBatchRequest will parse a batch request into a collection of requests from the given RawMessage, an indication
// if the request was a batch or an error when the request could not be read.
//
// pretty much same to the parseRequest. The only difference is that this will return
// a list of rpcRequest
func parseBatchRequest(incomingMsg json.RawMessage) ([]rpcRequest, bool, Error) {
	var in []jsonRequest
	if err := json.Unmarshal(incomingMsg, &in); err != nil {
		return nil, false, &invalidMessageError{err.Error()}
	}

	requests := make([]rpcRequest, len(in))
	for i, r := range in {
		if err := checkReqId(r.Id); err != nil {
			return nil, false, &invalidMessageError{err.Error()}
		}

		id := &in[i].Id

		// subscribe are special, they will always use `subscriptionMethod` as first param in the payload
		if strings.HasSuffix(r.Method, subscribeMethodSuffix) {
			requests[i] = rpcRequest{id: id, isPubSub: true}
			if len(r.Payload) > 0 {
				// first param must be subscription name
				var subscribeMethod [1]string
				if err := json.Unmarshal(r.Payload, &subscribeMethod); err != nil {
					log.Debug(fmt.Sprintf("Unable to parse subscription method: %v\n", err))
					return nil, false, &invalidRequestError{"Unable to parse subscription request"}
				}

				requests[i].service, requests[i].method = strings.TrimSuffix(r.Method, subscribeMethodSuffix), subscribeMethod[0]
				requests[i].params = r.Payload
				continue
			}

			return nil, true, &invalidRequestError{"Unable to parse (un)subscribe request arguments"}
		}

		if strings.HasSuffix(r.Method, unsubscribeMethodSuffix) {
			requests[i] = rpcRequest{id: id, isPubSub: true, method: r.Method, params: r.Payload}
			continue
		}

		if len(r.Payload) == 0 {
			requests[i] = rpcRequest{id: id, params: nil}
		} else {
			requests[i] = rpcRequest{id: id, params: r.Payload}
		}
		if elem := strings.Split(r.Method, serviceMethodSeparator); len(elem) == 2 {
			requests[i].service, requests[i].method = elem[0], elem[1]
		} else {
			requests[i].err = &methodNotFoundError{r.Method, ""}
		}
	}

	return requests, true, nil
}

// ParseRequestArguments tries to parse the given params (json.RawMessage) with the given
// types. It returns the parsed values or an error when the parsing failed.
//
// this function returns the parsed parameter values
func (c *jsonCodec) ParseRequestArguments(argTypes []reflect.Type, params interface{}) ([]reflect.Value, Error) {
	// check if the passed in params can be transfer to json.RawMessage
	if args, ok := params.(json.RawMessage); !ok {
		return nil, &invalidParamsError{"Invalid params supplied"}
	} else {
		return parsePositionalArguments(args, argTypes)
	}
}

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, Error) {
	// Read beginning of the args array.
	// can be treated as parsing the json elements
	// checking if the first element in json data is [
	// NOTE: params must be array args, which means it will start with [
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, &invalidParamsError{"non-array args"}
	}
	// Read args.
	args := make([]reflect.Value, 0, len(types))
	// more indicates if there are more elements in the decoder
	// it loops through rawArgs
	for i := 0; dec.More(); i++ {
		// if the number of arguments is greater than the number of argument types, error
		if i >= len(types) {
			return nil, &invalidParamsError{fmt.Sprintf("too many arguments, want at most %d", len(types))}
		}
		argval := reflect.New(types[i])
		// decode read one value from the decoder and stores into the argval's underlying value
		// .Interface is used to get argval's underlying value
		if err := dec.Decode(argval.Interface()); err != nil {
			return nil, &invalidParamsError{fmt.Sprintf("invalid argument %d: %v", i, err)}
		}

		// check if argval is nil. If it is nil, it means an empty argument was passed in
		if argval.IsNil() && types[i].Kind() != reflect.Ptr {
			return nil, &invalidParamsError{fmt.Sprintf("missing value for required argument %d", i)}
		}

		// append the value pointer by argval to args
		args = append(args, argval.Elem())
	}
	// At this point, all valid argument are stored in args
	//
	// Read end of args array.
	//
	// there should still have one delim left in the reader, which is ]
	// if cannot found it, it means param is invalid
	if _, err := dec.Token(); err != nil {
		return nil, &invalidParamsError{err.Error()}
	}
	// Set any missing args to nil.
	// optional parameters contained in the method must be pointer
	// and must be located at the end
	for i := len(args); i < len(types); i++ {
		// check if the parameter is optional parameter, if not, error
		if types[i].Kind() != reflect.Ptr {
			return nil, &invalidParamsError{fmt.Sprintf("missing value for required argument %d", i)}
		}
		// otherwise, set to nil
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

// CreateResponse will create a JSON-RPC success response with the given id and reply as result.
func (c *jsonCodec) CreateResponse(id interface{}, reply interface{}) interface{} {
	return &jsonSuccessResponse{Version: jsonrpcVersion, Id: id, Result: reply}
}

// CreateErrorResponse will create a JSON-RPC error response with the given id and error.
func (c *jsonCodec) CreateErrorResponse(id interface{}, err Error) interface{} {
	return &jsonErrResponse{Version: jsonrpcVersion, Id: id, Error: jsonError{Code: err.ErrorCode(), Message: err.Error()}}
}

// CreateErrorResponseWithInfo will create a JSON-RPC error response with the given id and error.
// info is optional and contains additional information about the error. When an empty string is passed it is ignored.
func (c *jsonCodec) CreateErrorResponseWithInfo(id interface{}, err Error, info interface{}) interface{} {
	return &jsonErrResponse{Version: jsonrpcVersion, Id: id,
		Error: jsonError{Code: err.ErrorCode(), Message: err.Error(), Data: info}}
}

// CreateNotification will create a JSON-RPC notification with the given subscription id and event as params.
func (c *jsonCodec) CreateNotification(subid, namespace string, event interface{}) interface{} {
	return &jsonNotification{Version: jsonrpcVersion, Method: namespace + notificationMethodSuffix,
		Params: jsonSubscription{Subscription: subid, Result: event}}
}

// Write message to client
func (c *jsonCodec) Write(res interface{}) error {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	return c.encode(res)
}

// Close the underlying connection
func (c *jsonCodec) Close() {
	c.closer.Do(func() {
		close(c.closed)
		c.rw.Close()
	})
}

// Closed returns a channel which will be closed when Close is called
func (c *jsonCodec) Closed() <-chan interface{} {
	return c.closed
}
