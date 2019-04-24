package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"
)

var testServer = NewServer()

/*
	Test function NewServer to check if the object server with default parameters
	was successfully created.
	Default Parameter:
		run: 1
		registered service: rpc
		method associated with the service: modules
*/

var defaultServiceType = reflect.TypeOf(&RPCService{})

func TestNewServer(t *testing.T) {
	server := NewServer()

	if server.run != 1 {
		t.Errorf("Server is not running")
	}

	svc, ok := server.services["rpc"]
	if !ok {
		t.Errorf("Default service RPCService is not registered")
	}
	// name, typ, callbacks, subscriptions
	if svc.name != "rpc" {
		t.Errorf("Wrong service name. got: %s, expected rpc", svc.name)
	}

	if svc.typ != defaultServiceType {
		t.Errorf("Wrong receiver type. got %s, expected %s",
			svc.typ, defaultServiceType)
	}

	if len(svc.callbacks) != 1 {
		t.Errorf("Wrong number of callbacks, got %d, expected 1",
			len(svc.callbacks))
	}

	_, ok = svc.callbacks["modules"]

	if !ok {
		t.Errorf("The method: modules associated with the default service does not exist")
	}

	if len(svc.subscriptions) != 0 {
		t.Errorf("The default service should not contain any subscription")
	}
}

/*
	Test function Modules to check if it will return a list of services available
	on the server
*/
func TestModules(t *testing.T) {
	server := NewServer()
	rpcSvc := &RPCService{server}
	modules := rpcSvc.Modules()

	if len(modules) != 1 {
		t.Fatalf("For the default server, got number of modules %d, expected 1",
			len(modules))
	}
	version, ok := modules["rpc"]
	if !ok {
		t.Fatalf("rpc is the default service stored in the server, it should existed")
	}

	if version != "1.0" {
		t.Errorf("Got module version: %s, expected 1.0", version)
	}
}

/*
	Test function RegisterName to check if the services can be correctly stored into server
*/
type CalculatorService struct {
	a, b int
}

func (c CalculatorService) Add() int {
	return c.a + c.b
}

func (c CalculatorService) Div(a, b int) (int, error) {
	if b == 0 {
		err := fmt.Errorf("denominator cannot be 0")
		return 0, err
	}
	q := a / b
	return q, nil
}

func (c CalculatorService) Mult(a, b int) int {
	return a * b
}

