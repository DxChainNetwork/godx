package twofishgcm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const (
	// GCMCipherKeyLength is the key length for GCMCipherKey
	GCMCipherKeyLength = 32
)

// GCMCipherKey is the implementation of two fish-GCM algorithm, implementing crypto.CipherKey interface
type GCMCipherKey [GCMCipherKeyLength]byte

// CodeName return the GCMCipherCode specifying the key type
func (gck *GCMCipherKey) CodeName() string {
	return "TwoFish_GCM"
}

// Overhead returns the additional overhead used for nonce
func (gck *GCMCipherKey) Overhead() uint8 {
	return 28
}

// Key returns the encryption/decryption key for the GCMCipherKey
func (gck *GCMCipherKey) Key() []byte {
	key := make([]byte, GCMCipherKeyLength)
	copy(key, gck[:])
	return key
}

// Encrypt encrypt the input plainText using AES-GCM algorithm
func (gck *GCMCipherKey) Encrypt(plainText []byte) ([]byte, error) {
	gcm, err := gck.newGCM()
	if err != nil {
		return nil, err
	}
	// randomize the nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// seal the plainText using nonce
	cipherText := gcm.Seal(nonce, nonce, plainText, nil)
	return cipherText, nil
}

// Decrypt decrypt the input cipherText using AES-GCM algorithm
func (gck *GCMCipherKey) Decrypt(cipherText []byte) ([]byte, error) {
	gcm, err := gck.newGCM()
	if err != nil {
		return nil, nil
	}
	// First part of cipherText is nonce, and rest is cipherText
	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, fmt.Errorf("decrypt error: cipherText has length %v smaller than nonce %v", len(cipherText),
			nonceSize)
	}
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	// Decrypt
	return gcm.Open(nil, nonce, cipherText, nil)
}

// DecryptInPlace make use of the input string and decrypt the cipherText in place
func (gck *GCMCipherKey) DecryptInPlace(cipherText []byte) ([]byte, error) {
	gcm, err := gck.newGCM()
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(cipherText) < gcm.NonceSize() {
		return nil, fmt.Errorf("decrypt error: cipherText has length %v smaller than nonce %v", len(cipherText),
			nonceSize)
	}
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	// Decrypt
	return gcm.Open(cipherText[:0], nonce, cipherText, nil)
}

// newGCM is the internal method to create an AEAD using aes.NewCipher and cipher.NewGCM.
// The method is used in encryption and decryption
func (gck *GCMCipherKey) newGCM() (cipher.AEAD, error) {
	c, err := aes.NewCipher(gck[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(c)
}

// NewGCMCipherKey returns a new GCMCipherKey using the input seed.
// The input key must be of exact size of GCMCipherKeyLength, which is 32
func NewGCMCipherKey(seed []byte) (*GCMCipherKey, error) {
	if len(seed) != GCMCipherKeyLength {
		err := fmt.Errorf("GCMCipherKey has unexpected length. Expect %v, Got %v", GCMCipherKeyLength, len(seed))
		return nil, err
	}

	gck := &GCMCipherKey{}
	copy(gck[:], seed)
	return gck, nil
}

// GenerateGCMCipherKey will generate a new GCMCipherKey with random seed
func GenerateGCMCipherKey() (*GCMCipherKey, error) {
	seed := make([]byte, GCMCipherKeyLength)
	_, err := rand.Read(seed)
	if err != nil {
		return nil, fmt.Errorf("cannot generate random seed")
	}
	return NewGCMCipherKey(seed)
}
