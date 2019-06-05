// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager"
	"math/rand"
	"time"
)

//import (
//	"fmt"
//	"github.com/DxChainNetwork/godx/common"
//	"github.com/DxChainNetwork/godx/storage"
//	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager"
//	"math/rand"
//	"testing"
//	"time"
//)
//
//var persistDir = "./testdata"
//
//func TestStorageClient_SetClientSettings(t *testing.T) {
//	// initialize new storage client
//	sc, err := New(persistDir)
//	if err != nil {
//		t.Fatalf("failed to initialize the storage client: %s", err.Error())
//	}
//
//	for i := 0; i < 100; i++ {
//		// generate random client settings data, and call set client settings
//		settings := randomClientSettingsGenerator()
//
//		if err := sc.SetClientSetting(settings); err != nil {
//			// if the error is expected, continue to the next one
//			if settingValidation(settings) {
//				continue
//			}
//			// else if the error is not expected, then terminate the test and return error
//			t.Fatalf("failed to set the client settings with value %+v: %s", settings, err.Error())
//		}
//
//		fmt.Println("successfully set the setting")
//
//		// validation persist json file
//		if err := sc.loadSettings(); err != nil {
//			t.Fatalf("failed to load the persist setting")
//		}
//		if sc.persist.MaxUploadSpeed != settings.MaxUploadSpeed {
//			t.Fatalf("the upload speed does not match with the one set: expected %+v, got %+v",
//				settings.MaxUploadSpeed, sc.persist.MaxUploadSpeed)
//		}
//		if sc.persist.MaxDownloadSpeed != settings.MaxDownloadSpeed {
//			t.Fatalf("the download speed does not match with the one set: expected %+v, got %+v",
//				settings.MaxDownloadSpeed, sc.persist.MaxUploadSpeed)
//		}
//
//		// contract manager rate limitation validation
//		read, write, packet := sc.contractManager.RetrieveRateLimit()
//		if read != settings.MaxDownloadSpeed || write != settings.MaxUploadSpeed {
//			t.Fatalf("failed to set the rate limits for contract manager: expected read %+v, got %+v. expected write %+v, got %+v",
//				settings.MaxDownloadSpeed, read, settings.MaxUploadSpeed, write)
//		}
//
//		if read == 0 && write == 0 && packet != 0 {
//			t.Fatalf("failed to set the packet size: expected %+v, got %+v",
//				0, packet)
//		}
//
//		if read != 0 || write != 0 && packet != DefaultPacketSize {
//			t.Fatalf("failed to set the packet size: expected %+v, got %+v",
//				DefaultPacketSize, packet)
//		}
//
//		// validate the rent payment
//
//	}
//}

/*
_____  _____  _______      __  _______ ______        ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__         | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|        |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____       | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|      |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func settingValidation(settings storage.ClientSetting) (expectedErr bool) {
	if settings.MaxUploadSpeed < 0 || settings.MaxDownloadSpeed < 0 {
		return true
	}

	if err := contractmanager.RentPaymentValidation(settings.RentPayment); err != nil {
		return true
	}

	return false
}

func randomClientSettingsGenerator() (settings storage.ClientSetting) {
	settings = storage.ClientSetting{
		RentPayment:       randRentPaymentGenerator(),
		EnableIPViolation: true,
		MaxUploadSpeed:    randInt64(),
		MaxDownloadSpeed:  randInt64(),
	}

	return
}

func randRentPaymentGenerator() (rentPayment storage.RentPayment) {
	rentPayment = storage.RentPayment{
		Fund:               common.RandomBigInt(),
		StorageHosts:       randUint64(),
		Period:             randUint64(),
		RenewWindow:        randUint64(),
		ExpectedStorage:    randUint64(),
		ExpectedUpload:     randUint64(),
		ExpectedDownload:   randUint64(),
		ExpectedRedundancy: randFloat64(),
	}

	return
}

func randUint64() (randUint uint64) {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint64()
}

func randFloat64() (randFloat float64) {
	rand.Seed(time.Now().UnixNano())
	return rand.Float64()
}

func randInt64() (randBool int64) {
	rand.Seed(time.Now().UnixNano())
	return int64(rand.Int())
}
