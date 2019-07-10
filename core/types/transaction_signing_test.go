// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"math/big"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
)

// TestMakeSigner test MakeSigner function.
// The function should make different type of signer based on the chainConfig
func TestMakeSigner(t *testing.T) {
	chainConfig := params.MainnetChainConfig
	tests := []struct {
		input *big.Int
		want  Signer
	}{
		{big.NewInt(0), EIP155Signer{}},
		{big.NewInt(1000000), EIP155Signer{}},
		{big.NewInt(2000000), EIP155Signer{}},
		{big.NewInt(3000000), EIP155Signer{}},
	}
	for _, test := range tests {
		s := MakeSigner(chainConfig, test.input)
		if reflect.TypeOf(s) != reflect.TypeOf(test.want) {
			t.Errorf("Signer returned by Make hash have unexpected type\nInput %d\nGot %v\nWant %v",
				test.input, reflect.TypeOf(s), reflect.TypeOf(test.want))
		}
	}
}

// TestSignTxSender test SignTx and Sender.
// SignTx sign a transaction
// Sender recover the public address from signature.
// 1. Sign a transaction using SignTx
// 2. Given signed transaction, recover the public address using Sender
// 3. Recovered public address should be exactly the public address signing the tx
func TestSignTxSender(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	tests := []Signer{
		NewEIP155Signer(big.NewInt(18)),
		HomesteadSigner{},
		FrontierSigner{},
	}
	for _, signer := range tests {
		tx, err := SignTx(NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil), signer, key)
		if err != nil {
			t.Errorf("Cannot sign transaction: %s", err.Error())
		}
		from, err := Sender(signer, tx)
		if err != nil {
			t.Errorf("Cannot get Sender: %s", err.Error())
		}
		if from != addr {
			t.Errorf("expected from and address to be equal. Got %x want %x", from, addr)
		}
	}
}

// TestNewEIP155Signer test NewEIP155Signer for normal execution.
// Values assigned in EIP155Signer are tested whether they have expected values.
func TestNewEIP155Signer(t *testing.T) {
	tests := []*big.Int{
		nil,
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(12381927398172399),
	}
	for _, test := range tests {
		s := NewEIP155Signer(test)
		if test == nil {
			if s.chainId.Cmp(big.NewInt(0)) != 0 || s.chainIdMul.Cmp(big.NewInt(0)) != 0 {
				t.Errorf("NewEIP155Signer with nil input give unexpected value:\nchainId: Got %d Want %d\nchainIdMul: Got %d Want %d",
					s.chainId, big.NewInt(0), s.chainIdMul, big.NewInt(0))
			}
		} else {
			if s.chainId.Cmp(test) != 0 || s.chainIdMul.Cmp(new(big.Int).Mul(test, big.NewInt(2))) != 0 {
				t.Errorf("NewEIP155Signer with input %d give unexpected value:\nchainId: Got %d Want %d\nchainIdMul: Got %d Want %d",
					test, s.chainId, test, s.chainIdMul, new(big.Int).Mul(test, big.NewInt(2)))
			}
		}
	}
}

// TestEIP155Signer_Equal test EIP155Signer.Equal
func TestEIP155Signer_Equal(t *testing.T) {
	s := NewEIP155Signer(big.NewInt(100))
	tests := []struct {
		input  Signer
		output bool
	}{
		{HomesteadSigner{}, false},
		{FrontierSigner{}, false},
		{NewEIP155Signer(nil), false},
		{NewEIP155Signer(big.NewInt(10000)), false},
		{NewEIP155Signer(big.NewInt(100)), true},
	}
	for _, test := range tests {
		if s.Equal(test.input) != test.output {
			t.Errorf("Inputing a signer %v return unexpected value. Got %v, Want %v", test.input,
				s.Equal(test.input), test.output)
		}
	}
}

