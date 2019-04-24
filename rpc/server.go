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
	Function handle():
		Procedures on handling regular RPC method call:
			1. Check number of arguments passed through the request against
               number of arguments needed for the service method
			2. Prepare arguments: pass in receiver value as first argument. Then check
			   if the method has context as first argument, then the rest of arguments
			3. Pass in the arguments and execute the method
			4. If method does not return, response nil
			5. If method responded error, return error response
			6. Otherwise, return responses returned from the method


		Procedures on handling RPC Subscription method call:
			NOTE: for the context passed in, it contains value, which is Notifier
			1. Prepare arguments: pass in receiver and context. Then the rest of arguments
			2. Pass in arguments and execute the subscription method. It will return a subscription object
			3. Create a function that gets a notifier from the context value, and activate the subscription
			   based on the subscription id (getting from the subscription newly created)

			Once this function was called (do not know when it will be called yet), it will activate
			the newly created subscription and send the buffered data to the client if having any

		Procedures on handling RPC Unsubscription method call:
			1. Get the arguments, which is the subscription id
			2. Get notifier from the context and call unsubscribe function
			   to remove the subscription from the active subscription list

	Difference between ServeCodec and ServeSingleRequest
		1. ServeCodec does not allow user-defined context to be passed in.
           Since the context passed in is the root one, therefore, the connection will not have timeout.
		   However, for `ServeSingleRequest`, user is allowed to pass in timeout or deadline context.

		2. ServeSingleRequest will return immediately after processed a single request,
		   where ServeCodec will return only when server stopped
		   (this part defined in serveRequest function)
*/

package rpc

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/DxChainNetwork/godx/log"
	mapset "github.com/deckarep/golang-set"
)

const MetadataApi = "rpc"

// CodecOption specifies which type of messages this codec supports
type CodecOption int

const (
	// OptionMethodInvocation is an indication that the codec supports RPC method calls
	OptionMethodInvocation CodecOption = 1 << iota // 01

	// OptionSubscriptions is an indication that the codec suports RPC notifications
	OptionSubscriptions = 1 << iota // support pub sub   10
)

// NewServer will create a new server instance with no registered handlers.
//
//will create one service called rpc, which has the callbacks modules, returns a list of
// services available in the server
func NewServer() *Server {
	server := &Server{
		services: make(serviceRegistry),
		codecs:   mapset.NewSet(),
		run:      1, // 1 means the server is currently running. If not 1, means server stopped
	}

	// register a default service which will provide meta information about the RPC service such as the services and
	// methods it offers.
	//
	// default service is RPCService which has method Modules
	// for each server, it must has a method to return all services provided by the server
	rpcService := &RPCService{server}
	server.RegisterName(MetadataApi, rpcService)

	return server
}

// RPCService gives meta information about the server.
// e.g. gives information about the loaded modules.
type RPCService struct {
	server *Server
}

// Modules returns the list of RPC services with their version number
//
// returns services available for the server
func (s *RPCService) Modules() map[string]string {
	modules := make(map[string]string)
	for name := range s.server.services {
		modules[name] = "1.0"
	}
	return modules
}

