package storagehost

import (
	"math/big"
	"math/rand"
	"os"
	"reflect"
	"testing"
)

func TestHostJSDataPreservation(t *testing.T) {
	// clear the saved data for testing
	if err := os.RemoveAll("./testdata/"); err != nil {
		t.Logf("cannot remove the data when testing")
		return
	}

	// try to do the first data json renew, use the default value
	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// extract the persistence data from host
	persist1 := host.extractPersistence()

	for Iterations := 10; Iterations > 0; Iterations-- {

		// close the host to check if it can save the data as expected
		if err := host.Close(); err != nil {
			t.Errorf(err.Error())
		}

		// renew the host again, check if match the data saving before closed
		host, err = NewStorageHost("./testdata/")
		if err != nil {
			t.Errorf(err.Error())
		}

		persist2 := host.extractPersistence()

		if !reflect.DeepEqual(persist1, persist2) {
			//spew.Dump(persist1)
			//spew.Dump(persist2)
			t.Errorf("two persistance does not equal to each other")
		}

		// change the setting again, use the random value
		host.broadcast = rand.Float32() < 0.5
		host.revisionNumber = uint64(rand.Intn(100000000))
		host.settings.AcceptingContracts = rand.Float32() < 0.5
		host.settings.Deposit = *big.NewInt(int64(rand.Intn(100000000)))

		persist1 = host.extractPersistence()
	}

	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}

	// clear the testing data
	if err := os.RemoveAll("./testdata/"); err != nil {
		t.Logf("cannot remove the data when testing")
		return
	}

}
