package writeaheadlog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

// Operation is a single operation defined by a name and data
type Operation struct {
	Name string
	Data []byte
}

// verify check the name field of the operation. If check failed, return an error
//  * name should not be empty
//  * name should be longer than 255 bytes (length could reside in 1 byte)
func (op *Operation) verify() error {
	if len(op.Name) == 0 {
		return errors.New("name cannot be empty")
	}
	if len(op.Name) > math.MaxUint8 {
		return errors.New("name longer than 255 bytes not supported")
	}
	return nil
}

// marshalOps marshal the ops to byte slice
func marshalOps(ops []Operation) []byte {
	// preallocate buffer of appropriate size
	var size int
	for _, op := range ops {
		size += 1 + len(op.Name)
		size += 8 + len(op.Data)
	}
	buf := make([]byte, size)

	var n int
	for _, op := range ops {
		// u.Name
		buf[n] = byte(len(op.Name))
		n++
		n += copy(buf[n:], op.Name)
		// u.Instructions
		binary.LittleEndian.PutUint64(buf[n:], uint64(len(op.Data)))
		n += 8
		n += copy(buf[n:], op.Data)
	}
	return buf
}

// unmarshalUpdates unmarshals the Operations of a transaction
func unmarshalOps(data []byte) ([]Operation, error) {
	// helper function for reading length-prefixed data
	buf := bytes.NewBuffer(data)

	var ops []Operation
	for {
		if buf.Len() == 0 {
			break
		}

		name, ok := nextPrefix(1, buf)
		if !ok {
			return nil, errors.New("failed to unmarshal name")
		} else if len(name) == 0 {
			// end of updates
			break
		}

		data, ok := nextPrefix(8, buf)
		if !ok {
			return nil, errors.New("failed to unmarshal instructions")
		}

		ops = append(ops, Operation{
			Name: string(name),
			Data: data,
		})
	}

	return ops, nil
}

// nextPrefix is a helper function that reads the next prefix of prefixLen and
// prefixed data from a buffer and returns the data and a bool to indicate
// success.
func nextPrefix(prefixLen int, buf *bytes.Buffer) ([]byte, bool) {
	if buf.Len() < prefixLen {
		// missing length prefix
		return nil, false
	}
	var l int
	switch prefixLen {
	case 8:
		l = int(binary.LittleEndian.Uint64(buf.Next(prefixLen)))
	case 4:
		l = int(binary.LittleEndian.Uint32(buf.Next(prefixLen)))
	case 2:
		l = int(binary.LittleEndian.Uint16(buf.Next(prefixLen)))
	case 1:
		l = int(buf.Next(prefixLen)[0])
	default:
		return nil, false
	}
	if l < 0 || l > buf.Len() {
		// invalid length prefix
		return nil, false
	}
	return buf.Next(l), true
}
