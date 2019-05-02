package erasurecode

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"
)

func TestNewStandardErasureCode(t *testing.T) {
	tests := []struct {
		minSectors uint32
		numSectors uint32
		expectErr  error
	}{
		{minSectors: 1, numSectors: 2, expectErr: nil},
		{minSectors: 0, numSectors: 1, expectErr: errors.New("0 not allowed for minSectors")},
		{minSectors: 1, numSectors: 1, expectErr: errors.New("minSectors > numSectors")},
	}
	for i, test := range tests {
		_, err := newStandardErasureCode(test.minSectors, test.numSectors)
		if (err == nil) != (test.expectErr == nil) {
			t.Errorf("Test %d: expect error: %v, got error: %v", i, test.expectErr, err)
		}
	}
}

func TestStandardErasureCode_Encode_Recover(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	tests := []struct {
		minSectors uint32
		numSectors uint32
		data       []byte
	}{
		{1, 2, randomBytes(1)},
		{1, 10, randomBytes(10)},
		{10, 11, randomBytes(10)},
		{10, 30, randomBytes(4096)},
	}
	for i, test := range tests {
		sec, err := newStandardErasureCode(test.minSectors, test.numSectors)
		if err != nil {
			t.Fatalf("Test %d: cannot new sec: %v", i, err)
		}
		encoded, err := sec.Encode(test.data)
		if err != nil {
			t.Fatalf("Test %d: cannot encode: %v", i, err)
		}
		// remove some of the encoded string
		removeIndex := rand.Perm(int(test.numSectors))[:test.numSectors-test.minSectors]
		for _, j := range removeIndex {
			encoded[j] = nil
		}
		recovered := new(bytes.Buffer)
		err = sec.Recover(encoded, len(test.data), recovered)
		if err != nil {
			t.Errorf("cannot recover data: %v", err)
		}
		if !bytes.Equal(recovered.Bytes(), test.data) {
			t.Errorf("Test %d: data not equal:\n\tExpect %x\n\tGot %x", i, test.data, recovered)
		}
	}
}

func BenchmarkStandardErasureCode_Encode(b *testing.B) {
	rsc, err := newStandardErasureCode(80, 100)
	if err != nil {
		b.Fatal(err)
	}
	data := randomBytes(1 << 20)

	b.SetBytes(1 << 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rsc.Encode(data)
	}
}

func BenchmarkStandardErasureCode_Recover(b *testing.B) {
	rsc, err := newStandardErasureCode(50, 200)
	if err != nil {
		b.Fatal(err)
	}
	data := randomBytes(1 << 20)
	pieces, err := rsc.Encode(data)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(1 << 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(pieces)/2; j += 2 {
			pieces[j] = nil
		}
		rsc.Recover(pieces, 1<<20, ioutil.Discard)
	}
}

func randomBytes(num int) []byte {
	data := make([]byte, num)
	_, err := rand.Read(data)
	if err != nil {
		panic(fmt.Sprintf("cannot generate random data: %v", err))
	}
	return data
}
