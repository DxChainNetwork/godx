/*
	Test Cases Covered All Public Functions
*/

package rlp

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestReadSizeWithRandomByteSlice(t *testing.T) {
	tables := []struct {
		b      []byte
		slen   byte
		result uint64
	}{
		{[]byte{0x44, 0x55}, 2, 17493},
		{[]byte{0xFF, 0xF4, 0x5F}, 3, 16774239},
		{[]byte{0xD8, 0xAD, 0xAF, 0xF1}, 4, 3635261425},
		{[]byte{0xD7, 0xBD, 0x6F, 0xC1, 0xD6}, 5, 926596186582},
		{[]byte{0xD7, 0xBD, 0x6F, 0xC1, 0xD6, 0xF7}, 6, 237208623765239},
	}

	for _, table := range tables {
		returnResult, err := readSize(table.b, table.slen)
		if err != nil {
			t.Errorf("%s", err)
		} else if table.result != returnResult {
			t.Errorf("input byte slice %x, input slen %d, got %d want %d",
				table.b, table.slen, returnResult, table.result)
		}
	}
}

func TestReadKindRawWithRlpEncodeListAndStringAndByte(t *testing.T) {
	tables := []struct {
		buf         []byte
		k           Kind
		tagsize     uint64
		contentsize uint64
	}{
		{[]byte{0x54}, Byte, 0, 1},
		{[]byte{0x82, 0x54, 0x56}, String, 1, 2},
		{[]byte{0xC1, 0x54}, List, 1, 1},
		{[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66},
			String, 2, 56},
	}

	for _, table := range tables {
		rk, rt, rc, err := readKind(table.buf)
		if err != nil {
			t.Errorf("%s \n", err)
		} else if table.k != rk {
			t.Errorf("input %x, got kind %d want kind %d", table.buf, rk, table.k)
		} else if table.tagsize != rt {
			t.Errorf("input %x, got tagsize %d want tagsize %d", table.buf, rt, table.tagsize)
		} else if table.contentsize != rc {
			t.Errorf("input %x, got contentsize %d want contentsize %d", table.buf, rc,
				table.contentsize)
		}
	}
}

func TestContentValuesWithRlpEncodeListAndStringAndByte(t *testing.T) {
	tables := []struct {
		b      []byte
		counts int
	}{
		{[]byte{0x54}, 1},
		{[]byte{0x82, 0x54, 0x56}, 1},
		{[]byte{0xC1, 0x54}, 1},
		{[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66}, 1},
	}

	for _, table := range tables {
		result, err := CountValues(table.b)
		if err != nil {
			t.Errorf("%s \n", err)
		} else if result != table.counts {
			t.Errorf("input %x, got %d, want %d", table.b, result, table.counts)
		}
	}
}

func TestListSizeRaw(t *testing.T) {
	tables := []struct {
		contentsize uint64
		listSize    uint64
	}{
		{3, 4},
		{55, 56},
		{56, 58},
	}

	for _, table := range tables {
		result := ListSize(table.contentsize)
		if result != table.listSize {
			t.Errorf("input content size %d got listsize %d want listsize %d",
				table.contentsize, result, table.listSize)
		}
	}
}

func TestSplitWithRlpEncodedByteAndSliceAndString(t *testing.T) {
	tables := []struct {
		b       []byte
		k       Kind
		content []byte
		rest    []byte
	}{
		{[]byte{0x54, 0x81}, Byte, []byte{0x54}, []byte{0x81}},
		{[]byte{0x82, 0x54, 0x56}, String, []byte{0x54, 0x56}, []byte{}},
		{[]byte{0xC1, 0x54, 0x56, 0x97}, List, []byte{0x54}, []byte{0x56, 0x97}},
		{[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66}, String, []byte{0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66}, []byte{}},
	}

	for _, table := range tables {
		rk, rc, rr, err := Split(table.b)
		if err != nil {
			t.Errorf("%s \n", err)
		} else if rk != table.k {
			t.Errorf("input %x, got kind %d, want kind %d \n", table.b, rk, table.k)
		} else if bytes.Compare(rc, table.content) != 0 {
			t.Errorf("input %x, got content %x, want content %x ", table.b, rc, table.content)
		} else if bytes.Compare(rr, table.rest) != 0 {
			t.Errorf("input %x, got rest %x, want rest %x", table.b, rr, table.rest)
		}
	}
}

// testing if the proper error message showed up
var listTemp = make([]byte, 58)

