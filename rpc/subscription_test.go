package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

var notifierTest = newNotifier(nil)

/*
	Test function CreateSubscription to check if newly created Subscription object
	will be placed under inactive filed
*/
func TestCreateSubscription(t *testing.T) {
	sub := notifierTest.CreateSubscription()
	if _, ok := notifierTest.inactive[sub.ID]; !ok {
		t.Errorf("Newly created Susbscription %v was not contained in inactive list",
			sub.ID)
	}
}

/*
	Test function activate to check if the subscription will be moved from inactive list
	to active list
*/
func TestActivateSubscription(t *testing.T) {
	sub := notifierTest.CreateSubscription()
	notifierTest.activate(sub.ID, "eth")
	if _, ok := notifierTest.active[sub.ID]; !ok {
		t.Errorf("Activated subscription %v, cannot be found in active list", sub.ID)
	}

	if _, ok := notifierTest.inactive[sub.ID]; ok {
		t.Errorf("Subscription %v should be removed from inactive list", sub.ID)
	}
}

/*
	Test function unsubscribe to check if the subscription will be moved from active list
	to inactive list
*/
func TestUnsubscribe(t *testing.T) {
	// create and activate subscription
	sub := notifierTest.CreateSubscription()
	notifierTest.activate(sub.ID, "eth")

	// unsubscribe
	err := notifierTest.unsubscribe(sub.ID)

	if err != nil {
		t.Fatalf("Subscription ID: %v, Error: %s", sub.ID, err)
	}

	if _, ok := notifierTest.inactive[sub.ID]; ok {
		t.Errorf("Subscription %v should be removed from inactive list", sub.ID)
	}

	if _, ok := notifierTest.active[sub.ID]; ok {
		t.Errorf("Subscription %v should be removed from active list", sub.ID)
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

type NotificationTestService struct {
	mu           sync.Mutex
	unsubscribed bool

	gotHangSubscriptionReq  chan struct{}
	unblockHangSubscription chan struct{}
}

func (s *NotificationTestService) Echo(i int) int {
	return i
}

func (s *NotificationTestService) wasUnsubCallbackCalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unsubscribed
}

func (s *NotificationTestService) Unsubscribe(subid string) {
	s.mu.Lock()
	s.unsubscribed = true
	s.mu.Unlock()
}

func (s *NotificationTestService) SomeSubscription(ctx context.Context, n, val int) (*Subscription, error) {
	notifier, supported := NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}

	// by explicitly creating an subscription we make sure that the subscription id is send back to the client
	// before the first subscription.Notify is called. Otherwise the events might be send before the response
	// for the eth_subscribe method.
	subscription := notifier.CreateSubscription()

	go func() {
		// test expects n events, if we begin sending event immediately some events
		// will probably be dropped since the subscription ID might not be send to
		// the client.
		time.Sleep(5 * time.Second)
		for i := 0; i < n; i++ {
			if err := notifier.Notify(subscription.ID, val+i); err != nil {
				return
			}
		}

		select {
		case <-notifier.Closed():
			s.mu.Lock()
			s.unsubscribed = true
			s.mu.Unlock()
		case <-subscription.Err():
			s.mu.Lock()
			s.unsubscribed = true
			s.mu.Unlock()
		}
	}()

	return subscription, nil
}

// HangSubscription blocks on s.unblockHangSubscription before
// sending anything.
func (s *NotificationTestService) HangSubscription(ctx context.Context, val int) (*Subscription, error) {
	notifier, supported := NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}

	s.gotHangSubscriptionReq <- struct{}{}
	<-s.unblockHangSubscription
	subscription := notifier.CreateSubscription()

	go func() {
		notifier.Notify(subscription.ID, val)
	}()
	return subscription, nil
}

func TestNotifications(t *testing.T) {
	server := NewServer()
	service := &NotificationTestService{}

	if err := server.RegisterName("eth", service); err != nil {
		t.Fatalf("unable to register test service %v", err)
	}

	clientConn, serverConn := net.Pipe()

	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation|OptionSubscriptions)

	out := json.NewEncoder(clientConn)
	in := json.NewDecoder(clientConn)

	n := 5
	val := 12345
	request := map[string]interface{}{
		"id":      1,
		"method":  "eth_subscribe",
		"version": "2.0",
		"params":  []interface{}{"someSubscription", n, val},
	}

	// create subscription
	if err := out.Encode(request); err != nil {
		t.Fatal(err)
	}

	var subid string
	response := jsonSuccessResponse{Result: subid}
	if err := in.Decode(&response); err != nil {
		t.Fatal(err)
	}

	var ok bool
	if _, ok = response.Result.(string); !ok {
		t.Fatalf("expected subscription id, got %T", response.Result)
	}

	for i := 0; i < n; i++ {
		var notification jsonNotification
		if err := in.Decode(&notification); err != nil {
			t.Fatalf("%v", err)
		}

		if int(notification.Params.Result.(float64)) != val+i {
			t.Fatalf("expected %d, got %d", val+i, notification.Params.Result)
		}
	}

	clientConn.Close() // causes notification unsubscribe callback to be called
	time.Sleep(1 * time.Second)

	if !service.wasUnsubCallbackCalled() {
		t.Error("unsubscribe callback not called after closing connection")
	}
}

