package writeaheadlog

import (
	"bytes"
	"math/rand"
	"testing"
	"fmt"
)

func BenchmarkPage_Marshal(b *testing.B) {
	randomBytes := make([]byte, pageSize-pageMetaSize)
	rand.Read(randomBytes)
	p := page{
		offset:   4096,
		payload:  randomBytes,
		nextPage: &page{offset: 11111},
	}
	buf := make([]byte, pageSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p.marshal(buf[:0])
	}
}

// TODO: remove the test
func TestPage(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 500-pageMetaSize)
	p := page{
		offset:4096,
		payload: data,
		nextPage: &page{offset: 11111},
	}
	buf := make([]byte, pageSize)
	fmt.Println(p.marshal(buf[:0]))
	fmt.Println(len(p.marshal(buf[:0])))
}
