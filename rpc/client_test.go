package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/log"
)

var jsonrpcMessageTesting = jsonrpcMessage{
	"2.0",
	[]byte{6, 4},
	"rlp_modules",
	[]byte{},
	nil,
	[]byte{},
}

var requestOpTesting = requestOp{
	[]json.RawMessage{{6, 4}},
	nil,
	nil,
	nil,
}

/*
	Test if the jsonrpcMessage is a notification
	Cases:
		1. msg.ID == nil, msg.Method != "" -- true
		2. msg.ID != nil, msg.Method != "" -- false
		3. msg.ID == nil, msg.Method == "" -- false
*/
func TestIsNotification(t *testing.T) {
	tables := []struct {
		msgId          json.RawMessage
		msgMethod      string
		isNotification bool
	}{
		{nil, "rlp_modules", true},
		{[]byte{6, 4}, "rlp_modules", false},
		{nil, "", false},
	}

	for _, table := range tables {
		jsonrpcMessageTesting.ID, jsonrpcMessageTesting.Method = table.msgId, table.msgMethod
		msg := &jsonrpcMessageTesting
		result := msg.isNotification()
		if result != table.isNotification {
			t.Errorf("id: %v, method: %v, got %t, expected %t",
				table.msgId, table.msgMethod, result, table.isNotification)
		}
	}
}

/*
	Test if the jsonrpcMessage is valid
	Cases:
		1. len(msg.ID) > 0, msg.ID[0] != '{', msg.ID[0] != '[' -- true
		2. len(msg.ID) < 0, msg.ID[0] != '{', msg.ID[0] != '[' -- false
		4. len(msg.ID) > 0, msg.ID[0] == '{', msg.ID[0] != '[' -- false
		5. len(msg.ID) > 0, msg.ID[0] != '{', msg.ID[0] == '[' -- false
*/
func TestHasValidID(t *testing.T) {
	tables := []struct {
		msgId   json.RawMessage
		isValid bool
	}{
		{[]byte{'('}, true},
		{[]byte{}, false},
		{[]byte{'{'}, false},
		{[]byte{'['}, false},
	}

	for _, table := range tables {
		jsonrpcMessageTesting.ID = table.msgId
		msg := &jsonrpcMessageTesting
		result := msg.hasValidID()
		if result != table.isValid {
			t.Errorf("id: %s, got %t, expected %t", table.msgId, result, table.isValid)
		}
	}
}

/*
	Test if the jsonrpcMessage is a response
	Cases:
		1. Valid MSG ID, msg.Method == "", len(msg.Params) == 0 -- true
		2. Valid MSG ID, msg.Method != "", len(msg.Params) == 0 -- false
		3. Valid MSG ID, msg.Method == "", len(msg.Params) != 0 -- false
		4. Invalid MSG ID, msg.Method == "", len(msg.Params) == 0 -- false
*/
func TestIsResponse(t *testing.T) {
	tables := []struct {
		msgId      json.RawMessage
		msgMethod  string
		msgParams  json.RawMessage
		isResponse bool
	}{
		{[]byte{6, 4}, "", []byte{}, true},
		{[]byte{6, 4}, "rpc", []byte{}, false},
		{[]byte{6, 4}, "", []byte{'a', 'b', 'c'}, false},
		{[]byte{}, "", []byte{}, false},
	}

	for _, table := range tables {
		jsonrpcMessageTesting.ID = table.msgId
		jsonrpcMessageTesting.Method = table.msgMethod
		jsonrpcMessageTesting.Params = table.msgParams

		msg := &jsonrpcMessageTesting
		result := msg.isResponse()

		if result != table.isResponse {
			t.Errorf("id: %s, methods: %s, params: %s, got %t, expected %t",
				table.msgId, table.msgMethod, table.msgParams, result, table.isResponse)
		}
	}
}

