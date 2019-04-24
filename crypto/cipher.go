package crypto

import (
	"errors"
	"github.com/DxChainNetwork/godx/crypto/twofishgcm"
)

const (
	CipherCodeNotSupport uint8 = iota // Code not supported
	PlainCipherCode                   // type code for plainCipherKey
	GCMCipherCode                     // type code for gcmCipherKey
)

var (
	ErrInvalidCipherType = errors.New("provided CipherType not supported")
)

// CipherKey is the interface for cipher key, which is implemented by plainCipherKey, and gcmCipherKey
type CipherKey interface {
	// Code return the code specified of the CipherKey type
	CodeName() string

	// Overhead returns the overhead for decrypted text
	Overhead() uint8

	// CipherKey return the key for the specified CipherKey type
	Key() []byte

	// Encrypt will encrypt the input byte slice to cipher text
	Encrypt([]byte) ([]byte, error)

	// Decrypt will decrypt the input cipher text to plain text
	Decrypt([]byte) ([]byte, error)

	// DecryptInPlace will reuse the input memory and decrypt into the input byte slice.
	// Note that the cipher text has greater length than plainText, so the plainText start
	// at index of overhead
	DecryptInPlace([]byte) ([]byte, error)
}

// plainCipherKey implements CipherKey interface. Used only for tests and in scenario that no encryption is needed.
type plainCipherKey struct{}

// NewPlainCipherKey return a new plainCipherKey
func NewPlainCipherKey() (*plainCipherKey, error) { return &plainCipherKey{}, nil }

func (pc *plainCipherKey) CodeName() string { return "PlainText" } // Plaintext has code PlainCipherCode
func (pc *plainCipherKey) Overhead() uint8  { return 0 }           // Plaintext has no overhead
func (pc *plainCipherKey) Key() []byte      { return []byte{} }

// Encrypt and Decript for plainCipherKey is simply return the input byte slice
func (pc *plainCipherKey) Encrypt(plaintext []byte) ([]byte, error)         { return plaintext[:], nil }
func (pc *plainCipherKey) Decrypt(cipherText []byte) ([]byte, error)        { return cipherText[:], nil }
func (pc *plainCipherKey) DecryptInPlace(cipherText []byte) ([]byte, error) { return cipherText[:], nil }

// NewCipherKey will create a CipherKey using the key type specified by cipherCode, value with the input key
func NewCipherKey(cipherCode uint8, key []byte) (CipherKey, error) {
	switch cipherCode {
	case PlainCipherCode:
		return NewPlainCipherKey()
	case GCMCipherCode:
		return twofishgcm.NewGCMCipherKey(key)
	default:
		return nil, ErrInvalidCipherType
	}
}

// GenerateCipherKey generate a random seed and new a key according to the CipherKey type specified by cipherCode
func GenerateCipherKey(cipherCode uint8) (CipherKey, error) {
	switch cipherCode {
	case PlainCipherCode:
		return &plainCipherKey{}, nil
	case GCMCipherCode:
		return twofishgcm.GenerateGCMCipherKey()
	default:
		return nil, ErrInvalidCipherType
	}
}

// Overhead return the size of the overhead for a cipher type specified by cipherCode
func Overhead(cipherCode uint8) uint8 {
	switch cipherCode {
	case PlainCipherCode:
		return (&plainCipherKey{}).Overhead()
	case GCMCipherCode:
		return (&(twofishgcm.GCMCipherKey{})).Overhead()
	default:
		return 0
	}
}

// CodeByName returns the cipher code associated with the cipher name
func CodeByName(cipherName string) uint8 {
	switch cipherName {
	case (&plainCipherKey{}).CodeName():
		return PlainCipherCode
	case (&(twofishgcm.GCMCipherKey{})).CodeName():
		return GCMCipherCode
	default:
		return CipherCodeNotSupport
	}
}
