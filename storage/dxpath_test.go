// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"testing"

	"github.com/DxChainNetwork/godx/rlp"
)

func TestNewDxPath(t *testing.T) {
	tests := []struct {
		s     string
		valid bool
	}{
		{"valid/dxpath", true},
		{"\\some\\windows\\Path", true}, // clean converts OS separators
		{"../../../directory/traversal", false},
		{"testpath", true},
		{"valid/dxpath/../with/directory/traversal", false},
		{"/usr/bin/", true},
		{"validpath/test", true},
		{"..validpath/..test", true},
		{"./invalid/Path", false},
		{".../Path", true},
		{"valid./Path", true},
		{"valid../Path", true},
		{"valid/Path./test", true},
		{"valid/Path../test", true},
		{"test/Path", true},
		{"/leading/slash", true}, // clean will trim leading slashes so this is a valid input
		{"foo/./bar", false},
		{"", false},
		{".dxdir/contain/dxdir", false},
		{"have/.dxdir/in/middle", false},
		{"blank/end/", true}, // clean will trim trailing slashes so this is a valid input
		{"double//dash", false},
		{"../", false},
		{"./", false},
		{".", false},
	}
	for _, test := range tests {
		_, err := NewDxPath(test.s)
		if err != nil && test.valid {
			t.Fatal("valid DxPath failed on:", test.s)
		}
		if err == nil && !test.valid {
			t.Fatal("invalid DxPath succeeded on:", test.s)
		}
	}
}

func TestDxPath_Parent(t *testing.T) {
	tests := []struct {
		s      string
		expect string
		err    error
	}{
		{"valid/dxpath.go", "valid", nil},
		{"", "", ErrAlreadyRoot},
		{"root", "", nil},
		{"valid../long/long/long/dir", "valid../long/long/long", nil},
	}
	for i, test := range tests {
		var dp DxPath
		if test.s == "" {
			dp = RootDxPath()
		} else {
			var err error
			dp, err = NewDxPath(test.s)
			if err != nil {
				t.Fatal(err)
			}
		}
		parent, err := dp.Parent()
		if test.err != err {
			t.Errorf("test %d: expect error %v, got %v", i, test.err, err)
			continue
		}
		var expect DxPath
		if test.expect == "" {
			expect = RootDxPath()
		} else {
			expect, err = NewDxPath(test.expect)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err == nil && !parent.Equals(expect) {
			t.Errorf("test %d: expect %v, got %v", i, expect, parent)
		}
	}
}

func TestSysPath_Join(t *testing.T) {
	tests := []struct {
		sp     SysPath
		dp     DxPath
		extra  []string
		expect SysPath
	}{
		{"/usr/bin/data", DxPath{"test"}, []string{".dxdir"}, "/usr/bin/data/test/.dxdir"},
		{"", RootDxPath(), []string{""}, ""},
		{"", DxPath{"testfile"}, []string{".dxdir"}, "testfile/.dxdir"},
	}
	for i, test := range tests {
		got := test.sp.Join(test.dp, test.extra...)
		if got != test.expect {
			t.Errorf("test %d: Expect %v, Got %v", i, test.expect, got)
		}
	}
}

func TestDxPath_EncodeRLP_DecodeRLP(t *testing.T) {
	tests := []struct {
		s string
	}{
		{"valid/dxpath"},
		{"\\some\\windows\\Path"},
		{"testpath"},
		{"/usr/bin/"},
		{"validpath/test"},
		{"..validpath/..test"},
		{".../Path"},
		{"valid./Path"},
		{"valid../Path"},
		{"valid/Path./test"},
		{"valid/Path../test"},
		{"test/Path"},
		{"/leading/slash"},
		{"blank/end/"},
	}
	for _, test := range tests {
		dp, err := NewDxPath(test.s)
		if err != nil {
			t.Fatal(err)
		}
		b, err := rlp.EncodeToBytes(dp)
		if err != nil {
			t.Fatal(err)
		}
		var recovered DxPath
		if err := rlp.DecodeBytes(b, &recovered); err != nil {
			t.Fatal(err)
		}
		if !recovered.Equals(dp) {
			t.Errorf("not equal. Expect %v, got %v", test.s, recovered.Path)
		}
	}
}