// RegisterName will create a service for the given rcvr type under the given name. When no methods on the given rcvr
// match the criteria to be either a RPC method or a subscription an error is returned. Otherwise a new service is
// created and added to the service collection this server instance serves.
//
// rcvr stands for receiver, svc stands for service
// one server can have many services (services serviceRegistry)
// one service can have many callbacks and subscriptions
func (s *Server) RegisterName(name string, rcvr interface{}) error {
	if s.services == nil {
		// serviceRegistry: string map to service structure
		s.services = make(serviceRegistry)
	}

	// declare new service pointer
	svc := new(service)

	// stores receiver type
	svc.typ = reflect.TypeOf(rcvr)

	// getting receiver value
	rcvrVal := reflect.ValueOf(rcvr)

	// specified name of the service cannot be empty
	if name == "" {
		return fmt.Errorf("no service name for type %s", svc.typ.String())
	}
	// check if the receiver structure is exported
	// Indirect returns the value that rcrVal pointed to
	// if v is not a pointer, indirect returns v itself
	// returns the name of the Structure ex: RPCService
	// why not Elem(): receiver may not be a pointer
	if !isExported(reflect.Indirect(rcvrVal).Type().Name()) {
		return fmt.Errorf("%s is not exported", reflect.Indirect(rcvrVal).Type().Name())
	}

	// NOTE: service type (svc.typ) is equivalent to receiver's type
	methods, subscriptions := suitableCallbacks(rcvrVal, svc.typ)

	if len(methods) == 0 && len(subscriptions) == 0 {
		return fmt.Errorf("Service %T doesn't have any suitable methods/subscriptions to expose", rcvr)
	}

	// already a previous service register under given name, merge methods/subscriptions
	//
	// if the service already existed, updated them
	if regsvc, present := s.services[name]; present {
		for _, m := range methods {
			regsvc.callbacks[formatName(m.method.Name)] = m
		}
		for _, s := range subscriptions {
			regsvc.subscriptions[formatName(s.method.Name)] = s
		}
		return nil
	}

	// Otherwise, stores into service, and then services
	svc.name = name
	svc.callbacks, svc.subscriptions = methods, subscriptions

	s.services[svc.name] = svc
	return nil
}

// serveRequest will reads requests from the codec, calls the RPC callback and
// writes the response to the given codec.
//
// If singleShot is true it will process a single request, otherwise it will handle
// requests until the codec returns an error when reading a request (in most cases
// an EOF). It executes requests in parallel when singleShot is false.
//
// CodecOption Selection:
// 00 -> does not support RPC method and RPC notification
// 01 -> only support RPC method
// 10 -> only support RPC notification
// 11 -> support both RPC method and notification
//
// Error Checking. Parse and Handle the request
func (s *Server) serveRequest(ctx context.Context, codec ServerCodec, singleShot bool, options CodecOption) error {
	var pend sync.WaitGroup

	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Error(string(buf))
		}
		s.codecsMu.Lock()
		s.codecs.Remove(codec)
		s.codecsMu.Unlock()
	}()

	//	ctx, cancel := context.WithCancel(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// if the codec supports notification include a notifier that callbacks can use
	// to send notification to clients. It is tied to the codec/connection. If the
	// connection is closed the notifier will stop and cancels all active subscriptions.
	//
	// if the codec support RPC notification
	// why use & operator instead of comparing directly?
	// because the CodecOption can be 11, which also support Notification
	if options&OptionSubscriptions == OptionSubscriptions {
		// if codec support notification, stores notifier and notiferKey
		// to the context
		ctx = context.WithValue(ctx, notifierKey{}, newNotifier(codec))
	}

	s.codecsMu.Lock()
	if atomic.LoadInt32(&s.run) != 1 { // server stopped
		s.codecsMu.Unlock()
		return &shutdownError{}
	}

	// add the codec to the server's codecs set
	s.codecs.Add(codec)
	s.codecsMu.Unlock()

	// test if the server is ordered to stop
	for atomic.LoadInt32(&s.run) == 1 {
		// read request from the connection and parse it into serverRequest format
		reqs, batch, err := s.readRequest(codec)

		// handle parsing error (parsing into rpcRequest format)
		if err != nil {
			// If a parsing error occurred, send an error to the client
			//
			// when codec calls Write function, it writes the response to the client through the connection
			if err.Error() != "EOF" {
				log.Debug(fmt.Sprintf("read error %v\n", err))
				codec.Write(codec.CreateErrorResponse(nil, err))
			}
			// Error or end of stream, wait for requests and tear down
			//
			// wait when a collection of goroutines were finished
			pend.Wait()
			return nil
		}

		// check if server is ordered to shutdown and return an error
		// telling the client that his request failed.
		if atomic.LoadInt32(&s.run) != 1 {
			err = &shutdownError{}
			if batch {
				resps := make([]interface{}, len(reqs))
				for i, r := range reqs {
					resps[i] = codec.CreateErrorResponse(&r.id, err)
				}
				codec.Write(resps)
			} else {
				codec.Write(codec.CreateErrorResponse(&reqs[0].id, err))
			}
			return nil
		}

		// If a single shot request is executing, run and return immediately
		if singleShot {
			if batch {
				s.execBatch(ctx, codec, reqs)
			} else {
				s.exec(ctx, codec, reqs[0])
			}
			return nil
		}
		// For multi-shot connections, start a goroutine to serve and loop back
		pend.Add(1)

		go func(reqs []*serverRequest, batch bool) {
			defer pend.Done()
			if batch {
				s.execBatch(ctx, codec, reqs)
			} else {
				s.exec(ctx, codec, reqs[0])
			}
		}(reqs, batch)
	}
	return nil
}

