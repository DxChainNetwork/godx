// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package params

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestValidatorConfig_JSON(t *testing.T) {
	validators := DefaultValidators
	b, err := json.Marshal(validators)
	if err != nil {
		t.Fatal(err)
	}
	var decodeValidators []ValidatorConfig
	if err := json.Unmarshal(b, &decodeValidators); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(validators, decodeValidators) {
		t.Errorf("cannot recover")
	}
}
