package twofishgcm

import (
	"bytes"
	"errors"
	"github.com/DxChainNetwork/go-dxc/common"
	"reflect"
	"testing"
)

func TestNewGCMCipher(t *testing.T) {
	tests := []struct {
		seed      []byte
		expectErr error
	}{
		{
			seed:      common.FromHex("123456789012456789012345678901234567890123456789012345678901234"),
			expectErr: nil,
		},
		{
			seed:      common.FromHex("12345678901245678901234567890123456789012345678901234567890"),
			expectErr: errors.New("GCMCipherKey has unexpected length. Expect 32, Got 30"),
		},
		{
			seed:      common.FromHex("123456789012456789012345678901234567890123456789012345678901234567"),
			expectErr: errors.New("GCMCipherKey has unexpected length. Expect 32, Got 33"),
		},
	}
	for i, test := range tests {
		gck, err := NewGCMCipherKey(test.seed)
		if test.expectErr != nil && err == nil {
			t.Errorf("Test %d: expect error [%v], got error [%v]", i, test.expectErr, err)
		}
		if gck != nil {
			if reflect.TypeOf(gck) != reflect.TypeOf(&GCMCipherKey{}) {
				t.Errorf("Test %d: expect type %T, got type %T", i, gck, &GCMCipherKey{})
			}
			if !bytes.Equal(gck[:], test.seed) {
				t.Errorf("Test %d: expect value %v, got %v", i, gck[:], test.seed)
			}
		}
	}
}

func TestGCMCipher(t *testing.T) {
	tests := []struct {
		seed      []byte
		plainText []byte
	}{
		{
			seed:      common.FromHex("123456789012456789012345678901234567890123456789012345678901234"),
			plainText: []byte("I "),
		},
		{
			seed:      common.FromHex("123456789012456789012345678901234567890123456789012345678901234"),
			plainText: []byte("I am jacky. I am genius"),
		},
	}
	for i, test := range tests {
		gck, err := NewGCMCipherKey(test.seed)
		if err != nil {
			t.Fatalf("Test %d: cannot initialize: %v", i, err)
		}
		ct, err := gck.Encrypt(test.plainText)
		if err != nil {
			t.Fatalf("Test %d: cannot encrypt: %v", i, err)
		}
		recovered, err := gck.Decrypt(ct)
		if err != nil {
			t.Fatalf("Test %d: cannot decrypt: %v", i, err)
		}
		if !bytes.Equal(recovered, test.plainText) {
			t.Errorf("Test %d: unexpected recovered text. Expect %v, Got %v", i, string(test.plainText), string(recovered))
		}
		recoveredInPlace, err := gck.DecryptInPlace(ct)
		if err != nil {
			t.Fatalf("Test %d: cannot decrypt in place: %v", i, err)
		}
		if !bytes.Equal(recoveredInPlace, test.plainText) {
			t.Errorf("Test %d: unexpected in place recovered text. Expect %v, Got %v", i, string(test.plainText), string(recoveredInPlace))
		}
	}
}