// ServeCodec reads incoming requests from codec, calls the appropriate callback and writes the
// response back using the given codec. It will block until the codec is closed or the server is
// stopped. In either case the codec is closed.
func (s *Server) ServeCodec(codec ServerCodec, options CodecOption) {
	defer codec.Close()
	s.serveRequest(context.Background(), codec, false, options)
}

// ServeSingleRequest reads and processes a single RPC request from the given codec. It will not
// close the codec unless a non-recoverable error has occurred. Note, this method will return after
// a single request has been processed!
func (s *Server) ServeSingleRequest(ctx context.Context, codec ServerCodec, options CodecOption) {
	s.serveRequest(ctx, codec, true, options)
}

// Stop will stop reading new requests, wait for stopPendingRequestTimeout to allow pending requests to finish,
// close all codecs which will cancel pending requests/subscriptions.
func (s *Server) Stop() {
	if atomic.CompareAndSwapInt32(&s.run, 1, 0) {
		log.Debug("RPC Server shutdown initiatied")
		s.codecsMu.Lock()
		defer s.codecsMu.Unlock()
		s.codecs.Each(func(c interface{}) bool {
			c.(ServerCodec).Close()
			return true
		})
	}
}

// createSubscription will call the subscription callback and returns the subscription id or error.
//
// call the method to create Subscription object, and return its' id
func (s *Server) createSubscription(ctx context.Context, c ServerCodec, req *serverRequest) (ID, error) {
	// subscription have as first argument the context following optional arguments
	//
	// arguments will be receiver, context with value, and rest of arguments
	// NOTE: the first argument should be context for subscription method
	args := []reflect.Value{req.callb.rcvr, reflect.ValueOf(ctx)}
	args = append(args, req.args...)
	reply := req.callb.method.Func.Call(args)

	// if the subscription method returned error
	if !reply[1].IsNil() { // subscription creation failed
		return "", reply[1].Interface().(error)
	}

	// return the subscription ID
	return reply[0].Interface().(*Subscription).ID, nil
}

