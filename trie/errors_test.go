package trie

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapErrorNil(t *testing.T) {
	var input error
	ctx := "attached context"
	var want error
	if errorOut := wrapError(input, ctx); errorOut != want {
		t.Errorf("wrapError(%s) -> %s, want %s", input, errorOut, want)
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		errorIn error
		ctxIn   string
		wantCtx []string
	}{
		{
			errorIn: errors.New("msg"),
			ctxIn:   "newCtx",
			wantCtx: []string{"newCtx"},
		},
		{
			errorIn: &decodeError{
				what:  errors.New("msg"),
				stack: []string{},
			},
			ctxIn:   "newCtx",
			wantCtx: []string{"newCtx"},
		},
		{
			errorIn: &decodeError{
				what:  errors.New("msg"),
				stack: []string{"ctx1"},
			},
			ctxIn:   "newCtx",
			wantCtx: []string{"ctx1", "newCtx"},
		},
	}
	for _, test := range tests {
		errorOut := wrapError(test.errorIn, test.ctxIn)
		de, ok := errorOut.(*decodeError)
		if !ok {
			t.Errorf("wrapError(%+v) -> %+v, output should be of type decodeError", test.errorIn, errorOut)
		}
		if !strings.HasPrefix(de.Error(), test.errorIn.Error()) {
			t.Errorf("wrapError(%+v) \nGot error message [%s] \nexpect starts with [%s]", test.errorIn, de.Error(), test.errorIn.Error())
		}
		if len(de.stack) != len(test.wantCtx) {
			t.Errorf("wrapError(%+v) -> %s, error stack length error, expect %d, get %d", test.errorIn, de, len(test.wantCtx), len(de.stack))
		}
		for i, s := range test.wantCtx {
			if s != de.stack[i] {
				t.Errorf("wrapError(%+v), stack content different\ngot ctx[%d]: %s\nexpect ctx[i]: %+v", test.errorIn, i, de.stack[i], s)
			}
		}
	}
}
