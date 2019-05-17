package storagemanager

import (
	"math/rand"
	"testing"
	"time"
)

func TestStorageManager(t *testing.T) {
	sm, err := newStorageManager(TestPath)
	if sm == nil || err != nil {
		t.Errorf("fail to create storage manager")
	}
	rand.Seed(time.Now().UTC().UnixNano())
	duration := rand.Int31n(4000)
	time.Sleep(time.Duration(duration) * time.Millisecond)
	if err := sm.Close(); err != nil {
		t.Errorf(err.Error())
	}
}
