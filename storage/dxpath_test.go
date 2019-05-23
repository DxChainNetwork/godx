/*
 * Copyright 2019 DxChain, All rights reserved.
 * Use of this source code is governed by an Apache
 * License 2.0 that can be found in the LICENSE file.
 */

package storage

import "testing"

func TestNewDxPath(t *testing.T) {
	tests := []struct {
		s     string
		valid bool
	}{
		{"valid/siapath", true},
		{"\\some\\windows\\path", true}, // clean converts OS separators
		{"../../../directory/traversal", false},
		{"testpath", true},
		{"valid/siapath/../with/directory/traversal", false},
		{"/usr/bin/", true},
		{"validpath/test", true},
		{"..validpath/..test", true},
		{"./invalid/path", false},
		{".../path", true},
		{"valid./path", true},
		{"valid../path", true},
		{"valid/path./test", true},
		{"valid/path../test", true},
		{"test/path", true},
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