// TestEIP155Signer_Sender test EIP155Signer.Sender normal execution
// Input transaction is given with rlp string.
func TestEIP155Signer_Sender(t *testing.T) {
	tests := []struct {
		name     string
		signer   Signer
		txRlpStr string
		wantAddr string
	}{
		{"protected", NewEIP155Signer(big.NewInt(1)), "f864808504a817c80082520894353535353535353535353535353535353535353580801ba0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d", "0x970557934cabd73bfec0266fea2a3d01fd736fac"},
		{"not protected", NewEIP155Signer(big.NewInt(1)), "f864808504a817c800825208943535353535353535353535353535353535353535808025a0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d", "0xf0f6f18bca1b28cd68e4357452947e021241e9ce"},
	}
	for _, test := range tests {
		var tx *Transaction
		if err := rlp.DecodeBytes(common.Hex2Bytes(test.txRlpStr), &tx); err != nil {
			t.Errorf("Cannot decode byte for test %s", test.name)
		}
		res, err := test.signer.Sender(tx)
		if err != nil {
			t.Errorf("Sender for test %s return unexpected error: %s", test.name, err.Error())
		}
		if res != common.HexToAddress(test.wantAddr) {
			t.Errorf("Sender for test %s return unexpected value.\nGot %x\nWant%s", test.name, res, test.wantAddr)
		}
	}
}

// TestEIP155Signer_SenderError test the error schema for EIP155Signer.Sender
// The error include invalid chainid and invalid v value.
func TestEIP155Signer_SenderError(t *testing.T) {
	tests := []struct {
		name      string
		signer    Signer
		txRlpStr  string
		wantError error
	}{
		{"Invalid ChainId", NewEIP155Signer(big.NewInt(1)), "f864808504a817c800825208943535353535353535353535353535353535353535808029a0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d",
			ErrInvalidChainId},
		{"Invalid v value", NewEIP155Signer(big.NewInt(1)), "f844808504a817c80082520894353535353535353535353535353535353535353580802580a0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d",
			ErrInvalidSig},
	}
	for _, test := range tests {
		var tx *Transaction
		if err := rlp.DecodeBytes(common.Hex2Bytes(test.txRlpStr), &tx); err != nil {
			t.Errorf("Cannot decode byte for test %s", test.name)
		}
		if _, err := test.signer.Sender(tx); err == nil {
			t.Errorf("Sender for %s does not return error. Want %s", test.name, test.wantError.Error())
		} else if err != test.wantError {
			t.Errorf("Sender for %s return unexpected error. \nGot %s\nWant %s", test.name, err.Error(), test.wantError.Error())
		}
	}
}

// TestEIP155Signer_SignatureValues test EIP155Signer.SignatureValues
func TestEIP155Signer_SignatureValues(t *testing.T) {
	tx := NewTransaction(0, common.HexToAddress("0x123"), new(big.Int), 0, new(big.Int), nil)
	tests := []struct {
		input               []byte
		signer              EIP155Signer
		wantR, wantS, wantV []byte
	}{
		{
			common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8ea5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ad"),
			NewEIP155Signer(big.NewInt(0)),
			common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8e"),
			common.FromHex("a5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670"),
			common.FromHex("c8"),
		},
		{
			common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8ea5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ad"),
			NewEIP155Signer(big.NewInt(1)),
			common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8e"),
			common.FromHex("a5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670"),
			common.FromHex("d2"),
		},
	}
	for _, test := range tests {
		r, s, v, err := test.signer.SignatureValues(tx, test.input)
		if err != nil {
			t.Errorf("Unexpected Error:\nInput %x\nError %s", test.input, err.Error())
		}
		if r.Cmp(new(big.Int).SetBytes(test.wantR)) != 0 || s.Cmp(new(big.Int).SetBytes(test.wantS)) != 0 ||
			v.Cmp(new(big.Int).SetBytes(test.wantV)) != 0 {
			t.Errorf("Unexpected value:\nInput %x\nGot\tR: %x S: %x V: %x\nWant\tR: %x S: %x V: %x",
				test.input, r, s, v, test.wantR, test.wantS, test.wantV)
		}
	}
}

// TestHomesteadSigner_Equal test HomesteadSigner.Equal
func TestHomesteadSigner_Equal(t *testing.T) {
	hs := HomesteadSigner{}
	tests := []struct {
		input  Signer
		output bool
	}{
		{HomesteadSigner{}, true},
		{NewEIP155Signer(big.NewInt(1)), false},
		{FrontierSigner{}, false},
	}
	for _, test := range tests {
		if res := hs.Equal(test.input); res != test.output {
			t.Errorf("HomesteadSigner equal return unexpected.\nInput %+v\nGot %v\nWant %v",
				test.input, res, test.output)
		}
	}
}

