// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import "testing"

func TestInteractionName(t *testing.T) {
	tests := []struct {
		it   InteractionType
		name string
	}{
		{InteractionGetConfig, "host config scan"},
		{InteractionCreateContract, "create contract"},
		{InteractionRenewContract, "renew contract"},
		{InteractionUpload, "upload"},
		{InteractionDownload, "download"},
	}
	for index, test := range tests {
		name := InteractionTypeToName(test.it)
		if name != test.name {
			t.Errorf("test %d interaction name not expected. Got %v, Expect %v", index, name, test.name)
		}
		it := InteractionNameToType(test.name)
		if it != test.it {
			t.Errorf("test %d interaction type not expected. Got %v, Expect %v", index, it, test.it)
		}
	}
}

func TestInteractionNameInvalid(t *testing.T) {
	invalidName := "ssss"
	if InteractionNameToType(invalidName) != InteractionInvalid {
		t.Errorf("invalid name does not yield expected result")
	}
	if InteractionTypeToName(InteractionInvalid) != "" {
		t.Errorf("invalid type does not yield empty name")
	}
	if InteractionTypeToName(InteractionType(100)) != "" {
		t.Errorf("invalid type does not yield empty name")
	}
}
