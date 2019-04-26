package crypto

import (
	"bytes"
	"github.com/DxChainNetwork/godx/crypto/twofishgcm"
	"reflect"
	"testing"
)

func TestNewCipherKey(t *testing.T) {
	tests := []struct {
		inputCode uint8
		inputKey  []byte
		expectKey CipherKey
		expectErr error
	}{
		{
			inputCode: PlainCipherCode, inputKey: []byte{},
			expectKey: &plainCipherKey{}, expectErr: nil,
		},
		{
			inputCode: GCMCipherCode, inputKey: bytes.Repeat([]byte{1}, int(twofishgcm.GCMCipherKeyLength)),
			expectKey: &twofishgcm.GCMCipherKey{}, expectErr: nil,
		},
		{
			inputCode: 255, inputKey: []byte{},
			expectKey: nil, expectErr: ErrInvalidCipherCode,
		},
	}
	for i, test := range tests {
		key, err := NewCipherKey(test.inputCode, test.inputKey)
		if err != test.expectErr {
			t.Errorf("Test %d: expect error %v, Got %v", i, test.expectErr, err)
		}
		if err != nil && test.expectErr != nil {
			if reflect.TypeOf(key) != reflect.TypeOf(test.expectKey) {
				t.Errorf("Test %d: expect type %t, Got %t", i, test.expectKey, key)
			}
		}
	}
}

func TestGenerateCipherKey(t *testing.T) {
	tests := []struct {
		inputCode uint8
		expectKey CipherKey
		expectErr error
	}{
		{
			inputCode: PlainCipherCode,
			expectKey: &plainCipherKey{}, expectErr: nil,
		},
		{
			inputCode: GCMCipherCode,
			expectKey: &twofishgcm.GCMCipherKey{}, expectErr: nil,
		},
		{
			inputCode: 255,
			expectKey: nil, expectErr: ErrInvalidCipherCode,
		},
	}
	for i, test := range tests {
		key, err := GenerateCipherKey(test.inputCode)
		if err != test.expectErr {
			t.Errorf("Test %d: expect error %v, Got %v", i, test.expectErr, err)
		}
		if err != nil && test.expectErr != nil {
			if reflect.TypeOf(key) != reflect.TypeOf(test.expectKey) {
				t.Errorf("Test %d: expect type %t, Got %t", i, test.expectKey, key)
			}
		}
	}
}

func TestCodeByName(t *testing.T) {
	tests := []struct {
		cipherName string
		cipherCode uint8
	}{
		{
			cipherName: "aaa",
			cipherCode: CipherCodeNotSupport,
		},
		{
			cipherName: "PlainText",
			cipherCode: PlainCipherCode,
		},
		{
			cipherName: "TwoFish_GCM",
			cipherCode: GCMCipherCode,
		},
	}
	for i, test := range tests {
		code := CodeByName(test.cipherName)
		if code != test.cipherCode {
			t.Errorf("Test %d. Expect %v, Got %v", i, test.cipherCode, code)
		}
	}
}
