// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package common

import (
	"testing"
)

func TestRandomBigIntRange(t *testing.T) {
	testingRange := []BigInt{
		BigInt0,
		NewBigInt(-200),
		NewBigInt(1000),
	}

	for _, data := range testingRange {
		val, err := RandomBigIntRange(data)
		if data.IsNeg() || data.IsEqual(BigInt0) {
			if err == nil {
				t.Errorf("negative or 0 range, expecte error")
			}
			continue
		}

		if val.Cmp(BigInt0) == -1 || val.Cmp(data) == 1 {
			t.Errorf("range from 0 - %v, %v is not expected", data, val)
		}
	}
}

func TestBigInt_IsNeg(t *testing.T) {
	tables := []struct {
		a      int64
		result bool
	}{
		{0, false},
		{1, false},
		{-1, true},
		{1000000000000, false},
		{-1000000000000, true},
	}

	for _, table := range tables {
		isNeg := NewBigInt(table.a).IsNeg()
		if isNeg != table.result {
			t.Errorf("input: %v, expected is negative: %t, got %t", table.a, table.result,
				isNeg)
		}
	}
}

func TestBigInt_IsEqual(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      BigInt
		result bool
	}{
		{BigInt1, BigInt1, true},
		{BigInt0, BigInt1, false},
		{NewBigInt(-1), NewBigInt(-1), true},
		{BigInt0, BigInt0, true},
	}

	for _, table := range tables {
		isEqual := table.a.IsEqual(table.b)
		if isEqual != table.result {
			t.Errorf("input %v, %v expected eqality %t, got %t", table.a, table.b, table.result,
				isEqual)
		}
	}
}

func TestBigInt_Add(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      BigInt
		result BigInt
	}{
		{NewBigInt(100000), NewBigInt(-100000), BigInt0},
		{NewBigInt(100000), NewBigInt(100000), NewBigInt(200000)},
		{NewBigInt(0), NewBigInt(-100000), NewBigInt(-100000)},
	}

	for _, table := range tables {
		val := table.a.Add(table.b)
		isEqual := val.Cmp(table.result)
		if isEqual != 0 {
			t.Errorf("input %v, %v. Expected sum %v, got sum %v", table.a, table.b, table.result,
				val)
		}
	}
}

func TestBigInt_Sub(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      BigInt
		result BigInt
	}{
		{NewBigInt(-100000), NewBigInt(0), NewBigInt(-100000)},
		{NewBigInt(100000), NewBigInt(100000), BigInt0},
		{NewBigInt(0), NewBigInt(-100000), NewBigInt(100000)},
	}

	for _, table := range tables {
		val := table.a.Sub(table.b)
		if !val.IsEqual(table.result) {
			t.Errorf("input %v, %v. Expected sum %v, got sum %v", table.a, table.b, table.result,
				val)
		}
	}
}

func TestBigInt_Mult(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      BigInt
		result BigInt
	}{
		{BigInt0, BigInt1, BigInt0},
		{BigInt1, NewBigInt(-1), NewBigInt(-1)},
		{NewBigInt(-1), NewBigInt(-1), BigInt1},
	}

	for _, table := range tables {
		val := table.a.Mult(table.b)
		if !val.IsEqual(table.result) {
			t.Errorf("input %v, %v. Expected product %v, got product %v", table.a, table.b, table.result,
				val)
		}
	}
}

func TestBigInt_MultInt(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      int64
		result BigInt
	}{
		{BigInt0, 10000, BigInt0},
		{BigInt1, 10000, NewBigInt(10000)},
		{BigInt1, -1, NewBigInt(-1)},
	}

	for _, table := range tables {
		val := table.a.MultInt(table.b)
		if !val.IsEqual(table.result) {
			t.Errorf("input %v, %v. Expected product %v, got product %v", table.a, table.b,
				table.result, val)
		}
	}
}

func TestBigInt_Div(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      BigInt
		result BigInt
	}{
		{BigInt0, BigInt1, BigInt0},
		{BigInt1, NewBigInt(-1), NewBigInt(-1)},
		{BigInt1, NewBigInt(-2), NewBigInt(0)},
		{NewBigInt(1000), NewBigInt(2), NewBigInt(500)},
	}

	for _, table := range tables {
		val := table.a.Div(table.b)
		if !val.IsEqual(table.result) {
			t.Errorf("input %v, %v. Expected quotient %v, got quotient %v", table.a,
				table.b, table.result, val)
		}
	}
}

func TestBigInt_DivUint64(t *testing.T) {
	tables := []struct {
		a      BigInt
		b      uint64
		result BigInt
	}{
		{BigInt0, 10000, BigInt0},
		{BigInt1, 10000, BigInt0},
		{NewBigInt(500), 2, NewBigInt(250)},
		{NewBigInt(500), 3, NewBigInt(166)},
	}

	for _, table := range tables {
		val := table.a.DivUint64(table.b)
		if !val.IsEqual(table.result) {
			t.Errorf("input %v, %v. Expected quotient %v, got quotient %v", table.a,
				table.b, table.result, val)
		}
	}
}

func TestBigInt_Cmp(t *testing.T) {
	tables := []struct {
		a      int64
		b      int64
		result int
	}{
		{-1, 0, -1},
		{0, 0, 0},
		{1, 0, 1},
	}

	for _, table := range tables {
		val := NewBigInt(table.a).Cmp(NewBigInt(table.b))
		if val != table.result {
			t.Errorf("error: comparison bettwen %v and %v should result %v instead of %v",
				table.a, table.b, table.result, val)
		}
	}
}

func TestCmp(t *testing.T) {
	tables := []struct {
		a      int64
		b      int64
		result int
	}{
		{200, 100, 1},
		{100, 200, -1},
		{100, 100, 0},
	}

	for _, table := range tables {
		val := NewBigInt(table.a).Cmp(NewBigInt(table.b))
		if val != table.result {
			t.Errorf("error comparing between %d and %d, expected %d, got %d",
				table.a, table.b, table.result, val)
		}
	}
}

func TestBigInt_DivWithFloatResult(t *testing.T) {
	tables := []struct {
		a      int64
		b      int64
		result float64
	}{
		{10000, 300, float64(10000) / float64(300)},
		{73846123, 321, float64(73846123) / float64(321)},
		{938381398213, 738321338211, float64(938381398213) / float64(738321338211)},
	}

	for _, table := range tables {
		got := NewBigInt(table.a).DivWithFloatResult(NewBigInt(table.b))
		if got != table.result {
			t.Errorf("Division result does not match. Expected %v, got %v", table.result, got)
		}
	}
}
