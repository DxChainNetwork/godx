package erasurecode

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewErasureCoder(t *testing.T) {
	tests := []struct {
		ecType     uint8
		minSectors uint32
		numSectors uint32
		extra      []interface{}
		expectType reflect.Type
		expectErr  error
	}{
		{ECTypeStandard, 1, 2, nil, reflect.TypeOf(&standardErasureCode{}), nil},
		{ECTypeStandard, 1, 2, []interface{}{64}, reflect.TypeOf(&standardErasureCode{}), nil},
		{ECTypeShard, 1, 2, nil, reflect.TypeOf(&shardErasureCode{}), nil},
		{ECTypeShard, 1, 2, []interface{}{64}, reflect.TypeOf(&shardErasureCode{}), nil},
		{ECTypeShard, 1, 2, []interface{}{"standard"}, reflect.TypeOf(&shardErasureCode{}), errors.New("extra format error")},
	}
	for i, test := range tests {
		ec, err := New(test.ecType, test.minSectors, test.numSectors, test.extra...)
		if (err == nil) != (test.expectErr == nil) {
			t.Errorf("Test %d: expected error %v, got error %v", i, test.expectErr, err)
		}
		if err == nil && test.expectErr == nil && reflect.TypeOf(ec) != test.expectType {
			t.Errorf("Test %d: expected type %v, got type %v", i, test.expectType, reflect.TypeOf(ec))
		}
	}
}