// Test if jsonrpcMessage object can be transferred into json formatted string
func TestString(t *testing.T) {
	tables := []struct {
		version       string
		method        string
		id            json.RawMessage
		stringVersion string
	}{
		{"2.4", "testing", json.RawMessage{'6', '4'}, "{\"jsonrpc\":\"2.4\",\"id\":64,\"method\":\"testing\"}"},
		{"mountain", "data", json.RawMessage{'3', '4'}, "{\"jsonrpc\":\"mountain\",\"id\":34,\"method\":\"data\"}"},
	}

	for _, table := range tables {
		msg := newMessage(table.version, table.method, table.id)
		result := msg.String()
		if result != table.stringVersion {
			t.Errorf("version: %s, method: %s, id: %s, got %s, expected %s",
				table.version, table.method, table.id, result, table.stringVersion)
		}
	}
}

/*
	Test the wait function check to see if the function will return ctx.Err() if context got canceled
	Moreover, check to see if the proper response will be returned once op.resp is passed into channel

	Cases:
		1. context timeout 3 seconds, sends resp 1 seconds -- get response
		2. context timeout 1 seconds, sends resp 3 seconds -- get context error
*/
func TestWait(t *testing.T) {
	tables := []struct {
		cTimeOut time.Duration
		rTimeOut time.Duration
		response *jsonrpcMessage
	}{
		{time.Second * 3, time.Second, &jsonrpcMessageTesting},
		{time.Second, time.Second * 3, nil},
	}

	for _, table := range tables {
		rp := &requestOpTesting
		rp.resp = make(chan *jsonrpcMessage)
		ctx, _ := context.WithTimeout(context.Background(), table.cTimeOut)

		go func() {
			time.Sleep(table.rTimeOut)
			rp.resp <- &jsonrpcMessageTesting
		}()
		result, _ := rp.wait(ctx)
		if table.response != result {
			t.Errorf("cTimeOut: %d, rTimeOut %d, got %v, expected %v",
				table.cTimeOut, table.rTimeOut, result, table.response)
		}
	}
}

/*
	Test netID function to see if the id is correctly calculated and transferred into byte slice

	Cases (three different number of digits):
		1. idCounter == 0 	   -->  []byte{1}
		2. idCounter == 100    -->  []byte{1, 0, 1}
		3. idCounter == 18476  -->  []byte{1, 8, 4, 7, 7}
*/
func TestNextID(t *testing.T) {
	tables := []struct {
		idCounter       uint32
		returnResult    []byte
		idCounterResult uint32
	}{
		{0, []byte{'1'}, 1},
		{100, []byte{'1', '0', '1'}, 101},
		{18476, []byte{'1', '8', '4', '7', '7'}, 18477},
	}

	for _, table := range tables {
		c := clientObjectGenerator(false, table.idCounter)
		result := c.nextID()
		if bytes.Compare(result, table.returnResult) != 0 {
			t.Errorf("input idCounter: %v, got result %v, expected result %v",
				table.idCounter, result, table.returnResult)
		}
		if c.idCounter != table.idCounterResult {
			t.Errorf("input idCounter: %v, got new idCounter %v, expected new idCounter %v",
				table.idCounter, c.idCounter, table.idCounterResult)
		}
	}
}

/*
	Test the function SupportedModules with HTTP connection to check if the correct available services
	can be returned
*/
func TestSupportedModules(t *testing.T) {
	server := NewServer()

	// start test HTTP server
	hs, url := httpServerStart(server)
	defer httpServerStop(hs)

	client, err := Dial(url)
	if err != nil {
		t.Fatalf("Failed to connection to HTTP Test Server. Error: %s", err)
	}
	result, err := client.SupportedModules()
	if err != nil {
		t.Fatalf("Failed to get available services supported by the server. Error: %s", err)
	}

	if len(result) != 1 {
		t.Errorf("Server should have only 1 available service")
	}

	if _, ok := result["rpc"]; !ok {
		t.Errorf("Default Service: RPC should existed")
	}
}

