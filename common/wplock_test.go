package common

import (
	"sync"
	"testing"
	"time"
)

// TestWPLock test WPLock
func TestWPLock(t *testing.T) {
	var wp WPLock
	var wg sync.WaitGroup
	for i := 0; i != 100; i++ {
		wg.Add(1)
		go func(i int) {
			<-time.After(time.Duration(20*i) * time.Millisecond)
			wp.RLock()
			<-time.After(time.Duration(40) * time.Millisecond)
			wp.RUnlock()
			wg.Done()
		}(i)
	}
	writeChan := make(chan string)
	go func() {
		<-time.After(time.Duration(200 * time.Millisecond))
		writeChan <- "Test"
	}()
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		t.Fatalf("Write blocked by read")
	case <-time.After(3000 * time.Millisecond):
		t.Fatal("time out")
	case s := <-writeChan:
		if s != "Test" {
			t.Fatalf("Text unexpected. Got %v, Want %v", s, "test")
		}
	}
}