// TestHomesteadSigner_Sender test HomesteadSigner.Sender
// input transaction is given with rlp string
func TestHomesteadSigner_Sender(t *testing.T) {
	tests := []struct {
		txRlpStr string
		wantAddr string
	}{
		{"f864808504a817c80082520894353535353535353535353535353535353535353580801ba0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d", "0x970557934cabd73bfec0266fea2a3d01fd736fac"},
	}
	for _, test := range tests {
		var tx *Transaction
		if err := rlp.DecodeBytes(common.Hex2Bytes(test.txRlpStr), &tx); err != nil {
			t.Errorf("Cannot decode byte for Transaction")
		}
		res, err := HomesteadSigner{}.Sender(tx)
		if err != nil {
			t.Errorf("HomesteadSigner.Sender return unexpected error: %s", err.Error())
		}
		if res != common.HexToAddress(test.wantAddr) {
			t.Errorf("HomesteadSigner.Sender return unexpected address.\nGot %x\nWant %x", res, test.wantAddr)
		}
	}
}

// TestFrontierSigner_Equal test FrontierSigner.Equal
func TestFrontierSigner_Equal(t *testing.T) {
	hs := FrontierSigner{}
	tests := []struct {
		input  Signer
		output bool
	}{
		{HomesteadSigner{}, false},
		{NewEIP155Signer(big.NewInt(1)), false},
		{FrontierSigner{}, true},
	}
	for _, test := range tests {
		if res := hs.Equal(test.input); res != test.output {
			t.Errorf("FrontierSigner.Equal return unexpected.\nInput %+v\nGot %v\nWant %v",
				test.input, res, test.output)
		}
	}
}

// TestFrontierSigner_SignatureValues test FrontierSigner.SignatureValues
func TestFrontierSigner_SignatureValues(t *testing.T) {
	tx := NewTransaction(0, common.HexToAddress("0x123"), new(big.Int), 0, new(big.Int), nil)
	fs := FrontierSigner{}
	tests := []struct {
		input               []byte
		wantR, wantS, wantV []byte
	}{
		{
			common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8ea5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ad"),
			common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8e"),
			common.FromHex("a5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670"),
			common.FromHex("c8"),
		},
	}
	for _, test := range tests {
		r, s, v, err := fs.SignatureValues(tx, test.input)
		if err != nil {
			t.Errorf("Unexpected Error:\nInput %x\nError %s", test.input, err.Error())
		}
		if r.Cmp(new(big.Int).SetBytes(test.wantR)) != 0 || s.Cmp(new(big.Int).SetBytes(test.wantS)) != 0 ||
			v.Cmp(new(big.Int).SetBytes(test.wantV)) != 0 {
			t.Errorf("Unexpected value:\nInput %x\nGot\tR: %x S: %x V: %x\nWant\tR: %x S: %x V: %x",
				test.input, r, s, v, test.wantR, test.wantS, test.wantV)
		}
	}
}

// TestFrontierSigner_Sender test FrontierSigner.Sender
// The input transaction is given in rlp string.
func TestFrontierSigner_Sender(t *testing.T) {
	tests := []struct {
		txRlpStr string
		wantAddr string
	}{
		{"f864808504a817c80082520894353535353535353535353535353535353535353580801ba0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d", "0x970557934cabd73bfec0266fea2a3d01fd736fac"},
	}
	for _, test := range tests {
		var tx *Transaction
		if err := rlp.DecodeBytes(common.Hex2Bytes(test.txRlpStr), &tx); err != nil {
			t.Errorf("Cannot decode byte for Transaction")
		}
		res, err := FrontierSigner{}.Sender(tx)
		if err != nil {
			t.Errorf("FrontierSigner.Sender return unexpected error: %s", err.Error())
		}
		if res != common.HexToAddress(test.wantAddr) {
			t.Errorf("FrontierSigner.Sender return unexpected address.\nGot %x\nWant %x", res, test.wantAddr)
		}
	}
}

// TestFrontierSigner_SignatureValuesPanic test whether the FrontierSigner.SignatureValues
// will panic if len(sig) != 65
func TestFrontierSigner_SignatureValuesPanic(t *testing.T) {
	tests := [][]byte{
		{},
		bytes.Repeat([]byte{0x01}, 64),
		bytes.Repeat([]byte{0x01}, 66),
	}
	fs := FrontierSigner{}
	for _, test := range tests {
		func(t *testing.T, data []byte) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("FrontierSigner.SignatureValues for input %x does not panic", data)
				}
			}()
			fs.SignatureValues(NewTransaction(0, common.HexToAddress("0x123"), new(big.Int), 0, new(big.Int), nil),
				data)
		}(t, test)
	}
}

