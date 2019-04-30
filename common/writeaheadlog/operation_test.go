package writeaheadlog

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func makeTestOps(num int) []Operation {
	var ops []Operation
	for i := 0; i != num; i++ {
		ops = append(ops, Operation{
			Name: fmt.Sprintf("Test%d", i),
			Data: bytes.Repeat([]byte{byte(i)}, 100*i),
		})
	}
	return ops
}

func TestOpsMarshalUnmarshal(t *testing.T) {
	ops := makeTestOps(10)
	data := marshalOps(ops)
	recovered, err := unmarshalOps(data)
	if err != nil {
		t.Fatal("Cannot unmarshal marshaled data")
	}
	for i, op := range ops {
		recover := recovered[i]
		if !reflect.DeepEqual(op, recover) {
			t.Errorf("Unmarshaled %v not expected\n\tExpect: %+v\n\tgot: %+v", op.Name, op, recover)
		}
	}
}

func TestOperation_verify(t *testing.T) {
	tests := []struct {
		op  Operation
		err error
	}{
		{
			op:  Operation{Name: "Normal", Data: []byte{1, 1, 1, 1, 1, 1}},
			err: nil,
		},
		{
			op:  Operation{Name: "", Data: []byte{1, 1, 1, 1, 1, 1}},
			err: errors.New("name cannot be empty"),
		},
		{
			op: Operation{
				Name: "This name is long, long enough that I can ever believe. The one writing this name must be a genius, who is talented, diligent, and handsome. I know that you cannot argue with that. Neither do I. This is the truth over the universe, born with the intellect of human beings. It is the truth that shine and provide like the sun",
				Data: []byte{1, 1, 1, 1, 1},
			},
			err: errors.New("name longer than 255 bytes not supported"),
		},
	}
	for i, test := range tests {
		err := test.op.verify()
		if (err == nil) != (test.err == nil) {
			t.Errorf("Unexpected error in test %d:\n\tExpected: [%v]\n\tGot: [%v]", i, test.err, err)
		}
	}
}

func BenchmarkMarshalUpdates(b *testing.B) {
	ops := make([]Operation, 100)
	for i := range ops {
		ops[i] = Operation{
			Name: "test",
			Data: randomBytes(1234),
		}
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalOps(ops)
	}
}

func BenchmarkUnmarshalUpdates(b *testing.B) {
	ops := make([]Operation, 100)
	for i := range ops {
		ops[i] = Operation{
			Name: "test",
			Data: randomBytes(1234),
		}
	}
	data := marshalOps(ops)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := unmarshalOps(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
