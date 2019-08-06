// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

var testDir = tempDir("dxfile")

// NewRandomDxFile creates a new random DxFile with random segments data.
// missRate is the params indicate the possibility that a sector of a segment is missed.
// If not set, all sectors will be available.
func (fs *FileSet) NewRandomDxFile(dxPath storage.DxPath, minSectors, numSectors uint32, ecCode uint8, ck crypto.CipherKey, fileSize uint64, missRate float32) (*FileSetEntryWithID, error) {
	// create the file
	sourcePath := storage.SysPath(filepath.Join(userHomeDir(), "temp", dxPath.Path))
	force := false
	ec, _ := erasurecode.New(ecCode, minSectors, numSectors, 64)
	fileMode := os.FileMode(0600)
	df, err := fs.NewDxFile(dxPath, sourcePath, force, ec, ck, fileSize, fileMode)
	if err != nil {
		return nil, err
	}
	// Add the segments
	df.lock.Lock()
	defer df.lock.Unlock()
	for i := 0; uint64(i) != df.metadata.numSegments(); i++ {
		seg := randomSegment(df.metadata.NumSectors, missRate)
		seg.Index = uint64(i)
		df.segments[i] = seg
		for _, sectors := range seg.Sectors {
			for _, sector := range sectors {
				df.hostTable[sector.HostID] = true
			}
		}
	}
	if err = df.saveAll(); err != nil {
		return nil, err
	}
	return df, nil
}

// newTestDxFile generate a random DxFile used for testing. The generated DxFile segments are empty
func newTestDxFile(t *testing.T, fileSize uint64, minSectors, numSectors uint32, ecCode uint8) (*DxFile, error) {
	ec, _ := erasurecode.New(ecCode, minSectors, numSectors, 64)
	ck, _ := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	path, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	filename := testDir.Join(path)
	wal, txns, _ := writeaheadlog.New(filepath.Join(string(testDir), t.Name()+".wal"))
	for _, txn := range txns {
		txn.Release()
	}
	df, err := New(filename, path, storage.SysPath(filepath.Join("~/tmp", t.Name())), wal, ec, ck, fileSize, 0777)
	if err != nil {
		return nil, err
	}
	return df, nil
}

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) storage.SysPath {
	path := filepath.Join(os.TempDir(), "dxfile", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v", path))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return storage.SysPath(path)
}

// userHomeDir returns the home directory of user
func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	} else if runtime.GOOS == "linux" {
		home := os.Getenv("XDG_CONFIG_HOME")
		if home != "" {
			return home
		}
	}
	return os.Getenv("HOME")
}

// newTestDxFileWithSegments generate a random DxFile with some segment data.
func newTestDxFileWithSegments(t *testing.T, fileSize uint64, minSectors, numSectors uint32, ecCode uint8) (*DxFile, error) {
	df, err := newTestDxFile(t, fileSize, minSectors, numSectors, ecCode)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; uint64(i) != df.metadata.numSegments(); i++ {
		seg := randomSegment(df.metadata.NumSectors)
		seg.Index = uint64(i)
		df.segments[i] = seg
		for _, sectors := range seg.Sectors {
			for _, sector := range sectors {
				df.hostTable[sector.HostID] = true
			}
		}
	}
	if err = df.saveAll(); err != nil {
		return nil, err
	}
	return df, nil
}

// randomHostTable create a random hostTable
func randomHostTable(numHosts int) hostTable {
	ht := make(hostTable)
	for i := 0; i != numHosts; i++ {
		ht[randomAddress()] = randomBool()
	}
	return ht
}

// randomSegment create a random segment. At missRate, the sector will not present
func randomSegment(numSectors uint32, missRate ...float32) *Segment {
	seg := &Segment{Sectors: make([][]*Sector, numSectors)}
	var miss bool
	if len(missRate) != 0 {
		mrand.Seed(time.Now().UnixNano())
		miss = missRate[0] >= 0
	}
	for i := range seg.Sectors {
		if miss {
			num := mrand.Float32()
			if num < missRate[0] {
				continue
			}
		}
		seg.Sectors[i] = append(seg.Sectors[i], randomSector())
	}
	return seg
}

// randomSector create a random sector
func randomSector() *Sector {
	s := &Sector{}
	rand.Read(s.HostID[:])
	rand.Read(s.MerkleRoot[:])
	return s
}

// randomAddress create a random enodeID
func randomAddress() (addr enode.ID) {
	rand.Read(addr[:])
	return
}

// randomBool create a random true/false
func randomBool() bool {
	b := make([]byte, 2)
	rand.Read(b)
	num := binary.LittleEndian.Uint16(b)
	return num%2 == 0
}

// randomUint64 create a random Uint64
func randomUint64() uint64 {
	b := make([]byte, 8)
	rand.Read(b)
	return binary.LittleEndian.Uint64(b)
}

// randomBytes create a random bytes of size input num
func randomBytes(num int) []byte {
	b := make([]byte, num)
	rand.Read(b)
	return b
}

// randomHash creates a random hash
func randomHash() common.Hash {
	var h common.Hash
	rand.Read(h[:])
	return h
}