func TestSplitStringWithRlpEncodedStringAndList(t *testing.T) {
	listTemp[0] = 0xF8
	listTemp[1] = 0x38
	for i := 2; i < 58; i++ {
		listTemp[i] = 0x7F
	}

	tables := []struct {
		b   []byte
		err error
	}{
		{[]byte{0xC2, 0x81, 0x4F}, ErrExpectedString},
		{listTemp, ErrExpectedString},
		{[]byte{0x7F}, nil},
		{[]byte{0x82, 0x66, 0x66}, nil},
	}

	for _, table := range tables {
		_, _, err := SplitString(table.b)
		if err != table.err {
			t.Errorf("input %x, got error %s, expected error %s", table.b, err, table.err)
		}
	}
}

// testing if the proper error message showed up
var stringTemp = make([]byte, 58)

func TestSplitListWithRlpEncodedStringAndList(t *testing.T) {
	stringTemp[0] = 0xB8
	stringTemp[1] = 0x38
	for i := 2; i < 58; i++ {
		stringTemp[i] = 0x6F
	}
	tables := []struct {
		b   []byte
		err error
	}{
		{[]byte{0xC2, 0x81, 0x4F}, nil},
		{listTemp, nil},
		{stringTemp, ErrExpectedList},
		{[]byte{0x7F}, ErrExpectedList},
		{[]byte{0x82, 0x66, 0x66}, ErrExpectedList},
	}

	for _, table := range tables {
		_, _, err := SplitList(table.b)
		if err != table.err {
			t.Errorf("input %x, got error: %s, expected error: %s", table.b, err, table.err)
		}
	}
}

/*
     _____                                _ _   _               _______        _
    / ____|                              (_) | (_)             |__   __|      | |
   | |     ___  _ __ ___  _ __   ___  ___ _| |_ _  ___  _ __      | | ___  ___| |_
   | |    / _ \| '_ ` _ \| '_ \ / _ \/ __| | __| |/ _ \| '_ \     | |/ _ \/ __| __|
   | |___| (_) | | | | | | |_) | (_) \__ \ | |_| | (_) | | | |    | |  __/\__ \ |_
    \_____\___/|_| |_| |_| .__/ \___/|___/_|\__|_|\___/|_| |_|    |_|\___||___/\__|
                         | |
                         |_|
*/

func TestCountValues(t *testing.T) {
	tests := []struct {
		input string // note: spaces in input are stripped by unhex
		count int
		err   error
	}{
		// simple cases
		{"", 0, nil},
		{"00", 1, nil},
		{"80", 1, nil},
		{"C0", 1, nil},
		{"01 02 03", 3, nil},
		{"01 C406070809 02", 3, nil},
		{"820101 820202 8403030303 04", 4, nil},

		// size errors
		{"8142", 0, ErrCanonSize},
		{"01 01 8142", 0, ErrCanonSize},
		{"02 84020202", 0, ErrValueTooLarge},

		{
			input: "A12000BF49F440A1CD0527E4D06E2765654C0F56452257516D793A9B8D604DCFDF2AB853F851808D10000000000000000000000000A056E81F171BCC55A6FF8345E692C0F86E5B48E01B996CADC001622FB5E363B421A0C5D2460186F7233C927E7DB2DCC703C0E500B653CA82273B7BFAD8045D85A470",
			count: 2,
		},
	}
	for i, test := range tests {
		count, err := CountValues(unhex(test.input))
		if count != test.count {
			t.Errorf("test %d: count mismatch, got %d want %d\ninput: %s", i, count, test.count, test.input)
		}
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("test %d: err mismatch, got %q want %q\ninput: %s", i, err, test.err, test.input)
		}
	}
}