// TestRecoverPlain test the error types returned by recoverPlain.
// The value returned by RecoverPlain is ensured by test TestSignTxSender
func TestRecoverPlain(t *testing.T) {
	secp256k1NPlus1, _ := new(big.Int).SetString("7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a1", 16)
	sigHash := "f864808504a817c800825208943535353535353535353535353535353535353535808025a0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d"
	tests := []struct {
		name        string
		R, S, Vb    *big.Int
		homestead   bool
		expectError error
	}{
		{"homestead ok", big.NewInt(1), big.NewInt(1), big.NewInt(27), true, nil},
		{"non-homestead ok", big.NewInt(1), big.NewInt(1), big.NewInt(27), false, nil},
		{"Vb exceed 255", big.NewInt(1), big.NewInt(1), big.NewInt(256), false, ErrInvalidSig},
		{"non-homestead large S ok", big.NewInt(1), secp256k1NPlus1, big.NewInt(27), false, nil},
		{"homestead large S error", big.NewInt(1), secp256k1NPlus1, big.NewInt(27), true, ErrInvalidSig},
		{"negative R value", big.NewInt(-1), big.NewInt(1), big.NewInt(27), false, ErrInvalidSig},
	}
	for _, test := range tests {
		_, err := recoverPlain(common.HexToHash(sigHash), test.R, test.S, test.Vb, test.homestead)
		if err != test.expectError {
			t.Errorf("RecoverPlain %s Got error %v Expect %v", test.name, err, test.expectError)
		}
	}
}

// TestDeriveChainId test function deriveChainId
func TestDeriveChainId(t *testing.T) {
	tests := []struct {
		input *big.Int
		want  *big.Int
	}{
		{big.NewInt(27), big.NewInt(0)},
		{big.NewInt(28), big.NewInt(0)},
		{big.NewInt(39), big.NewInt(2)},
		{new(big.Int).SetBytes(common.FromHex("de5407e78fb2863ca")), new(big.Int).SetBytes(common.FromHex("6f2a03f3c7d9431d3"))},
	}
	for _, test := range tests {
		if res := deriveChainId(test.input); res.Cmp(test.want) != 0 {
			t.Errorf("Input: %d\nGot: %d\nWant: %d", test.input, res, test.want)
		}
	}
}

// _______ _________          _______  _______   _________ _______  _______ _________ _______
// (  ____ \\__   __/|\     /|(  ____ \(  ____ )  \__   __/(  ____ \(  ____ \\__   __/(  ____ \
// | (    \/   ) (   | )   ( || (    \/| (    )|     ) (   | (    \/| (    \/   ) (   | (    \/
// | (__       | |   | (___) || (__    | (____)|     | |   | (__    | (_____    | |   | (_____
// |  __)      | |   |  ___  ||  __)   |     __)     | |   |  __)   (_____  )   | |   (_____  )
// | (         | |   | (   ) || (      | (\ (        | |   | (            ) |   | |         ) |
// | (____/\   | |   | )   ( || (____/\| ) \ \__     | |   | (____/\/\____) |   | |   /\____) |
// (_______/   )_(   |/     \|(_______/|/   \__/     )_(   (_______/\_______)   )_(   \_______)
//
func TestEIP155Signing(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	signer := NewEIP155Signer(big.NewInt(18))
	tx, err := SignTx(NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil), signer, key)
	if err != nil {
		t.Fatal(err)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != addr {
		t.Errorf("exected from and address to be equal. Got %x want %x", from, addr)
	}
}

func TestEIP155ChainId(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	signer := NewEIP155Signer(big.NewInt(18))
	tx, err := SignTx(NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil), signer, key)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.Protected() {
		t.Fatal("expected tx to be protected")
	}

	if tx.ChainId().Cmp(signer.chainId) != 0 {
		t.Error("expected chainId to be", signer.chainId, "got", tx.ChainId())
	}

	tx = NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil)
	tx, err = SignTx(tx, HomesteadSigner{}, key)
	if err != nil {
		t.Fatal(err)
	}

	if tx.Protected() {
		t.Error("didn't expect tx to be protected")
	}

	if tx.ChainId().Sign() != 0 {
		t.Error("expected chain id to be 0 got", tx.ChainId())
	}
}

