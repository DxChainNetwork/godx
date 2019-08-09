package types

import (
	"math/big"
	"math/rand"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
)

var RandomHash = common.BytesToHash([]byte("random"))
var RandomBigInt = new(big.Int).SetBytes([]byte("dxchainRandomNumber"))
var RandomAddr = common.BytesToAddress([]byte("random"))
var r = rand.New(rand.NewSource(time.Now().UnixNano()))

// MakeRandomBlock makes a random new block with give parameter. uncles are not set.
func MakeRandomBlock(txLen int, parentHash common.Hash, blockNumber *big.Int) (*Block, []*Receipt) {
	h := MakeRandomHeader(parentHash, blockNumber)

	// Make uncles
	txs := make([]*Transaction, 0, txLen)
	rs := make([]*Receipt, 0, txLen)
	signer := NewEIP155Signer(big.NewInt(1))

	addrBuf := make([]byte, common.AddressLength)
	for i := 0; i != txLen; i++ {
		// Create a transaction
		tx := MakeRandomTransaction()
		key, _ := crypto.GenerateKey()
		// Sign transaction
		tx, _ = SignTx(tx, signer, key)
		txs = append(txs, tx)
		hash := tx.Hash()
		// Make receipt
		rand.Read(addrBuf)
		r := MakeReceiptData(hash, common.BytesToAddress(addrBuf))
		rs = append(rs, r)
	}

	// Create a Block with txs and rs
	b := NewBlock(h, txs, []*Header{}, rs)
	bHash := b.Hash()

	// Create logs
	logIndex := 0
	for txIndex, tx := range b.transactions {
		tHash := tx.Hash()
		len := r.Intn(10)
		logs := MakeRandomLogs(len, blockNumber.Uint64(), bHash, tHash, uint(txIndex), uint(logIndex))
		rs[txIndex].Logs = logs
		logIndex += len
	}
	return b, rs
}

// makeLogsData makes some Log data with totally random value of length len
// Parameters passed in will be within Log structure.
func MakeRandomLogs(len int, blockNumber uint64, blockHash common.Hash, txHash common.Hash, txIndex uint, startIndex uint) []*Log {
	logs := make([]*Log, 0, len)
	for i := 0; i != len; i++ {
		// Create a random Log
		l := MakeLogData(blockNumber, blockHash, txHash, txIndex, startIndex+uint(i))
		logs = append(logs, l)
	}
	return logs
}

// MakeLogData makes a random log data based on info given.
// Random fields are: Address, topics (with length topicLen), data (with random length).
func MakeLogData(blockNumber uint64, blockHash common.Hash, txHash common.Hash, txIndex uint, index uint) *Log {
	l := Log{}
	addrBuf := make([]byte, common.AddressLength)
	hashBuf := make([]byte, common.HashLength)
	r.Read(addrBuf)
	l.Address = common.BytesToAddress(addrBuf)

	// Create random number of random topics
	topicLen := r.Intn(10)
	topics := make([]common.Hash, topicLen)
	for ti := 0; ti != topicLen; ti++ {
		r.Read(hashBuf)
		topics[ti] = common.BytesToHash(hashBuf)
	}
	l.Topics = topics

	// Create random data
	data := make([]byte, r.Intn(1000))
	r.Read(data)
	l.Data = make([]byte, len(data))
	copy(l.Data, data)

	// Copy passed in values
	l.BlockNumber = blockNumber
	l.TxHash = txHash
	l.TxIndex = txIndex
	l.BlockHash = blockHash
	l.Index = index
	l.Removed = false

	return &l
}

// makeReceiptsData make some random receipts
func MakeReceipts(len int, contractAddress common.Address) Receipts {
	receipts := make(Receipts, len)
	hashBuf := make([]byte, common.HashLength)
	for i := 0; i != len; i++ {
		r.Read(hashBuf)
		txHash := common.BytesToHash(hashBuf)
		r := MakeReceiptData(txHash, contractAddress)
		receipts[i] = r
	}
	return receipts
}

