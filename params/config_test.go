// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package params

import (
	"encoding/json"
	"testing"

	"github.com/DxChainNetwork/godx/common"
)

func TestValidatorConfig_MarshalJSON(t *testing.T) {
	var sampleValidators = ValidatorConfig{
		Deposit:     common.NewBigIntUint64(100000),
		RewardRatio: 80,
	}
	// marshal
	enc, err := json.Marshal(sampleValidators)
	if err != nil {
		t.Fatalf("error marshalling: %s", err.Error())
	}

	// unmarshal
	var vc ValidatorConfig
	if err := json.Unmarshal(enc, &vc); err != nil {
		t.Fatalf("error unmarshalling: %s", err.Error())
	}

}