/*
	Test function CallContext and Call to check if the service result can be obtained correctly
	The connection is established using DialInProc method (in-process connection)
*/
func TestCallService(t *testing.T) {
	var result int
	handler := serverHandler("calc", CalculatorService{1, 2})
	client := DialInProc(handler)
	err := client.CallContext(context.Background(), &result, "calc_add")

	if err != nil {
		t.Fatalf("Error on calling Calculator Service Add Method: %s", err)
	}
	if result != 3 {
		t.Errorf("Service: calc_add (1, 2). Got %d, expected: 3", result)
	}

	err = client.Call(&result, "calc_div", 6, 3)
	if err != nil {
		t.Fatalf("Error on calling Calculator Service Division Method: %s", err)
	}
	if result != 2 {
		t.Errorf("Service: calc_div (6, 3). Got %d, expected: 2", result)
	}

	err = client.Call(&result, "calc_mult", 3, 4)
	if err != nil {
		t.Fatalf("Error on calling Calculator Service Multiplication Method: %s", err)
	}
	if result != 12 {
		t.Errorf("Service: calc_mult (3, 4). Got %d, expected: 12", result)
	}
}

/*
	Test function newMessage to check if the jsonrpcMessage with correct field was created
*/
func TestNewMessage(t *testing.T) {
	client := clientObjectGenerator(false, 0)
	msg, err := client.newMessage("rpc_modules", 1, 2)
	if err != nil {
		t.Fatalf("Method: rpc_modules. Error: %s", err)
	}
	if bytes.Compare(msg.ID, json.RawMessage("1")) != 0 {
		t.Errorf("Method: rpc_modules. Got id: %d, expected 1", msg.ID)
	}
	if bytes.Compare(msg.Params, json.RawMessage("[1,2]")) != 0 {
		t.Errorf("Method: rpc_modules. Got parameters %s, expected [1, 2]",
			msg.Params)
	}
	// ***
	msg, err = client.newMessage("cal_add", []int{5, 6}, 7, "good")
	if err != nil {
		t.Fatalf("Method: cal_add. Error: %s", err)
	}
	if bytes.Compare(msg.ID, json.RawMessage("2")) != 0 {
		t.Errorf("Method: cal_add. Got id: %d, expected 2", msg.ID)
	}
	if bytes.Compare(msg.Params, json.RawMessage(`[[5,6],7,"good"]`)) != 0 {
		t.Errorf(`Method: cal_add. Got parameters %s, expected [[5,6],7,"good"]`,
			msg.Params)
	}
}

/*

 _____ _   _ _______ ______ _____  _   _          _        ______ _    _ _   _  _____ _______ _____ ____  _   _
|_   _| \ | |__   __|  ____|  __ \| \ | |   /\   | |      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
  | | |  \| |  | |  | |__  | |__) |  \| |  /  \  | |      | |__  | |  | |  \| | |       | |    | || |  | |  \| |
  | | | . ` |  | |  |  __| |  _  /| . ` | / /\ \ | |      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
 _| |_| |\  |  | |  | |____| | \ \| |\  |/ ____ \| |____  | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_____|_| \_|  |_|  |______|_|  \_\_| \_/_/    \_\______| |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|


*/

// start http server
func httpServerStart(srv *Server) (*httptest.Server, string) {
	// src implemented ServeHTTP method, which means
	// it can be passed in as handler
	var hs *httptest.Server
	hs = httptest.NewUnstartedServer(srv)
	hs.Start()
	url := "http" + "://" + hs.Listener.Addr().String()
	return hs, url
}

// close http server
func httpServerStop(hs *httptest.Server) {
	hs.Close()
}

func serverHandler(name string, rcvr interface{}) *Server {
	server := NewServer()
	server.RegisterName(name, rcvr)
	return server
}