// handle executes a request and returns the response from the callback.
func (s *Server) handle(ctx context.Context, codec ServerCodec, req *serverRequest) (interface{}, func()) {
	// error when parsing client's request to serverRequest format
	if req.err != nil {
		return codec.CreateErrorResponse(&req.id, req.err), nil
	}

	// if the request is for canceling subscription
	if req.isUnsubscribe { // cancel subscription, first param must be the subscription id
		if len(req.args) >= 1 && req.args[0].Kind() == reflect.String {
			notifier, supported := NotifierFromContext(ctx)

			// check if it is supported
			if !supported { // interface doesn't support subscriptions (e.g. http)
				return codec.CreateErrorResponse(&req.id, &callbackError{ErrNotificationsUnsupported.Error()}), nil
			}

			// getting the subscription id from the arguments
			subid := ID(req.args[0].String())

			// close the subscription error channel and remove the subscription from the active subscription
			// list based on the subscription id
			if err := notifier.unsubscribe(subid); err != nil {
				return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
			}

			return codec.CreateResponse(req.id, true), nil
		}
		return codec.CreateErrorResponse(&req.id, &invalidParamsError{"Expected subscription id as first argument"}), nil
	}

	// if the request method is for subscription
	if req.callb.isSubscribe {
		// Call the subscription method to create Subscription Object and get its' id
		subid, err := s.createSubscription(ctx, codec, req)
		if err != nil {
			return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
		}

		// active the subscription after the sub id was successfully sent to the client
		//
		// active subscription and send buffered data
		activateSub := func() {
			// notifier is passed through the context, from the function serveRquest
			notifier, _ := NotifierFromContext(ctx)

			// active the subscription
			notifier.activate(subid, req.svcname)
		}

		return codec.CreateResponse(req.id, subid), activateSub
	}

	// regular RPC call, prepare arguments
	//
	// check if number of arguments passed into the request is same as
	// number of arguments required for the method
	if len(req.args) != len(req.callb.argTypes) {
		rpcErr := &invalidParamsError{fmt.Sprintf("%s%s%s expects %d parameters, got %d",
			req.svcname, serviceMethodSeparator, req.callb.method.Name,
			len(req.callb.argTypes), len(req.args))}
		return codec.CreateErrorResponse(&req.id, rpcErr), nil
	}

	// method format: method (receiver, args ...)
	// add receiver to the arguments
	arguments := []reflect.Value{req.callb.rcvr}

	// while parsingArguments, if the first argument is context, then it was escaped
	// therefore, need to added back first
	if req.callb.hasCtx {
		arguments = append(arguments, reflect.ValueOf(ctx))
	}
	// append rest of arguments if existed
	// NOTE: req.args contained parsed arguments
	if len(req.args) > 0 {
		arguments = append(arguments, req.args...)
	}

	// execute RPC method and return result
	//
	// pass in parameters and execute the method
	reply := req.callb.method.Func.Call(arguments)

	// this means method has 0 return
	if len(reply) == 0 {
		return codec.CreateResponse(req.id, nil), nil
	}

	// if the method has an error return field,
	// check if the error returned by the function is nil
	// if not, send error response
	if req.callb.errPos >= 0 {
		if !reply[req.callb.errPos].IsNil() {
			e := reply[req.callb.errPos].Interface().(error)
			res := codec.CreateErrorResponse(&req.id, &callbackError{e.Error()})
			return res, nil
		}
	}
	// create and return the response based on the result
	return codec.CreateResponse(req.id, reply[0].Interface()), nil
}

// exec executes the given request and writes the result back using the codec.
func (s *Server) exec(ctx context.Context, codec ServerCodec, req *serverRequest) {
	var response interface{}
	var callback func()

	// parsing client's request into serverRequest error
	if req.err != nil {
		response = codec.CreateErrorResponse(&req.id, req.err)
	} else {
		response, callback = s.handle(ctx, codec, req)
	}

	if err := codec.Write(response); err != nil {
		log.Error(fmt.Sprintf("%v\n", err))
		codec.Close()
	}

	// when request was a subscribe request this allows these subscriptions to be activated
	// only subscription request has callback function
	if callback != nil {
		callback()
	}
}

// execBatch executes the given requests and writes the result back using the codec.
// It will only write the response back when the last request is processed.
func (s *Server) execBatch(ctx context.Context, codec ServerCodec, requests []*serverRequest) {
	responses := make([]interface{}, len(requests))
	var callbacks []func()
	for i, req := range requests {
		if req.err != nil {
			responses[i] = codec.CreateErrorResponse(&req.id, req.err)
		} else {
			var callback func()
			if responses[i], callback = s.handle(ctx, codec, req); callback != nil {
				callbacks = append(callbacks, callback)
			}
		}
	}

	if err := codec.Write(responses); err != nil {
		log.Error(fmt.Sprintf("%v\n", err))
		codec.Close()
	}

	// when request holds one of more subscribe requests this allows these subscriptions to be activated
	for _, c := range callbacks {
		c()
	}
}

