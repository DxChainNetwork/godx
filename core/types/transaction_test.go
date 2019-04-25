package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/pkg/errors"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// txJsonData is the data used for testing json.Marshal and json.Unmarshal.
// The data is also used in other tests.
var txJsonData = []struct {
	name            string
	json            string
	tx              *Transaction
	marshalError    error
	unmarshalError  error
	unmarshalError2 error
}{
	{
		name:           "ok",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             NewTransaction(uint64(1), common.BytesToAddress([]byte{1}), common.Big1, uint64(3), common.Big1, []byte("abcdf")),
		marshalError:   nil,
		unmarshalError: nil,
	},
	{
		name:           "unprotected with signature",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x1b","r":"0x2","s":"0x2","hash":"0x83b9eb4f5e0d56a01706bca8faecd493554ebc2f0add6e66694884148cd8a308"}`,
		tx:             NewTransaction(uint64(1), common.BytesToAddress([]byte{1}), common.Big1, uint64(3), common.Big1, []byte("abcdf")).simpleSignTx(big.NewInt(2), big.NewInt(2), big.NewInt(27)),
		marshalError:   nil,
		unmarshalError: nil,
	},
	{
		name:           "protected with signature",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x25","r":"0x2","s":"0x2","hash":"0xde8bcc0bdccfb6236b358228fd4749af68bf3712abfaf475f2acd52a93f64d97"}`,
		tx:             NewTransaction(uint64(1), common.BytesToAddress([]byte{1}), common.Big1, uint64(3), common.Big1, []byte("abcdf")).simpleSignTx(big.NewInt(2), big.NewInt(2), big.NewInt(37)),
		marshalError:   nil,
		unmarshalError: nil,
	},
	{
		name: "corner",
		json: `{"nonce":"0xffffffffffffffff","gasPrice":"0xebcbee2f77de396ca982b9d98472d938f94","gas":"0xffffffffffffffff","to":"0x0000000000000000000089f1089f1089f1089f10","value":"0xebcbee2f77de396ca982b9d98472d938f94","input":"0x09090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909090909","v":"0x0","r":"0xebcbee2f77de396ca982b9d98472d938f94","s":"0xebcbee2f77de396ca982b9d98472d938f94","hash":"0xa552f8981b3008ae271b90aa2abfcb0d0ba866cb739c63945932edff0f9ba437"}`,
		tx: &Transaction{data: txdata{
			uint64(18446744073709551615),
			stringToBigInt("1283798819823798174879817892379812789718932", 10),
			uint64(18446744073709551615),
			hex2Address(strings.Repeat("89f10", 4)),
			stringToBigInt("1283798819823798174879817892379812789718932", 10),
			bytes.Repeat([]byte{0x09}, 100),

			common.Big0,
			stringToBigInt("1283798819823798174879817892379812789718932", 10),
			stringToBigInt("1283798819823798174879817892379812789718932", 10),
			hex2Hash(strings.Repeat("89f1", 8)),
		}},
		marshalError:   nil,
		unmarshalError: nil,
	},
	{
		name:           "Missing data",
		json:           `{"nonce":"0x0","gasPrice":"0x1","gas":"0x0","to":"0x0000000000000000000000000000000000000000","value":"0x0","input":"0x","v":"0x0","r":"0x0","s":"0x0","hash":"0xbf9828b1dd232c0e66a94cb97c19186f8c9c42d9a2d86f811a0bb0cd447c202a"}`,
		tx:             NewTransaction(uint64(0), common.BytesToAddress([]byte{}), common.Big0, uint64(0), common.Big1, nil),
		marshalError:   nil,
		unmarshalError: nil,
	},
	{
		name:           "error signature",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x25","r":"0x0","s":"0x0","hash":"0xde8bcc0bdccfb6236b358228fd4749af68bf3712abfaf475f2acd52a93f64d97"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("invalid transaction v, r, s values"),
	},
	{
		name:           "Empty data",
		json:           `{"nonce":null,"gasPrice":null,"gas":"0x0","to":null,"value":null,"input":"0x","v":null,"r":null,"s":null,"hash":"0xc5b2c658f5fa236c598a6e7fbf7f21413dc42e2a41dd982eb772b30707cba2eb"}`,
		tx:             &Transaction{data: txdata{}},
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'nonce' for txdata"),
	},
	{
		name:            "JSON error",
		json:            `{"nonce":"0x1","gasPrice":"1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:              nil,
		marshalError:    nil,
		unmarshalError:  errors.New("json: cannot unmarshal hex string without 0x prefix into Go value of type *hexutil.Big"),
		unmarshalError2: errors.New("json: cannot unmarshal hex string without 0x prefix into Go struct field txdata.gasPrice of type *hexutil.Big"),
	},
	{
		name:           "JSON error",
		json:           `"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("invalid character ':' after top-level value"),
	},
	{
		name:           "JSON missing nonce",
		json:           `{"gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'nonce' for txdata"),
	},
	{
		name:           "JSON missing gasPrice",
		json:           `{"nonce":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'gasPrice' for txdata"),
	},
	{
		name:           "JSON missing gas",
		json:           `{"nonce":"0x1","gasPrice":"0x1","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'gas' for txdata"),
	},
	{
		name:           "JSON missing value",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","input":"0x6162636466","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'value' for txdata"),
	},
	{
		name:           "JSON missing input",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","v":"0x0","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'input' for txdata"),
	},
	{
		name:           "JSON missing v",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","r":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'v' for txdata"),
	},
	{
		name:           "JSON missing r",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","s":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 'r' for txdata"),
	},
	{
		name:           "JSON missing s",
		json:           `{"nonce":"0x1","gasPrice":"0x1","gas":"0x3","to":"0x0000000000000000000000000000000000000001","value":"0x1","input":"0x6162636466","v":"0x0","r":"0x0","hash":"0xf66ffe9c5358b305d84192938172f561d920e2cd191602f3a59e3d623eed158c"}`,
		tx:             nil,
		marshalError:   nil,
		unmarshalError: errors.New("missing required field 's' for txdata"),
	},
}

// TestTransaction_ChainId test Transaction.ChainId
// shoud return the correct chainId
func TestTransaction_ChainId(t *testing.T) {
	tests := []struct {
		input *big.Int
		want  *big.Int
	}{
		{big.NewInt(27), big.NewInt(0)},
		{big.NewInt(28), big.NewInt(0)},
		{big.NewInt(39), big.NewInt(2)},
		{new(big.Int).SetBytes(common.FromHex("de5407e78fb2863ca")), new(big.Int).SetBytes(common.FromHex("6f2a03f3c7d9431d3"))},
	}
	for i, test := range tests {
		tx := NewTransaction(uint64(i), common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))
		tx.data.V = new(big.Int).Set(test.input)
		if res := tx.ChainId(); res.Cmp(test.want) != 0 {
			t.Errorf("Input: %d\nGot: %d\nWant: %d", test.input, res, test.want)
		}
	}
}

// TestTransaction_Protected test Transaction.Protected
func TestTransaction_Protected(t *testing.T) {
	tests := []struct {
		input *big.Int
		want  bool
	}{
		{big.NewInt(27), false},
		{big.NewInt(28), false},
		{big.NewInt(39), true},
		{new(big.Int).SetBytes(common.FromHex("de5407e78fb2863ca")), true},
	}
	for i, test := range tests {
		tx := NewTransaction(uint64(i), common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))
		tx.data.V = new(big.Int).Set(test.input)
		if res := tx.Protected(); res != test.want {
			t.Errorf("Input: %d\nGot: %v\nWant: %v", test.input, res, test.want)
		}
	}
}

// TestTransaction_DecodeRLP test rlp decode Transaction.
// Transaction.DecodeRLP is not called directly. Since Transaction implements rlp.Decoder,
// rlp.DecodeBytes is used instead.
func TestTransaction_DecodeRLP(t *testing.T) {
	rlpStr := common.FromHex("f86103018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8255441ca098ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4aa08887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a3")
	tx, err := decodeTx(rlpStr)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !reflect.DeepEqual(rightvrsTx.data, tx.data) {
		t.Errorf("Unexpected Transaction value. \nGot %sWant%s", dumper.Sdump(tx), dumper.Sdump(rightvrsTx))
	}
	sizeData := tx.size.Load()
	if ss, ok := sizeData.(common.StorageSize); !ok || ss != sizeData {
		t.Errorf("Unexpected size data.\nGot %T %v\nwant %T %v", ss, ss, common.StorageSize(len(rlpStr)), common.StorageSize(len(rlpStr)))
	}
}

// TestTransaction_MarshalJSON test json.Marshal(Transaction).
func TestTransaction_MarshalJSON(t *testing.T) {
	for _, test := range txJsonData {
		if test.unmarshalError != nil {
			continue
		}
		tx := *test.tx
		res, err := json.Marshal(&tx)
		if err != test.marshalError {
			t.Errorf("Transaction marshal %s give unexpected error.\nGot %v\nWant %v", test.name, err, test.marshalError)
		}
		if test.json != string(res) {
			t.Errorf("Transaction marshal %s give unepected rlp. \nGot %s\n Want %s", test.name, res, test.json)
		}
	}
}

// TestTransaction_UnmarshalJSON test json.Unmarshal(Transaction)
func TestTransaction_UnmarshalJSON(t *testing.T) {
	for _, test := range txJsonData {
		var tx *Transaction
		err := json.Unmarshal([]byte(test.json), &tx)
		CheckError(t, test.name, err, test.unmarshalError, test.unmarshalError2)
		if err != nil {
			continue
		}
		if !checkTxJSONMarshalEqual(tx, test.tx) {
			t.Errorf("Transaction unmarshal %s give unexpected value.\nGot: %sWant: %s", test.name,
				dumper.Sdump(tx.data), dumper.Sdump(&test.tx.data))
		}
	}
}

// TestTransaction_Data test Attribute functions for Transaction.
// Include Transaction.Gas(), Transaction.GasPrice(), Transaction.Value().
// Transaction.Nonce(), Transaction.CheckNonce().
func TestTransaction_Attributes(t *testing.T) {
	for _, test := range txJsonData {
		// Only test for ok and corner
		if test.name != "ok" && test.name != "corner" {
			continue
		}
		// Test tx.Gas()
		tx := *test.tx
		if res := tx.Gas(); res != tx.data.GasLimit {
			t.Errorf("Input %s GasLimit\nGot %d\nWant %d", test.name, res, tx.data.GasLimit)
		}
		// Test tx.GasPrice()
		if res := tx.GasPrice(); res != nil && res.Cmp(tx.data.Price) != 0 {
			t.Errorf("Input %s GasPrice\nGot %d\nWant %d", test.name, res, tx.data.Price)
		}
		// Test tx.Value()
		if res := tx.Value(); res != nil && res.Cmp(tx.data.Amount) != 0 {
			t.Errorf("Input %s Value\nGot %d\nWant %d", test.name, res, tx.data.Amount)
		}
		// Test tx.Nonce()
		if res := tx.Nonce(); res != tx.data.AccountNonce {
			t.Errorf("Input %s Nonce\nGot %d\nWant %d", test.name, res, tx.data.AccountNonce)
		}
		// Test tx.CheckNonce()
		if res := tx.CheckNonce(); !res {
			t.Errorf("Input %s CheckNonce\nGot %v\nWant %v", test.name, res, true)
		}
	}
}

// TestTransaction_Data test transaction.Data()
// This is different from TestTransaction_Attributes since it could deal with nil cases.
// The return value should equal in value and differ in address to the original data.
func TestTransaction_Data(t *testing.T) {
	for _, test := range txJsonData {
		if test.tx == nil {
			continue
		}
		tx := *test.tx
		data := tx.Data()
		if (data == nil && tx.data.Payload != nil) ||
			(tx.data.Payload == nil && data != nil) {
			t.Errorf("Input %s Payload\nGot %x\nWant %x", test.name, data, tx.data.Payload)
		} else if data != nil && tx.data.Payload != nil {
			if !bytes.Equal(data, tx.data.Payload) {
				t.Errorf("Input %s Payload\nGot %x\nWant %x", test.name, data, tx.data.Payload)
			}
			if &data == &tx.data.Payload {
				t.Errorf("Does not copy content for %s Payload. Got %p, Want %p", test.name, data, tx.data.Payload)
			}
		}
	}
}

// TestTransaction_To test Transaction.To.
// This test function To is different from what's implemented before since it could deal with nil case.
// The return value should equal in value and differ in address to the original data.
func TestTransaction_To(t *testing.T) {
	for _, test := range txJsonData {
		// Skip tests for tx is nil
		if test.tx == nil {
			continue
		}
		res := test.tx.To()
		if res == nil {
			if test.tx.data.Recipient != nil {
				t.Errorf("Input %s To\nGot %x\nWant %x", test.name, res, test.tx.data.Recipient)
			}
		} else {
			if *res != *test.tx.data.Recipient {
				t.Errorf("Input %s To\nGot %x\nWant %x", test.name, res, test.tx.data.Recipient)
			}
			if res == test.tx.data.Recipient {
				t.Errorf("Input %s Returned address is identical to original\nGot %p\nWant %p", test.name, res, test.tx.data.Recipient)
			}
		}
	}
}

// TestTransaction_Hash test Transaction.Hash. Function should return expected hash.
// After that, changing the field in transaction, will not change the result Hash since
// the previous hash is already stored.
func TestTransaction_Hash(t *testing.T) {
	tx := *rightvrsTx
	want := common.FromHex("4da580fd2e4c04f328d9f947ecf356411eb8e4a3a5c745f383b3ccd79c36a8d4")
	hash := tx.Hash()
	if !bytes.Equal(hash.Bytes(), want) {
		t.Errorf("Transaction.Hash() return unexpected hash.\nGot %x\nWant %x", hash, want)
	}
	// Change the value of txdata, and Hash again.
	// Hash should retrieve the same hash as previous hash calculation.
	tx.data.AccountNonce = uint64(10130129301)
	if newHash := tx.Hash(); !bytes.Equal(hash.Bytes(), newHash.Bytes()) {
		t.Errorf("Hash again does not give the same hash.\nGot %x\nWant %x", newHash, hash)
	}
}

// TestTransaction_Hash test Transaction.Hash. Function should return expected rlp size.
// After that, changing the field in transaction, will not change the result size since
// the previous size is already stored.
func TestTransaction_Size(t *testing.T) {
	tx := *rightvrsTx
	wantLen := 0
	size := tx.Size()
	if size != common.StorageSize(99) {
		t.Errorf("Transaction.size() return unexpected rlp length. \nGot %v\nWant %v", size, wantLen)
	}
	// Change the value of txdata, and size again. size should retrieve the same hash as
	// previous hash calculation.
	tx.data.AccountNonce = uint64(10130129301)
	if newSize := tx.Size(); newSize != size {
		t.Errorf("size again does not give the same hash.\nGot %v\nWant %v", newSize, size)
	}
}

// TestTransaction_AsMessage test transaction.AsMessage
// 1. Create a signed tx
// 2. Define want value
// 3. Check function returns wanted value.
func TestTransaction_AsMessage(t *testing.T) {
	// Create a signed tx
	tx := NewTransaction(uint64(1), common.BytesToAddress([]byte{1}), common.Big1, uint64(3), common.Big1, []byte("abcdf"))
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	signer := NewEIP155Signer(big.NewInt(1))
	tx, err := SignTx(tx, signer, key)
	if err != nil {
		t.Errorf("Cannot sign the contract")
	}
	// Want value
	want := Message{
		to:         hex2Address("01"),
		from:       addr,
		nonce:      uint64(1),
		amount:     common.Big1,
		gasLimit:   uint64(3),
		gasPrice:   common.Big1,
		data:       []byte("abcdf"),
		checkNonce: true,
	}
	// Check consistency
	res, err := tx.AsMessage(signer)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if !reflect.DeepEqual(res, want) {
		t.Errorf("Not Equal. Got %sWant %s", dumper.Sdump(res), dumper.Sdump(want))
	}
}

// TestTransaction_WithSignature tests Transaction.WithSignature
// Returned tx should
// 1. Have data.R, data.S, data.V with input value
// 2. Have other data fields same as original
// 3. Address not the same as the original
// 4. size, hash, from should have nil value since this a new tx
func TestTransaction_WithSignature(t *testing.T) {
	tx := NewTransaction(uint64(1), common.BytesToAddress([]byte{1}), common.Big1, uint64(3), common.Big1, []byte("abcdf"))
	signer := HomesteadSigner{}
	sig := common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8ea5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ad")
	R, S, V, err := signer.SignatureValues(tx, sig)
	wantData := tx.data
	wantData.R = new(big.Int).Set(R)
	wantData.S = new(big.Int).Set(S)
	wantData.V = new(big.Int).Set(V)

	if err != nil {
		t.Errorf("Cannot get signatureValues for sig %x", sig)
	}
	newTx, err := tx.WithSignature(signer, sig)
	if err != nil {
		t.Errorf("WithSignature return err: %s", err.Error())
	}
	if newTx == tx {
		t.Errorf("After tx.WithSignature returned signature is the same as the original")
	}
	if !reflect.DeepEqual(newTx.data, wantData) {
		t.Errorf("Returned Transaction doesn't have data same as the original.")
	}
	if size := newTx.size.Load(); size != nil {
		t.Errorf("Returned Transaction has size %v", size)
	}
	if hash := newTx.hash.Load(); hash != nil {
		t.Errorf("Returned Transaction has hash %v", hash)
	}
	if from := newTx.from.Load(); from != nil {
		t.Errorf("Returned Transaction has from %v", from)
	}
}

// TestTransaction_RawSignatureValues test RawSignatureValues
func TestTransaction_RawSignatureValues(t *testing.T) {
	tx := NewTransaction(uint64(1), common.BytesToAddress([]byte{1}), common.Big1, uint64(3), common.Big1, []byte("abcdf"))
	signer := HomesteadSigner{}
	sig := common.FromHex("f0f6f18bca1b28cd68e4357452947e021241e9cef27f2e6be0972bb06455bf8ea5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ad")
	wantR, wantS, wantV, err := signer.SignatureValues(tx, sig)
	tx = tx.simpleSignTx(wantR, wantS, wantV)
	if err != nil {
		t.Errorf("Cannot get signatureValues for sig %x", sig)
	}
	V, R, S := tx.RawSignatureValues()
	if R.Cmp(wantR) != 0 {
		t.Errorf("RawSignatureValues return unexpected R\nGot:\t%x\nWant:\t%x", R, wantR)
	}
	if S.Cmp(wantS) != 0 {
		t.Errorf("RawSignatureValues return unexpected S\nGot:\t%x\nWant:\t%x", S, wantS)
	}
	if V.Cmp(wantV) != 0 {
		t.Errorf("RawSignatureValues return unexpected V\nGot:\t%x\nWant:\t%x", V, wantV)
	}
}

// TestTransaction_Cost test Transaction.Cost() == amount + price * gasLimit
func TestTransaction_Cost(t *testing.T) {
	tests := []struct {
		amount, price *big.Int
		gasLimit      uint64
		want          *big.Int
	}{
		{new(big.Int), new(big.Int), uint64(0), new(big.Int)},
		{big.NewInt(1000000000000), big.NewInt(1000), uint64(5000), big.NewInt(1000005000000)},
		{stringToBigInt("1928398718927389719237981273172897389173298", 10), stringToBigInt("123981927389128389759199283", 10), uint64(18446744073709551615), stringToBigInt("2288991283031419454955550034575261868748665343", 10)},
	}
	for i, test := range tests {
		tx := NewTransaction(uint64(1), common.HexToAddress("0x01"), test.amount, test.gasLimit, test.price, []byte{})
		res := tx.Cost()
		if res.Cmp(test.want) != 0 {
			t.Errorf("Transaction.Cost test %d\nGot %v\nWant %v", i, res, test.want)
		}
	}
}

// TestTxDifference test function TxDifference
// 1. Make two Transactions a and b which contains common or excluded txs
// 2. Apply TxDifference
// 3. Check the result is expected.
func TestTxDifference(t *testing.T) {
	// Make Transactions a and b
	// a have nonce 0-9, b have nonce 5-14
	length := 10
	aStart := 0
	bStart := 5
	a := make(Transactions, 0, length)
	for i := aStart; i != length; i++ {
		a = append(a, simpleNewTransactionByNonce(uint64(i)))
	}
	// Make transactions b
	b := make(Transactions, 0, length)
	for i := bStart; i != length; i++ {
		b = append(b, simpleNewTransactionByNonce(uint64(i)))
	}
	// Expect to return tx having nonce 0-4
	want := make(map[common.Hash]int)
	for i := aStart; i != bStart; i++ {
		want[simpleNewTransactionByNonce(uint64(i)).Hash()] = i
	}
	// Apply TxDifference and check result
	res := TxDifference(a, b)
	for _, diff := range res {
		hash := diff.Hash()
		if _, ok := want[hash]; !ok {
			t.Errorf("TxDifference Returned unexpected tx: %d", diff.data.AccountNonce)
		} else {
			delete(want, hash)
		}
	}
	for _, index := range want {
		t.Errorf("Didn't find expected transaction with nonce: %d", index)
	}
}

func TestTransactions_Swap(t *testing.T) {
	txs := make(Transactions, 0, 10)
	for i := 0; i != 10; i++ {
		txs = append(txs, simpleNewTransactionByNonce(uint64(i)))
	}
	// swap index 3, 7
	want3, want7 := txs[7], txs[3]
	txs.Swap(3, 7)
	if txs[3] != want3 || txs[7] != want7 {
		t.Errorf("Transactions Swap doesn't have expected value. \nGot %p / %p\nWant %p / %p",
			txs[3], txs[7], want3, want7)
	}
}

// TestTxByNonce test whether the type TxByNonce implements sort.Sorter.
// and whether sort.Sort could be applied to the type according to tx.data.AccountNonce
func TestTxByNonce(t *testing.T) {
	// Create 25 transaction
	numGen := 25
	tbn := make(TxByNonce, 0, numGen)
	for i := 0; i != numGen; i++ {
		tbn = append(tbn, simpleNewTransactionByNonce(r.Uint64()))
	}
	if tbn.Len() != numGen {
		t.Errorf("After inserting Transactions, Len() unexpected value\nGot %d\nWant %d", tbn.Len(),
			numGen)
	}
	sort.Sort(tbn)
	var pre *Transaction
	for _, tx := range tbn {
		if pre == nil {
			pre = tx
			continue
		}
		if pre.data.AccountNonce > tx.data.AccountNonce {
			t.Errorf("Previous nonce larger than latter. Prev: %d\nCur: %d", pre.data.AccountNonce,
				tx.data.AccountNonce)
		}
	}
}

// TestTxByPrice test whether the type TxByPrice implements sort.Sorter.
// and whether sort.Sort could be applied to the type according to tx.data.Price
func TestTxByPrice(t *testing.T) {
	// Create 25 transaction
	numGen := 25
	tbp := make(TxByPrice, 0, numGen)
	tbp.Push(simpleNewTransactionByPrice(big.NewInt(r.Int63())))
	// sort tbp
	sort.Sort(tbp)
	// Pop and check sorting order
	pre := tbp.Pop().(*Transaction)
	for tbp.Len() > 0 {
		cur := tbp.Pop().(*Transaction)
		if pre.data.Price.Cmp(cur.data.Price) < 0 {
			t.Errorf("Previous Price smaller than latter. \nPrev: \t%d\nCur: \t%d", pre.data.Price,
				cur.data.Price)
		}
		pre = cur
	}
}

// TestNewMessage test NewMessage
func TestNewMessage(t *testing.T) {
	tests := []Message{
		{
			from:       common.HexToAddress("0x01"),
			to:         hex2Address("0x02"),
			nonce:      uint64(1),
			amount:     big.NewInt(100),
			gasLimit:   uint64(10),
			gasPrice:   big.NewInt(100),
			data:       []byte{},
			checkNonce: true,
		},
	}
	for _, test := range tests {
		m := NewMessage(test.from, test.to, test.nonce, test.amount, test.gasLimit, test.gasPrice,
			test.data, test.checkNonce)
		if !reflect.DeepEqual(m, test) {
			t.Errorf("New message return unexpected value.\nGot %s\nWant %s", dumper.Sdump(m),
				dumper.Sdump(test))
		}
	}
}

// TestMessage_Attributes test Message.From(), Message.To(), Message.GasPrice(), Message.Value(),
// Message.Gas(), Message.Nonce(), Message.Data(). Message.CheckNonce().
func TestMessage_Attributes(t *testing.T) {
	tests := []Message{
		{
			from:       common.HexToAddress("0x01"),
			to:         hex2Address("0x02"),
			nonce:      uint64(1),
			amount:     big.NewInt(100),
			gasLimit:   uint64(10),
			gasPrice:   big.NewInt(100),
			data:       []byte{},
			checkNonce: true,
		},
		{
			from:       common.HexToAddress("0x0000000000000000000000000000000000000001"),
			to:         hex2Address("0x" + strings.Repeat("89f1", 8)),
			nonce:      uint64(18446744073709551615),
			amount:     stringToBigInt("1283798819823798174879817892379812789718932", 10),
			gasLimit:   uint64(18446744073709551615),
			gasPrice:   stringToBigInt("1283798819823798174879817892379812789718932", 10),
			data:       []byte("lksakodjfklaodfioq3ij jioasiojfiaodsjfoiajsdifoioajiosdfjioajiosdfjioasdjfoiajsodf"),
			checkNonce: false,
		},
	}
	for _, test := range tests {
		// Test From()
		if from := test.From(); !bytes.Equal(from.Bytes(), test.from.Bytes()) {
			t.Errorf("Message.From return unexpected value\nGot %x\nWant %x", from.String(), test.from.String())
		}
		// test To()
		if to := test.To(); to != test.to {
			t.Errorf("Message.To return unexpected value\nGot %p %v\nWant %p %v", to, to, test.to, test.to)
		}
		// Test GasPrice()
		if gasPrice := test.GasPrice(); gasPrice.Cmp(test.gasPrice) != 0 {
			t.Errorf("Message.GasPrice return unexpected value\nGot %v\nWant %v", gasPrice, test.gasPrice)
		}
		// test Value()
		if value := test.Value(); value.Cmp(test.amount) != 0 {
			t.Errorf("Message.Value return unexpected value\nGot %v\nWant %v", value, test.amount)
		}
		// test Gas()
		if gas := test.Gas(); gas != test.gasLimit {
			t.Errorf("Message.Gas return unexpected value\nGot %d\nWant %d", gas, test.gasLimit)
		}
		// test Nonce()
		if nonce := test.Nonce(); nonce != test.nonce {
			t.Errorf("Message.Nonce return unexpected value\nGot %d\nWant %d", nonce, test.nonce)
		}
		// test Data()
		if data := test.Data(); !bytes.Equal(data, test.data) {
			t.Errorf("Message.Data return unexpected value\nGot %x\nWant %x", data, test.data)
		}
		// Test CheckNonce()
		if cn := test.CheckNonce(); cn != test.checkNonce {
			t.Errorf("Message.CheckNonce return unexpected value\nGot %v\nWant %v", cn, test.checkNonce)
		}
	}
}

func checkTxJSONMarshalEqual(a, b *Transaction) bool {
	if a.data.AccountNonce != b.data.AccountNonce {
		return false
	}
	if a.data.Price.Cmp(b.data.Price) != 0 {
		return false
	}
	if a.data.GasLimit != b.data.GasLimit {
		return false
	}
	if !bytes.Equal(a.data.Recipient.Bytes(), b.data.Recipient.Bytes()) {
		return false
	}
	if a.data.Amount.Cmp(b.data.Amount) != 0 {
		return false
	}
	if !bytes.Equal(a.data.Payload, b.data.Payload) {
		return false
	}
	if a.data.V.Cmp(b.data.V) != 0 {
		return false
	}
	if a.data.R.Cmp(b.data.R) != 0 {
		return false
	}
	if a.data.S.Cmp(b.data.S) != 0 {
		return false
	}
	return true
}

// checkLogUnmarshalError checks whether the got error equals the want error.
func checkTransactionUnmarshalError(t *testing.T, testname string, got, want error) bool {
	if got == nil {
		if want != nil {
			t.Errorf("test %q: got no error, want %q", testname, want)
			return false
		}
		return true
	}
	if want == nil {
		t.Errorf("test %q: unexpected error %q", testname, got)
	} else if got.Error() != want.Error() {
		t.Errorf("test %q: got error %q, want %q", testname, got, want)
	}
	return false
}

func stringToBigInt(str string, base int) *big.Int {
	res, ok := new(big.Int).SetString(str, base)
	if !ok {
		panic(fmt.Sprintf("Cannot convert string %s-%d to big.Int", str, base))
	}
	return res
}

func hex2Address(str string) *common.Address {
	addr := common.HexToAddress(str)
	return &addr
}

func hex2Hash(str string) *common.Hash {
	hash := common.HexToHash(str)
	return &hash
}

func simpleNewTransactionByNonce(nonce uint64) *Transaction {
	return newTransaction(nonce, hex2Address("0x01"), new(big.Int), 0, new(big.Int), []byte{})
}

func simpleNewTransactionByPrice(price *big.Int) *Transaction {
	return newTransaction(uint64(0), hex2Address("0x01"), new(big.Int), 0, price, []byte{})
}

func (tx *Transaction) simpleSignTx(R, S, V *big.Int) *Transaction {
	tx.data.R = new(big.Int).Set(R)
	tx.data.S = new(big.Int).Set(S)
	tx.data.V = new(big.Int).Set(V)
	return tx
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
// The values in those tests are from the Transaction Tests
// at github.com/ethereum/tests.
var (
	emptyTx = NewTransaction(
		0,
		common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"),
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)

	rightvrsTx, _ = NewTransaction(
		3,
		common.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b"),
		big.NewInt(10),
		2000,
		big.NewInt(1),
		common.FromHex("5544"),
	).WithSignature(
		HomesteadSigner{},
		common.Hex2Bytes("98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"),
	)
)

func TestTransactionSigHash(t *testing.T) {
	var homestead HomesteadSigner
	if homestead.Hash(emptyTx) != common.HexToHash("c775b99e7ad12f50d819fcd602390467e28141316969f4b57f0626f74fe3b386") {
		t.Errorf("empty transaction hash mismatch, got %x", emptyTx.Hash())
	}
	if homestead.Hash(rightvrsTx) != common.HexToHash("fe7a79529ed5f7c3375d06b26b186a8644e0e16c373d7a12be41c62d6042b77a") {
		t.Errorf("RightVRS transaction hash mismatch, got %x", rightvrsTx.Hash())
	}
}

func TestTransactionEncode(t *testing.T) {
	txb, err := rlp.EncodeToBytes(rightvrsTx)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	should := common.FromHex("f86103018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8255441ca098ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4aa08887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a3")
	if !bytes.Equal(txb, should) {
		t.Errorf("encoded RLP mismatch, got %x", txb)
	}
}

func decodeTx(data []byte) (*Transaction, error) {
	var tx Transaction
	t, err := &tx, rlp.Decode(bytes.NewReader(data), &tx)

	return t, err
}

func defaultTestKey() (*ecdsa.PrivateKey, common.Address) {
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return key, addr
}

func TestRecipientEmpty(t *testing.T) {
	_, addr := defaultTestKey()
	tx, err := decodeTx(common.Hex2Bytes("f8498080808080011ca09b16de9d5bdee2cf56c28d16275a4da68cd30273e2525f3959f5d62557489921a0372ebd8fb3345f7db7b5a86d42e24d36e983e259b0664ceb8c227ec9af572f3d"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	from, err := Sender(HomesteadSigner{}, tx)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if addr != from {
		t.Error("derived address doesn't match")
	}
}

func TestRecipientNormal(t *testing.T) {
	_, addr := defaultTestKey()

	tx, err := decodeTx(common.Hex2Bytes("f85d80808094000000000000000000000000000000000000000080011ca0527c0d8f5c63f7b9f41324a7c8a563ee1190bcbf0dac8ab446291bdbf32f5c79a0552c4ef0a09a04395074dab9ed34d3fbfb843c2f2546cc30fe89ec143ca94ca6"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	from, err := Sender(HomesteadSigner{}, tx)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if addr != from {
		t.Error("derived address doesn't match")
	}
}

// Tests that transactions can be correctly sorted according to their price in
// decreasing order, but at the same time with increasing nonces when issued by
// the same account.
func TestTransactionPriceNonceSort(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*ecdsa.PrivateKey, 25)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
	}

	signer := HomesteadSigner{}
	// Generate a batch of transactions with overlapping values, but shifted nonces
	groups := map[common.Address]Transactions{}
	for start, key := range keys {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		for i := 0; i < 25; i++ {
			tx, _ := SignTx(NewTransaction(uint64(start+i), common.Address{}, big.NewInt(100), 100, big.NewInt(int64(start+i)), nil), signer, key)
			groups[addr] = append(groups[addr], tx)
		}
	}

	// Sort the transactions and cross check the nonce ordering
	txset := NewTransactionsByPriceAndNonce(signer, groups)
	txs := Transactions{}
	i := 0
	for tx := txset.Peek(); tx != nil; tx = txset.Peek() {
		if i == 25 {
			// The 2nd account is unavailable
			txset.Pop()
		} else {
			txs = append(txs, tx)
			txset.Shift()
		}
		i++
	}
	if len(txs) != 24*25 {
		t.Errorf("expected %d transactions, found %d", 24*25, len(txs))
	}
	for i, txi := range txs {
		fromi, _ := Sender(signer, txi)

		// Make sure the nonce order is valid
		for j, txj := range txs[i+1:] {
			fromj, _ := Sender(signer, txj)
			if fromi == fromj && txi.Nonce() > txj.Nonce() {
				fmt.Printf("%d - %d\n", txi.Nonce(), txj.Nonce())
				t.Errorf("invalid nonce ordering: tx #%d (A=%x N=%v) < tx #%d (A=%x N=%v)", i, fromi[:4], txi.Nonce(), i+j, fromj[:4], txj.Nonce())
			}
		}

		// If the next tx has different from account, the price must be lower than the current one
		if i+1 < len(txs) {
			next := txs[i+1]
			fromNext, _ := Sender(signer, next)
			if fromi != fromNext && txi.GasPrice().Cmp(next.GasPrice()) < 0 {
				t.Errorf("invalid gasprice ordering: tx #%d (A=%x P=%v) < tx #%d (A=%x P=%v)", i, fromi[:4], txi.GasPrice(), i+1, fromNext[:4], next.GasPrice())
			}
		}
	}
}

// TestTransactionJSON tests serializing/de-serializing to/from JSON.
func TestTransactionJSON(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("could not generate key: %v", err)
	}
	signer := NewEIP155Signer(common.Big1)

	transactions := make([]*Transaction, 0, 50)
	for i := uint64(0); i < 25; i++ {
		var tx *Transaction
		switch i % 2 {
		case 0:
			tx = NewTransaction(i, common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))
		case 1:
			tx = NewContractCreation(i, common.Big0, 1, common.Big2, []byte("abcdef"))
		}
		transactions = append(transactions, tx)

		signedTx, err := SignTx(tx, signer, key)
		if err != nil {
			t.Fatalf("could not sign transaction: %v", err)
		}

		transactions = append(transactions, signedTx)
	}

	for _, tx := range transactions {
		data, err := json.Marshal(tx)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}

		var parsedTx *Transaction
		if err := json.Unmarshal(data, &parsedTx); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// compare nonce, price, gaslimit, recipient, amount, payload, V, R, S
		if tx.Hash() != parsedTx.Hash() {
			t.Errorf("parsed tx differs from original tx, want %v, got %v", tx, parsedTx)
		}
		if tx.ChainId().Cmp(parsedTx.ChainId()) != 0 {
			t.Errorf("invalid chain id, want %d, got %d", tx.ChainId(), parsedTx.ChainId())
		}
	}
}
