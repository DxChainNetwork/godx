package writeaheadlog

import (
	"bytes"
	"fmt"
	"testing"
)

// TODO: remove the test
func TestChecksum(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 4096-pageMetaSize)
	p := &page{
		offset:   4096,
		payload:  data,
		nextPage: &page{offset: 11111, payload: data},
	}
	txn := Transaction{
		txnId: 1,
		status: 100,
		headPage: p,
	}
	fmt.Println(txn.checksum())
}