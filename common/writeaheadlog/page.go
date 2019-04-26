package writeaheadlog

import (
	"encoding/binary"
	"fmt"
	"github.com/DxChainNetwork/godx/common/math"
)

const (
	PageSize       = 4096 // size of the page.
	PageMetaSize   = 8    // size of the page.offset field, which is uint64
	MaxPayloadSize = PageSize - PageMetaSize
)

// page is a in-memory linked page list.
// When marshalled, page is [p.nextPage.offset | p.payload]
type page struct {
	// nextPage points to the next page, makes up a linked page list
	nextPage *page

	// offset is the offset of the current page
	offset uint64

	// payload is actual data in the memory page. It is updates in this module
	payload []byte
}

// The on-disk size is the size of (offset + payload)
func (p page) size() int { return PageMetaSize + len(p.payload) }

// nextOffset return the offset of the next page. if the next page is nil, return max uint64
func (p page) nextOffset() uint64 {
	if p.nextPage == nil {
		return math.MaxUint64
	}
	return p.nextPage.offset
}

// marshal marshal the page to buffer
func (p *page) marshal(buf []byte) []byte {
	// page shall not exceed the size of PageSize
	if p.size() > PageSize {
		panic(fmt.Sprintf("page(%d) too large: %d bytes", p.offset, p.size()))
	}
	var b []byte
	if rest := buf[len(buf):]; cap(rest) >= p.size() {
		b = rest[:p.size()]
	} else {
		b = make([]byte, p.size())
	}
	// marshal the page content
	binary.LittleEndian.PutUint64(b[0:], p.nextOffset())
	copy(b[8:], p.payload)

	return append(buf, b...)
}