// readRequest requests the next (batch) request from the codec. It will return the collection
// of requests, an indication if the request was a batch, the invalid request identifier and an
// error when the request could not be read/parsed.
//
// all errors are stored in serverRequest object under err field
// other than error occurred when calling ReadRequestHeaders function
func (s *Server) readRequest(codec ServerCodec) ([]*serverRequest, bool, Error) {
	// for JsonCodec, which is the one defined in RPC (json.go)
	// ReadRequestHeaders() is basically used to convert the incoming request into rpcRequest format
	// and also return if the request is batch request or not
	reqs, batch, err := codec.ReadRequestHeaders()
	if err != nil {
		return nil, batch, err
	}

	requests := make([]*serverRequest, len(reqs))

	// verify requests
	for i, r := range reqs {
		var ok bool
		var svc *service

		if r.err != nil {
			requests[i] = &serverRequest{id: r.id, err: r.err}
			continue
		}

		// if the request is used for unsubscription
		// get id, isUnsubscribe, args
		if r.isPubSub && strings.HasSuffix(r.method, unsubscribeMethodSuffix) {
			requests[i] = &serverRequest{id: r.id, isUnsubscribe: true}
			argTypes := []reflect.Type{reflect.TypeOf("")} // expect subscription id as first arg
			// args will be []reflect.Value stores the actual value of the argument with
			// its' originally type
			if args, err := codec.ParseRequestArguments(argTypes, r.params); err == nil {
				requests[i].args = args
			} else {
				requests[i].err = &invalidParamsError{err.Error()}
			}
			continue
		}

		// Based on the service request (name of the service), get the corresponded service
		// from server's available services (registerName function)
		if svc, ok = s.services[r.service]; !ok { // rpc method isn't available
			requests[i] = &serverRequest{id: r.id, err: &methodNotFoundError{r.service, r.method}}
			continue
		}

		// if the request is PubSub
		// get id, svcname, callb (where callb stores in the service subscriptions field)
		if r.isPubSub { // eth_subscribe, r.method contains the subscription method name
			// getting the callback function from service subscriptions field
			if callb, ok := svc.subscriptions[r.method]; ok {
				requests[i] = &serverRequest{id: r.id, svcname: svc.name, callb: callb}
				if r.params != nil && len(callb.argTypes) > 0 {
					argTypes := []reflect.Type{reflect.TypeOf("")}
					argTypes = append(argTypes, callb.argTypes...)
					if args, err := codec.ParseRequestArguments(argTypes, r.params); err == nil {
						requests[i].args = args[1:] // first one is service.method name which isn't an actual argument
					} else {
						requests[i].err = &invalidParamsError{err.Error()}
					}
				}
			} else {
				requests[i] = &serverRequest{id: r.id, err: &methodNotFoundError{r.service, r.method}}
			}
			continue
		}

		// remotely call a method (request is not PubSub)
		// get the callback from the service
		if callb, ok := svc.callbacks[r.method]; ok { // lookup RPC method
			requests[i] = &serverRequest{id: r.id, svcname: svc.name, callb: callb}
			// if the callback function has arguments and the request passed in has parameters
			// meaning the parameters are used as method arguments
			if r.params != nil && len(callb.argTypes) > 0 {
				if args, err := codec.ParseRequestArguments(callb.argTypes, r.params); err == nil {
					requests[i].args = args
				} else {
					requests[i].err = &invalidParamsError{err.Error()}
				}
			}
			continue
		}

		requests[i] = &serverRequest{id: r.id, err: &methodNotFoundError{r.service, r.method}}
	}

	return requests, batch, nil
}