func waitForMessages(t *testing.T, in *json.Decoder, successes chan<- jsonSuccessResponse,
	failures chan<- jsonErrResponse, notifications chan<- jsonNotification, errors chan<- error) {

	// read and parse server messages
	for {
		var rmsg json.RawMessage
		if err := in.Decode(&rmsg); err != nil {
			return
		}

		var responses []map[string]interface{}
		if rmsg[0] == '[' {
			if err := json.Unmarshal(rmsg, &responses); err != nil {
				errors <- fmt.Errorf("Received invalid message: %s", rmsg)
				return
			}
		} else {
			var msg map[string]interface{}
			if err := json.Unmarshal(rmsg, &msg); err != nil {
				errors <- fmt.Errorf("Received invalid message: %s", rmsg)
				return
			}
			responses = append(responses, msg)
		}

		for _, msg := range responses {
			// determine what kind of msg was received and broadcast
			// it to over the corresponding channel
			if _, found := msg["result"]; found {
				successes <- jsonSuccessResponse{
					Version: msg["jsonrpc"].(string),
					Id:      msg["id"],
					Result:  msg["result"],
				}
				continue
			}
			if _, found := msg["error"]; found {
				params := msg["params"].(map[string]interface{})
				failures <- jsonErrResponse{
					Version: msg["jsonrpc"].(string),
					Id:      msg["id"],
					Error:   jsonError{int(params["subscription"].(float64)), params["message"].(string), params["data"]},
				}
				continue
			}
			if _, found := msg["params"]; found {
				params := msg["params"].(map[string]interface{})
				notifications <- jsonNotification{
					Version: msg["jsonrpc"].(string),
					Method:  msg["method"].(string),
					Params:  jsonSubscription{params["subscription"].(string), params["result"]},
				}
				continue
			}
			errors <- fmt.Errorf("Received invalid message: %s", msg)
		}
	}
}

// TestSubscriptionMultipleNamespaces ensures that subscriptions can exists
// for multiple different namespaces.
func TestSubscriptionMultipleNamespaces(t *testing.T) {
	var (
		namespaces             = []string{"eth", "shh", "bzz"}
		server                 = NewServer()
		service                = NotificationTestService{}
		clientConn, serverConn = net.Pipe()

		out           = json.NewEncoder(clientConn)
		in            = json.NewDecoder(clientConn)
		successes     = make(chan jsonSuccessResponse)
		failures      = make(chan jsonErrResponse)
		notifications = make(chan jsonNotification)

		errors = make(chan error, 10)
	)

	// setup and start server
	for _, namespace := range namespaces {
		if err := server.RegisterName(namespace, &service); err != nil {
			t.Fatalf("unable to register test service %v", err)
		}
	}

	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation|OptionSubscriptions)
	defer server.Stop()

	// wait for message and write them to the given channels
	go waitForMessages(t, in, successes, failures, notifications, errors)

	// create subscriptions one by one
	n := 3
	for i, namespace := range namespaces {
		request := map[string]interface{}{
			"id":      i,
			"method":  fmt.Sprintf("%s_subscribe", namespace),
			"version": "2.0",
			"params":  []interface{}{"someSubscription", n, i},
		}

		if err := out.Encode(&request); err != nil {
			t.Fatalf("Could not create subscription: %v", err)
		}
	}

	// create all subscriptions in 1 batch
	var requests []interface{}
	for i, namespace := range namespaces {
		requests = append(requests, map[string]interface{}{
			"id":      i,
			"method":  fmt.Sprintf("%s_subscribe", namespace),
			"version": "2.0",
			"params":  []interface{}{"someSubscription", n, i},
		})
	}

	if err := out.Encode(&requests); err != nil {
		t.Fatalf("Could not create subscription in batch form: %v", err)
	}

	timeout := time.After(30 * time.Second)
	subids := make(map[string]string, 2*len(namespaces))
	count := make(map[string]int, 2*len(namespaces))

	for {
		done := true
		for id := range count {
			if count, found := count[id]; !found || count < (2*n) {
				done = false
			}
		}

		if done && len(count) == len(namespaces) {
			break
		}

		select {
		case err := <-errors:
			t.Fatal(err)
		case suc := <-successes: // subscription created
			subids[namespaces[int(suc.Id.(float64))]] = suc.Result.(string)
		case failure := <-failures:
			t.Errorf("received error: %v", failure.Error)
		case notification := <-notifications:
			if cnt, found := count[notification.Params.Subscription]; found {
				count[notification.Params.Subscription] = cnt + 1
			} else {
				count[notification.Params.Subscription] = 1
			}
		case <-timeout:
			for _, namespace := range namespaces {
				subid, found := subids[namespace]
				if !found {
					t.Errorf("Subscription for '%s' not created", namespace)
					continue
				}
				if count, found := count[subid]; !found || count < n {
					t.Errorf("Didn't receive all notifications (%d<%d) in time for namespace '%s'", count, n, namespace)
				}
			}
			return
		}
	}
}
