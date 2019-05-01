package dxfile

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"sync"
)

type (
	// DxFile is saved to disk with sequence:
	// headerLength | ChunkOffset | header             | dataSegment
	// 0:8          | 8:16        | 16:16+headerLength | segmentOffset:
	DxFile struct {
		// headerLength is the size of the rlp string of header, which is put
		headerLength uint64

		// segmentOffset is the offset of the first segment
		segmentOffset  uint64

		// header is the persist header is the header of the dxfile
		header persistHeader

		// dataSegments is a list of segments the file is split into
		dataSegments []Segment

		// utils field
		deleted bool
		lock    sync.RWMutex
		ID      string
		wal     *writeaheadlog.Wal

		// filename is the file of the content locates
		filename string
	}

	// persistHeader has two
	persistHeader struct {
		// metadata includes all info related to dxfile that is ready to be flushed to data file
		metadata metadata

		// hostAddresses is a list of addresses that contains address and whether the host
		// is used
		hostAddresses []hostAddress
	}

	// hostAddress is a combination of host address for a dxfile and whether the specific host is used in the dxfile
	// when encoding, the default rlp encoding algorithm is used
	hostAddress struct {
		address common.Hash
		used    bool
	}

	// Segment is the structure a dxfile is split into
	Segment struct {
		// TODO: Check the ExtensionInfo could be actually removed
		sectors [][]Sector // sectors contains the recoverable message about the Sector in the Segment
		stuck   bool       // stuck indicates whether the Segment is stuck or not
	}

	// Sector is the smallest unit of storage. It the erasure code encoded Segment
	Sector struct {
		hostAddress common.Address
		merkleRoot  common.Hash
	}
)