func MakeReceiptData(txHash common.Hash, contractAddress common.Address) *Receipt {
	receipt := Receipt{}
	postState := make([]byte, common.HashLength)
	r.Read(postState)
	receipt.PostState = postState
	receipt.Status = ReceiptStatusSuccessful
	receipt.CumulativeGasUsed = r.Uint64()
	receipt.Logs = []*Log{}
	receipt.Bloom.SetBytes([]byte{})

	receipt.TxHash = txHash
	receipt.ContractAddress = contractAddress
	receipt.GasUsed = r.Uint64()
	return &receipt
}

func MakeTransactionWithReceipt(contractAddress common.Address) (*Transaction, *Receipt) {
	tx := MakeRandomTransaction()
	hash := tx.Hash()
	receipt := MakeReceiptData(hash, contractAddress)
	return tx, receipt
}

func MakeRandomTransaction() *Transaction {
	addrBuf := make([]byte, common.AddressLength)
	r.Read(addrBuf)
	rec := common.BytesToAddress(addrBuf)

	payload := make([]byte, r.Intn(300))
	r.Read(payload)
	return &Transaction{
		data: txdata{
			AccountNonce: r.Uint64(),
			Price:        big.NewInt(int64(r.Uint64())),
			GasLimit:     r.Uint64(),
			Recipient:    &rec,
			Amount:       big.NewInt(int64(r.Uint64())),
			Payload:      payload,
		},
	}
}

// MakeRandomHeader makes a random header with random Root,
// Difficulty,, GasLimit, GasUsed, Time, Extra, MixDigest, and Nonce.
// If parentHash == RandomHash, ParentHash is also randomly picked.
// If number == RandomBigInt, Number is also randomly picked.
func MakeRandomHeader(parentHash common.Hash, number *big.Int) *Header {
	hashBuf := make([]byte, common.HashLength)
	addrBuf := make([]byte, common.AddressLength)
	if parentHash == RandomHash {
		r.Read(hashBuf)
		parentHash = common.BytesToHash(hashBuf)
	}
	r.Read(hashBuf)
	root := common.BytesToHash(hashBuf)
	r.Read(addrBuf)
	coinBase := common.BytesToAddress(addrBuf)
	difficulty := new(big.Int).SetUint64(r.Uint64())
	if number.Cmp(RandomBigInt) == 0 {
		number.SetUint64(r.Uint64())
	}
	gasLimit := r.Uint64()
	gasUsed := r.Uint64()
	hTime := new(big.Int).SetUint64(r.Uint64())
	extra := make([]byte, r.Intn(200))
	r.Read(extra)
	r.Read(hashBuf)
	mixDigest := common.BytesToHash(hashBuf)
	nonceBuf := [8]byte{}
	r.Read(nonceBuf[:])
	nonce := BlockNonce(nonceBuf)
	return &Header{
		ParentHash: parentHash,
		Coinbase:   coinBase,
		Root:       root,
		Difficulty: difficulty,
		Number:     number,
		GasLimit:   gasLimit,
		GasUsed:    gasUsed,
		Time:       hTime,
		Extra:      extra,
		MixDigest:  mixDigest,
		Nonce:      nonce,
	}
}

func MakeRandomBlockForTest(txLen int, uncleLen int, blockNumber *big.Int) *Block {
	txs := make(Transactions, txLen)
	receipts := make(Receipts, txLen)
	for i := 0; i != txLen; i++ {
		txs[i], receipts[i] = MakeTransactionWithReceipt(common.Address{})
	}
	uncles := make([]*Header, uncleLen)
	for i := 0; i != uncleLen; i++ {
		uncles[i] = MakeRandomHeader(RandomHash, RandomBigInt)
	}
	h := MakeRandomHeader(RandomHash, blockNumber)
	b := NewBlock(h, txs, uncles, receipts)
	return b
}