func TestSplitTypes(t *testing.T) {
	if _, _, err := SplitString(unhex("C100")); err != ErrExpectedString {
		t.Errorf("SplitString returned %q, want %q", err, ErrExpectedString)
	}
	if _, _, err := SplitList(unhex("01")); err != ErrExpectedList {
		t.Errorf("SplitString returned %q, want %q", err, ErrExpectedList)
	}
	if _, _, err := SplitList(unhex("81FF")); err != ErrExpectedList {
		t.Errorf("SplitString returned %q, want %q", err, ErrExpectedList)
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		input     string
		kind      Kind
		val, rest string
		err       error
	}{
		{input: "01FFFF", kind: Byte, val: "01", rest: "FFFF"},
		{input: "80FFFF", kind: String, val: "", rest: "FFFF"},
		{input: "C3010203", kind: List, val: "010203"},

		// errors
		{input: "", err: io.ErrUnexpectedEOF},

		{input: "8141", err: ErrCanonSize, rest: "8141"},
		{input: "B800", err: ErrCanonSize, rest: "B800"},
		{input: "B802FFFF", err: ErrCanonSize, rest: "B802FFFF"},
		{input: "B90000", err: ErrCanonSize, rest: "B90000"},
		{input: "B90055", err: ErrCanonSize, rest: "B90055"},
		{input: "BA0002FFFF", err: ErrCanonSize, rest: "BA0002FFFF"},
		{input: "F800", err: ErrCanonSize, rest: "F800"},
		{input: "F90000", err: ErrCanonSize, rest: "F90000"},
		{input: "F90055", err: ErrCanonSize, rest: "F90055"},
		{input: "FA0002FFFF", err: ErrCanonSize, rest: "FA0002FFFF"},

		{input: "81", err: ErrValueTooLarge, rest: "81"},
		{input: "8501010101", err: ErrValueTooLarge, rest: "8501010101"},
		{input: "C60607080902", err: ErrValueTooLarge, rest: "C60607080902"},

		// size check overflow
		{input: "BFFFFFFFFFFFFFFFFF", err: ErrValueTooLarge, rest: "BFFFFFFFFFFFFFFFFF"},
		{input: "FFFFFFFFFFFFFFFFFF", err: ErrValueTooLarge, rest: "FFFFFFFFFFFFFFFFFF"},

		{
			input: "B838FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
			err:   ErrValueTooLarge,
			rest:  "B838FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
		},
		{
			input: "F838FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
			err:   ErrValueTooLarge,
			rest:  "F838FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
		},

		// a few bigger values, just for kicks
		{
			input: "F839FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
			kind:  List,
			val:   "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
			rest:  "",
		},
		{
			input: "F90211A060EF29F20CC1007AE6E9530AEE16F4B31F8F1769A2D1264EC995C6D1241868D6A07C62AB8AC9838F5F5877B20BB37B387BC2106E97A3D52172CBEDB5EE17C36008A00EAB6B7324AADC0F6047C6AFC8229F09F7CF451B51D67C8DFB08D49BA8C3C626A04453343B2F3A6E42FCF87948F88AF7C8FC16D0C2735CBA7F026836239AB2C15FA024635C7291C882CE4C0763760C1A362DFC3FFCD802A55722236DE058D74202ACA0A220C808DE10F55E40AB25255201CFF009EA181D3906638E944EE2BF34049984A08D325AB26796F1CCB470F69C0F842501DC35D368A0C2575B2D243CFD1E8AB0FDA0B5298FF60DA5069463D610513C9F04F24051348391A143AFFAB7197DFACDEA72A02D2A7058A4463F8FB69378369E11EF33AE3252E2DB86CB545B36D3C26DDECE5AA0888F97BCA8E0BD83DC5B3B91CFF5FAF2F66F9501010682D67EF4A3B4E66115FBA0E8175A60C93BE9ED02921958F0EA55DA0FB5E4802AF5846147BAD92BC2D8AF26A08B3376FF433F3A4250FA64B7F804004CAC5807877D91C4427BD1CD05CF912ED8A09B32EF0F03BD13C37FF950C0CCCEFCCDD6669F2E7F2AA5CB859928E84E29763EA09BBA5E46610C8C8B1F8E921E5691BF8C7E40D75825D5EA3217AA9C3A8A355F39A0EEB95BC78251CCCEC54A97F19755C4A59A293544EEE6119AFA50531211E53C4FA00B6E86FE150BF4A9E0FEEE9C90F5465E617A861BB5E357F942881EE762212E2580",
			kind:  List,
			val:   "A060EF29F20CC1007AE6E9530AEE16F4B31F8F1769A2D1264EC995C6D1241868D6A07C62AB8AC9838F5F5877B20BB37B387BC2106E97A3D52172CBEDB5EE17C36008A00EAB6B7324AADC0F6047C6AFC8229F09F7CF451B51D67C8DFB08D49BA8C3C626A04453343B2F3A6E42FCF87948F88AF7C8FC16D0C2735CBA7F026836239AB2C15FA024635C7291C882CE4C0763760C1A362DFC3FFCD802A55722236DE058D74202ACA0A220C808DE10F55E40AB25255201CFF009EA181D3906638E944EE2BF34049984A08D325AB26796F1CCB470F69C0F842501DC35D368A0C2575B2D243CFD1E8AB0FDA0B5298FF60DA5069463D610513C9F04F24051348391A143AFFAB7197DFACDEA72A02D2A7058A4463F8FB69378369E11EF33AE3252E2DB86CB545B36D3C26DDECE5AA0888F97BCA8E0BD83DC5B3B91CFF5FAF2F66F9501010682D67EF4A3B4E66115FBA0E8175A60C93BE9ED02921958F0EA55DA0FB5E4802AF5846147BAD92BC2D8AF26A08B3376FF433F3A4250FA64B7F804004CAC5807877D91C4427BD1CD05CF912ED8A09B32EF0F03BD13C37FF950C0CCCEFCCDD6669F2E7F2AA5CB859928E84E29763EA09BBA5E46610C8C8B1F8E921E5691BF8C7E40D75825D5EA3217AA9C3A8A355F39A0EEB95BC78251CCCEC54A97F19755C4A59A293544EEE6119AFA50531211E53C4FA00B6E86FE150BF4A9E0FEEE9C90F5465E617A861BB5E357F942881EE762212E2580",
			rest:  "",
		},
		{
			input: "F877A12000BF49F440A1CD0527E4D06E2765654C0F56452257516D793A9B8D604DCFDF2AB853F851808D10000000000000000000000000A056E81F171BCC55A6FF8345E692C0F86E5B48E01B996CADC001622FB5E363B421A0C5D2460186F7233C927E7DB2DCC703C0E500B653CA82273B7BFAD8045D85A470",
			kind:  List,
			val:   "A12000BF49F440A1CD0527E4D06E2765654C0F56452257516D793A9B8D604DCFDF2AB853F851808D10000000000000000000000000A056E81F171BCC55A6FF8345E692C0F86E5B48E01B996CADC001622FB5E363B421A0C5D2460186F7233C927E7DB2DCC703C0E500B653CA82273B7BFAD8045D85A470",
			rest:  "",
		},
	}

	for i, test := range tests {
		kind, val, rest, err := Split(unhex(test.input))
		if kind != test.kind {
			t.Errorf("test %d: kind mismatch: got %v, want %v", i, kind, test.kind)
		}
		if !bytes.Equal(val, unhex(test.val)) {
			t.Errorf("test %d: val mismatch: got %x, want %s", i, val, test.val)
		}
		if !bytes.Equal(rest, unhex(test.rest)) {
			t.Errorf("test %d: rest mismatch: got %x, want %s", i, rest, test.rest)
		}
		if err != test.err {
			t.Errorf("test %d: error mismatch: got %q, want %q", i, err, test.err)
		}
	}
}