// TestEIP155SigningVitalik test Sender(signer, tx)
func TestEIP155SigningVitalik(t *testing.T) {
	// Test vectors come from http://vitalik.ca/files/eip155_testvec.txt
	for i, test := range []struct {
		txRlp, addr string
	}{
		{"f864808504a817c800825208943535353535353535353535353535353535353535808025a0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d", "0xf0f6f18bca1b28cd68e4357452947e021241e9ce"},
		{"f864018504a817c80182a410943535353535353535353535353535353535353535018025a0489efdaa54c0f20c7adf612882df0950f5a951637e0307cdcb4c672f298b8bcaa0489efdaa54c0f20c7adf612882df0950f5a951637e0307cdcb4c672f298b8bc6", "0x23ef145a395ea3fa3deb533b8a9e1b4c6c25d112"},
		{"f864028504a817c80282f618943535353535353535353535353535353535353535088025a02d7c5bef027816a800da1736444fb58a807ef4c9603b7848673f7e3a68eb14a5a02d7c5bef027816a800da1736444fb58a807ef4c9603b7848673f7e3a68eb14a5", "0x2e485e0c23b4c3c542628a5f672eeab0ad4888be"},
		{"f865038504a817c803830148209435353535353535353535353535353535353535351b8025a02a80e1ef1d7842f27f2e6be0972bb708b9a135c38860dbe73c27c3486c34f4e0a02a80e1ef1d7842f27f2e6be0972bb708b9a135c38860dbe73c27c3486c34f4de", "0x82a88539669a3fd524d669e858935de5e5410cf0"},
		{"f865048504a817c80483019a28943535353535353535353535353535353535353535408025a013600b294191fc92924bb3ce4b969c1e7e2bab8f4c93c3fc6d0a51733df3c063a013600b294191fc92924bb3ce4b969c1e7e2bab8f4c93c3fc6d0a51733df3c060", "0xf9358f2538fd5ccfeb848b64a96b743fcc930554"},
		{"f865058504a817c8058301ec309435353535353535353535353535353535353535357d8025a04eebf77a833b30520287ddd9478ff51abbdffa30aa90a8d655dba0e8a79ce0c1a04eebf77a833b30520287ddd9478ff51abbdffa30aa90a8d655dba0e8a79ce0c1", "0xa8f7aba377317440bc5b26198a363ad22af1f3a4"},
		{"f866068504a817c80683023e3894353535353535353535353535353535353535353581d88025a06455bf8ea6e7463a1046a0b52804526e119b4bf5136279614e0b1e8e296a4e2fa06455bf8ea6e7463a1046a0b52804526e119b4bf5136279614e0b1e8e296a4e2d", "0xf1f571dc362a0e5b2696b8e775f8491d3e50de35"},
		{"f867078504a817c807830290409435353535353535353535353535353535353535358201578025a052f1a9b320cab38e5da8a8f97989383aab0a49165fc91c737310e4f7e9821021a052f1a9b320cab38e5da8a8f97989383aab0a49165fc91c737310e4f7e9821021", "0xd37922162ab7cea97c97a87551ed02c9a38b7332"},
		{"f867088504a817c8088302e2489435353535353535353535353535353535353535358202008025a064b1702d9298fee62dfeccc57d322a463ad55ca201256d01f62b45b2e1c21c12a064b1702d9298fee62dfeccc57d322a463ad55ca201256d01f62b45b2e1c21c10", "0x9bddad43f934d313c2b79ca28a432dd2b7281029"},
		{"f867098504a817c809830334509435353535353535353535353535353535353535358202d98025a052f8f61201b2b11a78d6e866abc9c3db2ae8631fa656bfe5cb53668255367afba052f8f61201b2b11a78d6e866abc9c3db2ae8631fa656bfe5cb53668255367afb", "0x3c24d7329e92f84f08556ceb6df1cdb0104ca49f"},
	} {
		signer := NewEIP155Signer(big.NewInt(1))

		var tx *Transaction
		err := rlp.DecodeBytes(common.Hex2Bytes(test.txRlp), &tx)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}

		from, err := Sender(signer, tx)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}

		addr := common.HexToAddress(test.addr)
		if from != addr {
			t.Errorf("%d: expected %x got %x", i, addr, from)
		}

	}
}

func TestChainId(t *testing.T) {
	key, _ := defaultTestKey()

	tx := NewTransaction(0, common.Address{}, new(big.Int), 0, new(big.Int), nil)

	var err error
	tx, err = SignTx(tx, NewEIP155Signer(big.NewInt(1)), key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Sender(NewEIP155Signer(big.NewInt(2)), tx)
	if err != ErrInvalidChainId {
		t.Error("expected error:", ErrInvalidChainId)
	}

	_, err = Sender(NewEIP155Signer(big.NewInt(1)), tx)
	if err != nil {
		t.Error("expected no error")
	}
}
