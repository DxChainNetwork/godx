package writeaheadlog

import (
	"math/rand"
	"testing"
)

func BenchmarkPage_Marshal(b *testing.B) {
	randomBytes := make([]byte, PageSize-pageMetaSize)
	rand.Read(randomBytes)
	p := page{
		offset:   4096,
		payload:  randomBytes,
		nextPage: &page{offset: 11111},
	}
	buf := make([]byte, PageSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p.marshal(buf[:0])
	}
}