// function that is used to create and initialize jsonrpcMessage
func newMessage(version string, method string, id json.RawMessage) *jsonrpcMessage {
	msg := new(jsonrpcMessage)
	msg.Version = version
	msg.Method = method
	msg.ID = id

	return msg
}

// function that is used to create and initialize RPC *Client object
func clientObjectGenerator(isHTTP bool, idCounter uint32) *Client {
	return &Client{
		idCounter:   idCounter,
		isHTTP:      isHTTP,
		close:       make(chan struct{}),
		closing:     make(chan struct{}),
		didClose:    make(chan struct{}),
		reconnected: make(chan net.Conn),
		readErr:     make(chan error),
		readResp:    make(chan []*jsonrpcMessage),
		requestOp:   make(chan *requestOp),
		sendDone:    make(chan error, 1),
		respWait:    make(map[string]*requestOp),
		subs:        make(map[string]*ClientSubscription),
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

func TestClientRequest(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	var resp Result
	if err := client.Call(&resp, "service_echo", "hello", 10, &Args{"world"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp, Result{"hello", 10, &Args{"world"}}) {
		t.Errorf("incorrect result %#v", resp)
	}
}

func TestClientBatchRequest(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	batch := []BatchElem{
		{
			Method: "service_echo",
			Args:   []interface{}{"hello", 10, &Args{"world"}},
			Result: new(Result),
		},
		{
			Method: "service_echo",
			Args:   []interface{}{"hello2", 11, &Args{"world"}},
			Result: new(Result),
		},
		{
			Method: "no_such_method",
			Args:   []interface{}{1, 2, 3},
			Result: new(int),
		},
	}
	if err := client.BatchCall(batch); err != nil {
		t.Fatal(err)
	}
	wantResult := []BatchElem{
		{
			Method: "service_echo",
			Args:   []interface{}{"hello", 10, &Args{"world"}},
			Result: &Result{"hello", 10, &Args{"world"}},
		},
		{
			Method: "service_echo",
			Args:   []interface{}{"hello2", 11, &Args{"world"}},
			Result: &Result{"hello2", 11, &Args{"world"}},
		},
		{
			Method: "no_such_method",
			Args:   []interface{}{1, 2, 3},
			Result: new(int),
			Error:  &jsonError{Code: -32601, Message: "The method no_such_method_ does not exist/is not available"},
		},
	}
	if !reflect.DeepEqual(batch, wantResult) {
		t.Errorf("batch results mismatch:\ngot %swant %s", spew.Sdump(batch), spew.Sdump(wantResult))
	}
}

// func TestClientCancelInproc(t *testing.T) { testClientCancel("inproc", t) }
func TestClientCancelWebsocket(t *testing.T) { testClientCancel("ws", t) }
func TestClientCancelHTTP(t *testing.T)      { testClientCancel("http", t) }
func TestClientCancelIPC(t *testing.T)       { testClientCancel("ipc", t) }

// This test checks that requests made through CallContext can be canceled by canceling
// the context.
func testClientCancel(transport string, t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()

	// What we want to achieve is that the context gets canceled
	// at various stages of request processing. The interesting cases
	// are:
	//  - cancel during dial
	//  - cancel while performing a HTTP request
	//  - cancel while waiting for a response
	//
	// To trigger those, the times are chosen such that connections
	// are killed within the deadline for every other call (maxKillTimeout
	// is 2x maxCancelTimeout).
	//
	// Once a connection is dead, there is a fair chance it won't connect
	// successfully because the accept is delayed by 1s.
	maxContextCancelTimeout := 300 * time.Millisecond
	fl := &flakeyListener{
		maxAcceptDelay: 1 * time.Second,
		maxKillTimeout: 600 * time.Millisecond,
	}

	var client *Client
	switch transport {
	case "ws", "http":
		c, hs := httpTestClient(server, transport, fl)
		defer hs.Close()
		client = c
	case "ipc":
		c, l := ipcTestClient(server, fl)
		defer l.Close()
		client = c
	default:
		panic("unknown transport: " + transport)
	}

	// These tests take a lot of time, run them all at once.
	// You probably want to run with -parallel 1 or comment out
	// the call to t.Parallel if you enable the logging.
	t.Parallel()

	// The actual test starts here.
	var (
		wg       sync.WaitGroup
		nreqs    = 10
		ncallers = 6
	)
	caller := func(index int) {
		defer wg.Done()
		for i := 0; i < nreqs; i++ {
			var (
				ctx     context.Context
				cancel  func()
				timeout = time.Duration(rand.Int63n(int64(maxContextCancelTimeout)))
			)
			if index < ncallers/2 {
				// For half of the callers, create a context without deadline
				// and cancel it later.
				ctx, cancel = context.WithCancel(context.Background())
				time.AfterFunc(timeout, cancel)
			} else {
				// For the other half, create a context with a deadline instead. This is
				// different because the context deadline is used to set the socket write
				// deadline.
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
			}
			// Now perform a call with the context.
			// The key thing here is that no call will ever complete successfully.
			err := client.CallContext(ctx, nil, "service_sleep", 2*maxContextCancelTimeout)
			if err != nil {
				log.Debug(fmt.Sprint("got expected error:", err))
			} else {
				t.Errorf("no error for call with %v wait time", timeout)
			}
			cancel()
		}
	}
	wg.Add(ncallers)
	for i := 0; i < ncallers; i++ {
		go caller(i)
	}
	wg.Wait()
}

func TestClientSubscribeInvalidArg(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	check := func(shouldPanic bool, arg interface{}) {
		defer func() {
			err := recover()
			if shouldPanic && err == nil {
				t.Errorf("EthSubscribe should've panicked for %#v", arg)
			}
			if !shouldPanic && err != nil {
				t.Errorf("EthSubscribe shouldn't have panicked for %#v", arg)
				buf := make([]byte, 1024*1024)
				buf = buf[:runtime.Stack(buf, false)]
				t.Error(err)
				t.Error(string(buf))
			}
		}()
		client.EthSubscribe(context.Background(), arg, "foo_bar")
	}
	check(true, nil)
	check(true, 1)
	check(true, (chan int)(nil))
	check(true, make(<-chan int))
	check(false, make(chan int))
	check(false, make(chan<- int))
}

func TestClientSubscribe(t *testing.T) {
	server := newTestServer("eth", new(NotificationTestService))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	nc := make(chan int)
	count := 10
	sub, err := client.EthSubscribe(context.Background(), nc, "someSubscription", count, 0)
	if err != nil {
		t.Fatal("can't subscribe:", err)
	}
	for i := 0; i < count; i++ {
		if val := <-nc; val != i {
			t.Fatalf("value mismatch: got %d, want %d", val, i)
		}
	}

	sub.Unsubscribe()
	select {
	case v := <-nc:
		t.Fatal("received value after unsubscribe:", v)
	case err := <-sub.Err():
		if err != nil {
			t.Fatalf("Err returned a non-nil error after explicit unsubscribe: %q", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("subscription not closed within 1s after unsubscribe")
	}
}

func TestClientSubscribeCustomNamespace(t *testing.T) {
	namespace := "custom"
	server := newTestServer(namespace, new(NotificationTestService))
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	nc := make(chan int)
	count := 10
	sub, err := client.Subscribe(context.Background(), namespace, nc, "someSubscription", count, 0)
	if err != nil {
		t.Fatal("can't subscribe:", err)
	}
	for i := 0; i < count; i++ {
		if val := <-nc; val != i {
			t.Fatalf("value mismatch: got %d, want %d", val, i)
		}
	}

	sub.Unsubscribe()
	select {
	case v := <-nc:
		t.Fatal("received value after unsubscribe:", v)
	case err := <-sub.Err():
		if err != nil {
			t.Fatalf("Err returned a non-nil error after explicit unsubscribe: %q", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("subscription not closed within 1s after unsubscribe")
	}
}

// In this test, the connection drops while EthSubscribe is
// waiting for a response.
func TestClientSubscribeClose(t *testing.T) {
	service := &NotificationTestService{
		gotHangSubscriptionReq:  make(chan struct{}),
		unblockHangSubscription: make(chan struct{}),
	}
	server := newTestServer("eth", service)
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	var (
		nc   = make(chan int)
		errc = make(chan error)
		sub  *ClientSubscription
		err  error
	)
	go func() {
		sub, err = client.EthSubscribe(context.Background(), nc, "hangSubscription", 999)
		errc <- err
	}()

	<-service.gotHangSubscriptionReq
	client.Close()
	service.unblockHangSubscription <- struct{}{}

	select {
	case err := <-errc:
		if err == nil {
			t.Errorf("EthSubscribe returned nil error after Close")
		}
		if sub != nil {
			t.Error("EthSubscribe returned non-nil subscription after Close")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("EthSubscribe did not return within 1s after Close")
	}
}

// This test checks that Client doesn't lock up when a single subscriber
// doesn't read subscription events.
func TestClientNotificationStorm(t *testing.T) {
	server := newTestServer("eth", new(NotificationTestService))
	defer server.Stop()

	doTest := func(count int, wantError bool) {
		client := DialInProc(server)
		defer client.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Subscribe on the server. It will start sending many notifications
		// very quickly.
		nc := make(chan int)
		sub, err := client.EthSubscribe(ctx, nc, "someSubscription", count, 0)
		if err != nil {
			t.Fatal("can't subscribe:", err)
		}
		defer sub.Unsubscribe()

		// Process each notification, try to run a call in between each of them.
		for i := 0; i < count; i++ {
			select {
			case val := <-nc:
				if val != i {
					t.Fatalf("(%d/%d) unexpected value %d", i, count, val)
				}
			case err := <-sub.Err():
				if wantError && err != ErrSubscriptionQueueOverflow {
					t.Fatalf("(%d/%d) got error %q, want %q", i, count, err, ErrSubscriptionQueueOverflow)
				} else if !wantError {
					t.Fatalf("(%d/%d) got unexpected error %q", i, count, err)
				}
				return
			}
			var r int
			err := client.CallContext(ctx, &r, "eth_echo", i)
			if err != nil {
				if !wantError {
					t.Fatalf("(%d/%d) call error: %v", i, count, err)
				}
				return
			}
		}
	}

	doTest(8000, false)
	doTest(10000, true)
}

func TestClientHTTP(t *testing.T) {
	server := newTestServer("service", new(Service))
	defer server.Stop()

	client, hs := httpTestClient(server, "http", nil)
	defer hs.Close()
	defer client.Close()

	// Launch concurrent requests.
	var (
		results    = make([]Result, 100)
		errc       = make(chan error)
		wantResult = Result{"a", 1, new(Args)}
	)
	defer client.Close()
	for i := range results {
		i := i
		go func() {
			errc <- client.Call(&results[i], "service_echo",
				wantResult.String, wantResult.Int, wantResult.Args)
		}()
	}

	// Wait for all of them to complete.
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for i := range results {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatal(err)
			}
		case <-timeout.C:
			t.Fatalf("timeout (got %d/%d) results)", i+1, len(results))
		}
	}

	// Check results.
	for i := range results {
		if !reflect.DeepEqual(results[i], wantResult) {
			t.Errorf("result %d mismatch: got %#v, want %#v", i, results[i], wantResult)
		}
	}
}

func TestClientReconnect(t *testing.T) {
	startServer := func(addr string) (*Server, net.Listener) {
		srv := newTestServer("service", new(Service))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		go http.Serve(l, srv.WebsocketHandler([]string{"*"}))
		return srv, l
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start a server and corresponding client.
	s1, l1 := startServer("127.0.0.1:0")
	client, err := DialContext(ctx, "ws://"+l1.Addr().String())
	if err != nil {
		t.Fatal("can't dial", err)
	}

	// Perform a call. This should work because the server is up.
	var resp Result
	if err := client.CallContext(ctx, &resp, "service_echo", "", 1, nil); err != nil {
		t.Fatal(err)
	}

	// Shut down the server and try calling again. It shouldn't work.
	l1.Close()
	s1.Stop()
	if err := client.CallContext(ctx, &resp, "service_echo", "", 2, nil); err == nil {
		t.Error("successful call while the server is down")
		t.Logf("resp: %#v", resp)
	}

	// Allow for some cool down time so we can listen on the same address again.
	time.Sleep(2 * time.Second)

	// Start it up again and call again. The connection should be reestablished.
	// We spawn multiple calls here to check whether this hangs somehow.
	s2, l2 := startServer(l1.Addr().String())
	defer l2.Close()
	defer s2.Stop()

	start := make(chan struct{})
	errors := make(chan error, 20)
	for i := 0; i < cap(errors); i++ {
		go func() {
			<-start
			var resp Result
			errors <- client.CallContext(ctx, &resp, "service_echo", "", 3, nil)
		}()
	}
	close(start)
	errcount := 0
	for i := 0; i < cap(errors); i++ {
		if err = <-errors; err != nil {
			errcount++
		}
	}
	t.Log("err:", err)
	if errcount > 1 {
		t.Errorf("expected one error after disconnect, got %d", errcount)
	}
}

func newTestServer(serviceName string, service interface{}) *Server {
	server := NewServer()
	if err := server.RegisterName(serviceName, service); err != nil {
		panic(err)
	}
	return server
}

func httpTestClient(srv *Server, transport string, fl *flakeyListener) (*Client, *httptest.Server) {
	// Create the HTTP server.
	var hs *httptest.Server
	switch transport {
	case "ws":
		hs = httptest.NewUnstartedServer(srv.WebsocketHandler([]string{"*"}))
	case "http":
		hs = httptest.NewUnstartedServer(srv)
	default:
		panic("unknown HTTP transport: " + transport)
	}
	// Wrap the listener if required.
	if fl != nil {
		fl.Listener = hs.Listener
		hs.Listener = fl
	}
	// Connect the client.
	hs.Start()
	client, err := Dial(transport + "://" + hs.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return client, hs
}

func ipcTestClient(srv *Server, fl *flakeyListener) (*Client, net.Listener) {
	// Listen on a random endpoint.
	endpoint := fmt.Sprintf("go-ethereum-test-ipc-%d-%d", os.Getpid(), rand.Int63())
	if runtime.GOOS == "windows" {
		endpoint = `\\.\pipe\` + endpoint
	} else {
		endpoint = os.TempDir() + "/" + endpoint
	}
	l, err := ipcListen(endpoint)
	if err != nil {
		panic(err)
	}
	// Connect the listener to the server.
	if fl != nil {
		fl.Listener = l
		l = fl
	}
	go srv.ServeListener(l)
	// Connect the client.
	client, err := Dial(endpoint)
	if err != nil {
		panic(err)
	}
	return client, l
}

// flakeyListener kills accepted connections after a random timeout.
type flakeyListener struct {
	net.Listener
	maxKillTimeout time.Duration
	maxAcceptDelay time.Duration
}

func (l *flakeyListener) Accept() (net.Conn, error) {
	delay := time.Duration(rand.Int63n(int64(l.maxAcceptDelay)))
	time.Sleep(delay)

	c, err := l.Listener.Accept()
	if err == nil {
		timeout := time.Duration(rand.Int63n(int64(l.maxKillTimeout)))
		time.AfterFunc(timeout, func() {
			log.Debug(fmt.Sprintf("killing conn %v after %v", c.LocalAddr(), timeout))
			c.Close()
		})
	}
	return c, err
}
