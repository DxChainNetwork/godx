package dxfile

const (
	// PageSize is the page size of persist data
	PageSize = 4096

	// sectorPersistSize is the size of rlp encoded string of a sector
	sectorPersistSize = 58

	// Overhead for PersistSegment persist data. The value is larger than data actually used
	segmentPersistOverhead = 16

	// Duplication rate is the expected duplication of the size of the sectors
	redundancyRate float64 = 1.0
)

type (
	// PersistHostTable is the table to be stored in dxfile
	PersistHostTable []*PersistHostAddress

	// PersistHostAddress is a combination of host address for a dxfile and whether the specific host is used in the dxfile
	// when encoding, the default rlp encoding algorithm is used
	PersistHostAddress struct {
		HostAddress hostAddress
	}

	// PersistSegment is the structure a dxfile is split into
	PersistSegment struct {
		Sectors [][]*PersistSector // Sectors contains the recoverable message about the PersistSector in the PersistSegment
	}

	// PersistSector is the smallest unit of storage. It the erasure code encoded PersistSegment
	PersistSector struct {
		Sector sector
	}
)
