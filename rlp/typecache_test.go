/*
	No Public Functions In This File

	Private Functions Not Tested:
		* genTypeInfo -- already tested make writer/make decoder, nothing different because the only job of genTypeInfo is to call makeDecoder and makeWriter
						 store and return writer and decoder back
		* cachedTypeInfo1 -- the main job of cachedTypeInfo1 is to call genTypeInfo with some multiprocessing handling

		* cachedTypeInfo -- the main job of cahcedTypeInfo is to call cachedTypeInfo1 with some multiprocessing handling
*/

package rlp

import (
	"reflect"
	"testing"
)

var u7 uintptr = 0x958321832

func TestIsUintWithDifferentSizeUintData(t *testing.T) {
	tables := []struct {
		val    interface{}
		result bool
	}{
		{u0, true},
		{u1, true},
		{u2, true},
		{u3, true},
		{u4, true},
		{u7, true},
	}

	for _, table := range tables {
		rkind := reflect.TypeOf(table.val).Kind()
		returnResult := isUint(rkind)
		if returnResult != table.result {
			t.Errorf("input %d, got %t want %t", table.val, returnResult, table.result)
		}
	}
}

var nu0 = -100
var nu1 = "test"
var nu2 = 'c'
var nu3 = []uint{10, 20, 30}

func TestIsUintWithNonUintData(t *testing.T) {
	tables := []struct {
		val    interface{}
		result bool
	}{
		{nu0, false},
		{nu1, false},
		{nu2, false},
		{nu3, false},
	}

	for _, table := range tables {
		rkind := reflect.TypeOf(table.val).Kind()
		returnResult := isUint(rkind)
		if returnResult != table.result {
			t.Errorf("input %d, got %t want %t", table.val, returnResult, table.result)
		}
	}
}

func TestParseStructTagWithValidTags(t *testing.T) {
	tables := []struct {
		p  interface{}
		fi int
		ts tags
	}{
		{p1, 0, tags{false, false, false}},
		{p1, 1, tags{false, false, false}},
		{p1, 2, tags{false, true, false}},
		{p2, 0, tags{false, false, true}},
		{p2, 1, tags{false, false, false}},
		{p2, 2, tags{false, false, false}},
	}

	for _, table := range tables {
		rtyp := reflect.TypeOf(table.p)
		resultTags, err := parseStructTag(rtyp, table.fi)
		if err != nil {
			t.Errorf("%s", err)
		} else if resultTags != table.ts {
			t.Errorf("input %v, got %v want %v", table.p, resultTags, table.ts)
		}
	}
}

func TestStructFieldsAlongWithWriterFunctionReturn(t *testing.T) {
	tables := []struct {
		p          interface{}
		fi         int
		writerName string
	}{
		{p1, 0, "github.com/DxChainNetwork/godx/rlp.writeString"},
		{p1, 1, "github.com/DxChainNetwork/godx/rlp.writeUint"},
		{p1, 2, "github.com/DxChainNetwork/godx/rlp.makeSliceWriter.func1"},
		{p2, 0, "github.com/DxChainNetwork/godx/rlp.writeUint"},
		{p2, 1, "github.com/DxChainNetwork/godx/rlp.makeSliceWriter.func1"},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.p)
		rtype := rval.Type()
		fields, err := structFields(rtype)
		funcName := getFunctionName(fields[table.fi].info.writer)
		if err != nil {
			t.Errorf("%s", err)
		} else if funcName != table.writerName {
			t.Errorf("input %v, got %s want %s", table.p, funcName, table.writerName)
		}
	}
}