func TestRegisterName(t *testing.T) {
	var cal = CalculatorService{1, 2}
	err := testServer.RegisterName("calculator", cal)
	if err != nil {
		t.Fatalf("Register CalculatorService. Error: %s", err)
	}

	// server should have two services currently: rpc and calculator
	if len(testServer.services) != 2 {
		t.Fatalf("Input server: testServer, should have 2 services registered")
	}

	svc, ok := testServer.services["calculator"]
	if !ok {
		t.Fatalf("Input server: testServer, should have service named calculator")
	}

	if svc.name != "calculator" {
		t.Errorf("Service: calculator. Got service name: %s, expected calculator",
			svc.name)
	}

	if svc.typ != reflect.TypeOf(cal) {
		t.Errorf("Service: calculator. Got receiver type: %s, expected %s",
			svc.typ, reflect.TypeOf(cal))
	}

	if len(svc.callbacks) != 3 {
		t.Errorf("Service: calculator. Got callbacks: %d, expected 3",
			len(svc.callbacks))
	}

	if len(svc.subscriptions) != 0 {
		t.Errorf("Service: calcualtor. Got subscriptiosn: %d, expected 0",
			len(svc.subscriptions))
	}

	methodList := []string{"add", "div", "mult"}
	for k, callback := range svc.callbacks {
		if !stringInSlice(k, methodList) {
			t.Fatalf("Method with name %s does not exist", k)
		}
		if callback.rcvr.Interface() != reflect.ValueOf(cal).Interface() {
			t.Errorf("Callback receiver value does not match. got %v, expected %v",
				callback.rcvr.Interface(), reflect.ValueOf(cal).Interface())
		}

		if k == "add" {
			if len(callback.argTypes) != 0 {
				t.Errorf("Callback function add. Got arguments number: %d, expected: %d",
					len(callback.argTypes), 0)
			}
		}
		if len(callback.argTypes) == 0 {
			if k != "add" {
				t.Errorf("Number of arguments: 0. Got method %s, expected: %s",
					k, "add")
			}
		} else {
			if len(callback.argTypes) != 2 {
				t.Errorf("Method: %s. Got number of arguments %d, expected: 2",
					k, len(callback.argTypes))
			}
		}

		if callback.hasCtx {
			t.Errorf("Method: %s does not have context as first argument",
				k)
		}

		if callback.isSubscribe {
			t.Errorf("Method %s is not subscribable", k)
		}

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

func stringInSlice(s string, list []string) bool {
	for _, val := range list {
		if s == val {
			return true
		}
	}
	return false
}

/*


  ____  _____  _____ _____ _____ _   _          _        _______ ______  _____ _______ _____           _____ ______  _____
 / __ \|  __ \|_   _/ ____|_   _| \ | |   /\   | |      |__   __|  ____|/ ____|__   __/ ____|   /\    / ____|  ____|/ ____|
| |  | | |__) | | || |  __  | | |  \| |  /  \  | |         | |  | |__  | (___    | | | |       /  \  | (___ | |__  | (___
| |  | |  _  /  | || | |_ | | | | . ` | / /\ \ | |         | |  |  __|  \___ \   | | | |      / /\ \  \___ \|  __|  \___ \
| |__| | | \ \ _| || |__| |_| |_| |\  |/ ____ \| |____     | |  | |____ ____) |  | | | |____ / ____ \ ____) | |____ ____) |
 \____/|_|  \_\_____\_____|_____|_| \_/_/    \_\______|    |_|  |______|_____/   |_|  \_____/_/    \_\_____/|______|_____/



*/

type Service struct{}

type Args struct {
	S string
}

func (s *Service) NoArgsRets() {
}

type Result struct {
	String string
	Int    int
	Args   *Args
}

func (s *Service) Echo(str string, i int, args *Args) Result {
	return Result{str, i, args}
}

func (s *Service) EchoWithCtx(ctx context.Context, str string, i int, args *Args) Result {
	return Result{str, i, args}
}

func (s *Service) Sleep(ctx context.Context, duration time.Duration) {
	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}
}

func (s *Service) Rets() (string, error) {
	return "", nil
}

func (s *Service) InvalidRets1() (error, string) {
	return nil, ""
}

func (s *Service) InvalidRets2() (string, string) {
	return "", ""
}

func (s *Service) InvalidRets3() (string, string, error) {
	return "", "", nil
}

func (s *Service) Subscription(ctx context.Context) (*Subscription, error) {
	return nil, nil
}

func TestServerRegisterName(t *testing.T) {
	server := NewServer()
	service := new(Service)

	if err := server.RegisterName("calc", service); err != nil {
		t.Fatalf("%v", err)
	}

	if len(server.services) != 2 {
		t.Fatalf("Expected 2 service entries, got %d", len(server.services))
	}

	svc, ok := server.services["calc"]
	if !ok {
		t.Fatalf("Expected service calc to be registered")
	}

	if len(svc.callbacks) != 5 {
		t.Errorf("Expected 5 callbacks for service 'calc', got %d", len(svc.callbacks))
	}

	if len(svc.subscriptions) != 1 {
		t.Errorf("Expected 1 subscription for service 'calc', got %d", len(svc.subscriptions))
	}
}

func testServerMethodExecution(t *testing.T, method string) {
	server := NewServer()
	service := new(Service)

	if err := server.RegisterName("test", service); err != nil {
		t.Fatalf("%v", err)
	}

	stringArg := "string arg"
	intArg := 1122
	argsArg := &Args{"abcde"}
	params := []interface{}{stringArg, intArg, argsArg}

	request := map[string]interface{}{
		"id":      12345,
		"method":  "test_" + method,
		"version": "2.0",
		"params":  params,
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation)

	out := json.NewEncoder(clientConn)
	in := json.NewDecoder(clientConn)

	if err := out.Encode(request); err != nil {
		t.Fatal(err)
	}

	response := jsonSuccessResponse{Result: &Result{}}
	if err := in.Decode(&response); err != nil {
		t.Fatal(err)
	}

	if result, ok := response.Result.(*Result); ok {
		if result.String != stringArg {
			t.Errorf("expected %s, got : %s\n", stringArg, result.String)
		}
		if result.Int != intArg {
			t.Errorf("expected %d, got %d\n", intArg, result.Int)
		}
		if !reflect.DeepEqual(result.Args, argsArg) {
			t.Errorf("expected %v, got %v\n", argsArg, result)
		}
	} else {
		t.Fatalf("invalid response: expected *Result - got: %T", response.Result)
	}
}

func TestServerMethodExecution(t *testing.T) {
	testServerMethodExecution(t, "echo")
}

func TestServerMethodWithCtx(t *testing.T) {
	testServerMethodExecution(t, "echoWithCtx")
}