func TestReadSize(t *testing.T) {
	tests := []struct {
		input string
		slen  byte
		size  uint64
		err   error
	}{
		{input: "", slen: 1, err: io.ErrUnexpectedEOF},
		{input: "FF", slen: 2, err: io.ErrUnexpectedEOF},
		{input: "00", slen: 1, err: ErrCanonSize},
		{input: "36", slen: 1, err: ErrCanonSize},
		{input: "37", slen: 1, err: ErrCanonSize},
		{input: "38", slen: 1, size: 0x38},
		{input: "FF", slen: 1, size: 0xFF},
		{input: "FFFF", slen: 2, size: 0xFFFF},
		{input: "FFFFFF", slen: 3, size: 0xFFFFFF},
		{input: "FFFFFFFF", slen: 4, size: 0xFFFFFFFF},
		{input: "FFFFFFFFFF", slen: 5, size: 0xFFFFFFFFFF},
		{input: "FFFFFFFFFFFF", slen: 6, size: 0xFFFFFFFFFFFF},
		{input: "FFFFFFFFFFFFFF", slen: 7, size: 0xFFFFFFFFFFFFFF},
		{input: "FFFFFFFFFFFFFFFF", slen: 8, size: 0xFFFFFFFFFFFFFFFF},
		{input: "0102", slen: 2, size: 0x0102},
		{input: "010203", slen: 3, size: 0x010203},
		{input: "01020304", slen: 4, size: 0x01020304},
		{input: "0102030405", slen: 5, size: 0x0102030405},
		{input: "010203040506", slen: 6, size: 0x010203040506},
		{input: "01020304050607", slen: 7, size: 0x01020304050607},
		{input: "0102030405060708", slen: 8, size: 0x0102030405060708},
	}

	for _, test := range tests {
		size, err := readSize(unhex(test.input), test.slen)
		if err != test.err {
			t.Errorf("readSize(%s, %d): error mismatch: got %q, want %q", test.input, test.slen, err, test.err)
			continue
		}
		if size != test.size {
			t.Errorf("readSize(%s, %d): size mismatch: got %#x, want %#x", test.input, test.slen, size, test.size)
		}
	}
}
