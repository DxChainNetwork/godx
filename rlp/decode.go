/*

Two Main Concepts:
* Information On Encoded Data -> stores in s.r
* Information On Storage -> the decoding data type defined by user

NOTE:
* Data decoding type is decided by users
* The passed in data (val interface{}) will all be pointers, and the decoded value will be stored in the value pointed by the pointer
* The type referenced in makeDecoder is val's underlying data type
* The val referenced in each type decoder is underlying data val

Storage Preparation, type of the storage is provided by user. If the storage has some flaws,
it is codes' job to fix the storage

Why the receiver for the decoder/encoder interface must be pointer?
The DecodeRLP / EncodeRLP must be able to change the values of the receiver

Removed Function:
	* func NewListStream(r io.Reader, len uint64) *Stream
		-- NOT USED BY ANY PLACE OTHER THAN TESTCASE

*/

package rlp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
)

type Kind int

// data kinds are representing with integers 0, 1, 2 (Byte, String, List)
const (
	Byte   Kind = iota // 0
	String             // 1
	List               // 2
)

var (
	// defines bigInt type
	bigInt = reflect.TypeOf(big.Int{})

	// defines decoder interface type
	decoderInterface = reflect.TypeOf(new(Decoder)).Elem()

	// EOL is not an actual error, it just indicates the end of a list is reached during streaming
	EOL = errors.New("rlp: end of list")

	// Actual Errors
	ErrExpectedString = errors.New("rlp: expected String or Byte")
	ErrExpectedList   = errors.New("rlp: expected List")
	// when data contains leading zero bytes
	ErrCanonInt         = errors.New("rlp: non-canonical integer format")
	ErrCanonSize        = errors.New("rlp: non-canonical size information")
	ErrElemTooLarge     = errors.New("rlp: element is larger than containing list")
	ErrValueTooLarge    = errors.New("rlp: value size exceeds available input length")
	ErrMoreThanOneValue = errors.New("rlp: input contains more than one value")

	// internal errors
	errNotInList     = errors.New("rlp: call of ListEnd outside of any list")
	errNotAtEOL      = errors.New("rlp: call of ListEnd not positioned at EOL")
	errUintOverflow  = errors.New("rlp: uint overflow")
	errNoPointer     = errors.New("rlp: interface given to Decode must be a pointer")
	errDecodeIntoNil = errors.New("rlp: pointer given to Decode must not be nil")
)

// Decoder interface, allows user to implement custom decoding rules or need to decode into private fields
// through DecodeRLP interface
type Decoder interface {
	DecodeRLP(*Stream) error
}

// Any data implements io.Reader or io.ByteReader interface also implements ByteReader Interface
type ByteReader interface {
	// Reader pops out number of data based on the size of the byte slice passed in, until EOF
	io.Reader
	// ByteReader pops out only one byte data each time until EOF
	io.ByteReader
}

type listpos struct {
	pos, size uint64
}

// core data structure
type Stream struct {
	r ByteReader // stores RLP encoded data. ByteReader type
	// WHY USE READER? logic is straightforward, like a queue, read one, delete one
	// WHY USE TWO DIFFERENT KINDS OF READER? read by bytes, or read multiple bytes at a time
	remaining uint64 // number of bytes remaining to be read from r.
	// Purpose: tracking the number of bytes left in the reader
	limited bool // main purpose: error checking. If there is a limit, then there is a relationship between size and remaining
	// example: if you want to read a bunch of bytes from the reader. what if the number you are tyring to
	// read is greater than the number of bytes left in the reader
	// NOTE: the only time it is set to be false is when the passed in reader is not string.Reade or Byte.Reader
	uintbuf []byte // serve as a temp buffer that used to store the data length (RULE 3, 4). For uint, it will hold its' whole value
	// Example: B810.... where B8 is the string header, 10 is the hex representation of the data content length
	kind Kind // kind of encoded data, will be found out based on the first byte
	// there is only three options: byte, string, list
	size uint64 // the length of encoded data content (in bytes)
	// for example: B810, the length of encoded data content is 16 bytes
	byteval byte // if byteval != 0, it indicates that the encoded value is single byte
	// the decoded value will be stored in byteval if the encoded value is a single byte (RULE #1)
	// (smaller than 0x80)
	kinderr error // errors when trying to get the kind of the encoded data
	// will be stored and formatted
	stack []listpos
}

// data structure for decoding errors
type decodeError struct {
	msg string
	typ reflect.Type
	ctx []string
}

// format decoding errors
func (err *decodeError) Error() string {
	ctx := ""
	if len(err.ctx) > 0 {
		ctx = ", decoding into "
		for i := len(err.ctx) - 1; i >= 0; i-- {
			ctx += err.ctx[i]
		}
	}
	return fmt.Sprintf("rlp: %s for %v%s", err.msg, err.typ, ctx)
}

// passed in io.Reader where the encoded data is stored in
// val interface -> a pointer pointed to the form of data you want to decoded as
// for example, a specific structure, uint, list, etc.
// metaphor: form of storage
func Decode(r io.Reader, val interface{}) error {
	return NewStream(r, 0).Decode(val)
}

// similar to decoder
// but instead of pass in a reader, pass in the encoded data directly
// the reader will automatically initialized as bytes reader
func DecodeBytes(b []byte, val interface{}) error {
	// initialize byte reader
	r := bytes.NewReader(b)
	if err := NewStream(r, uint64(len(b))).Decode(val); err != nil {
		return err
	}
	if r.Len() > 0 {
		return ErrMoreThanOneValue
	}
	return nil
}

// declare and initialize a stream pointer data, and return it
func NewStream(r io.Reader, inputLimit uint64) *Stream {
	s := new(Stream)
	s.Reset(r, inputLimit)
	return s
}

// initialize stream. Passing in the RLP encoded data to s.r and initialize other aspects
func (s *Stream) Reset(r io.Reader, inputLimit uint64) {
	if inputLimit > 0 {
		s.remaining = inputLimit
		s.limited = true
	} else {
		// based on the type of the reader, set up the remaining and limited fields
		// check if r is bytes reader or string reader
		switch br := r.(type) {
		case *bytes.Reader:
			s.remaining = uint64(br.Len())
			s.limited = true
		case *strings.Reader:
			s.remaining = uint64(br.Len())
			s.limited = true
		default:
			s.limited = false
		}
	}

	// Check if the reader implement ByteReader Interface
	// NOTE: for a variable that implements ByteReader, it must implements both Reader and ByteReader
	bufr, ok := r.(ByteReader)
	if !ok {
		// Since the data passed in implemented io.Reader interface, if it does not implement ByteReader interface
		// it means the data does not implement io.ByteReader interface. Therefore, by using bufio.NewReader, the returned
		// reader will implement both interfaces
		bufr = bufio.NewReader(r)
	}
	// assign value to stream.r, which is RLP encoded data
	s.r = bufr

	// reset the decoding context
	s.stack = s.stack[:0]
	s.size = 0
	s.kind = -1
	s.kinderr = nil

	// allocates 8 byte uint buffer
	if s.uintbuf == nil {
		s.uintbuf = make([]byte, 8)
	}
}

// decoded data will be stored into the value pointed by val
func (s *Stream) Decode(val interface{}) error {
	// if val does not pointed to any address, there is no place to store the decoded data
	if val == nil {
		return errDecodeIntoNil
	}

	// getting the value and the type of val
	rval := reflect.ValueOf(val)
	rtyp := rval.Type()

	// the passed in val must be a pointer
	if rtyp.Kind() != reflect.Ptr {
		return errNoPointer
	}

	// checked again if passed in val is pointed to nil
	if rval.IsNil() {
		return errDecodeIntoNil
	}

	// get the decoder based on the data type pointed by the val
	info, err := cachedTypeInfo(rtyp.Elem(), tags{})
	if err != nil {
		return err
	}
	// passed in stream as well as the value of data pointed by val that will be used to store the decoded data
	err = info.decoder(s, rval.Elem())

	// check if the err is type *decodeError and the length of ctx is greater than 0
	if decErr, ok := err.(*decodeError); ok && len(decErr.ctx) > 0 {
		decErr.ctx = append(decErr.ctx, fmt.Sprint("(", rtyp.Elem(), ")"))
	}
	return err
}

// function that makes decoders based on the type of the storage
func makeDecoder(typ reflect.Type, tags tags) (dec decoder, err error) {
	kind := typ.Kind()
	switch {
	case typ == rawValueType:
		return decodeRawValue, nil
	// for data that implemented DecodeRLP method (pointer receiver)
	case typ.Implements(decoderInterface):
		return decodeDecoder, nil
	// pointer type of the variable implements the decoder interface (pointer receiver)
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(decoderInterface):
		return decodeDecoderNoPtr, nil
	// if the type is *bigInt
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		return decodeBigInt, nil
	case typ.AssignableTo(bigInt):
		return decodeBigIntNoPtr, nil
	case isUint(kind):
		return decodeUint, nil
	case kind == reflect.Bool:
		return decodeBool, nil
	case kind == reflect.String:
		return decodeString, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return makeListDecoder(typ, tags)
	case kind == reflect.Struct:
		return makeStructDecoder(typ)
	case kind == reflect.Ptr:
		if tags.nilOK {
			return makeOptionalPtrDecoder(typ)
		}
		return makePtrDecoder(typ)
	case kind == reflect.Interface:
		return decodeInterface, nil
	default:
		return nil, fmt.Errorf("rlp: type %v is not RLP-serializable", typ)
	}
}

// decoder used decode value with type RawValue
func decodeRawValue(s *Stream, val reflect.Value) error {
	// r will be byte slice with decoded values
	r, err := s.Raw()
	if err != nil {
		return err
	}

	// sets val's underlying value (value that val pointed to) to r
	val.SetBytes(r)
	return nil
}

// decoder used to decode data that implements the decoder interface (double pointer)
func decodeDecoder(s *Stream, val reflect.Value) error {
	if val.Kind() == reflect.Ptr && val.IsNil() {
		// set the value to the pointer pointed to the 0 represented by the data type
		val.Set(reflect.New(val.Type().Elem()))
	}
	// transfer the reflect type back to Decoder interface type, and call DecodeRLP method
	return val.Interface().(Decoder).DecodeRLP(s)
}

// this decoder is used for non-pointer values of types that implement
// the Decoder interface using a pointer receiver
func decodeDecoderNoPtr(s *Stream, val reflect.Value) error {
	return val.Addr().Interface().(Decoder).DecodeRLP(s)
}

// decoder used to decode data with type *BigInt (Note: double pointer)
func decodeBigInt(s *Stream, val reflect.Value) error {
	// get the content of the string
	b, err := s.Bytes()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}

	// assign i to *big.Int type (its' original type instead of the reflect.val type)
	i := val.Interface().(*big.Int)

	// this means the storage prepared to store the data has some flaws, it pointed nil
	// therefore, we need to fix this storage, and make this storage be able to store the data
	if i == nil {
		// allocated space and let i pointed to it
		i = new(big.Int)
		// pass the address pointed by i (ValuOf(i)) to val (data synchronization)
		val.Set(reflect.ValueOf(i))
	}

	// no leading 0s
	if len(b) > 0 && b[0] == 0 {
		return wrapStreamError(ErrCanonInt, val.Type())
	}

	// assigning values
	i.SetBytes(b)
	return nil
}

// decoder for bigInt type data
func decodeBigIntNoPtr(s *Stream, val reflect.Value) error {
	return decodeBigInt(s, val.Addr())
}

// decoder for Uint kind data
// NOTE: uint, uint32, uint8, uint64, uintptr are all considered as uint kind data
// the definition can be found in isUint function
func decodeUint(s *Stream, val reflect.Value) error {
	// for uint, the default size is 64 bits
	typ := val.Type()

	// typ.Bits: getting the size of uint kind data (ex: 6, 32, 64 bits)
	// num is decoded data
	num, err := s.uint(typ.Bits())
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetUint(num)
	return nil
}

// decoder for Boolean kind data
// the encoded data will be either 80 or 01, where 80 will let it be treated as string
func decodeBool(s *Stream, val reflect.Value) error {
	b, err := s.Bool()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetBool(b)
	return nil
}

// decoder for String kind data
func decodeString(s *Stream, val reflect.Value) error {
	// get the content of the string
	b, err := s.Bytes()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}

	// stringify the content, and passed in to val
	val.SetString(string(b))
	return nil
}

func makeListDecoder(typ reflect.Type, tag tags) (decoder, error) {
	// getting list element type
	etype := typ.Elem()

	// check if the underlying element is byte (if the list is byte slice or byte array, will be treated as string)
	if etype.Kind() == reflect.Uint8 && !reflect.PtrTo(etype).Implements(decoderInterface) {
		if typ.Kind() == reflect.Array {
			return decodeByteArray, nil
		}
		return decodeByteSlice, nil
	}

	// get the decoder corresponded to the data type contained in the list
	etypeinfo, err := cachedTypeInfo1(etype, tags{})
	if err != nil {
		return nil, err
	}
	var dec decoder

	// because the checking condition is slice or array, therefore, need to confirm the type
	// and assign decoder for each specific type
	switch {
	case typ.Kind() == reflect.Array:
		dec = func(s *Stream, val reflect.Value) error {
			return decodeListArray(s, val, etypeinfo.decoder)
		}
	// if tail == true, it will not be treated as a slice/array because List function is not called
	// inside decodeSliceElems
	case tag.tail:
		dec = func(s *Stream, val reflect.Value) error {
			return decodeSliceElems(s, val, etypeinfo.decoder)
		}
	default:
		dec = func(s *Stream, val reflect.Value) error {
			return decodeListSlice(s, val, etypeinfo.decoder)
		}
	}
	return dec, nil
}

func makeStructDecoder(typ reflect.Type) (decoder, error) {
	// parse the structure fields, will get the index of the field and corresponded decoder
	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}
	dec := func(s *Stream, val reflect.Value) (err error) {
		// stores the list info into stack
		if _, err := s.List(); err != nil {
			return wrapStreamError(err, typ)
		}
		for _, f := range fields {
			// decode each data based on each structure field
			// val.Field returns field val based on the field index
			err := f.info.decoder(s, val.Field(f.index))
			if err == EOL {
				return &decodeError{msg: "too few elements", typ: typ}
			} else if err != nil {
				return addErrorContext(err, "."+typ.Field(f.index).Name)
			}
		}
		return wrapStreamError(s.ListEnd(), typ)
	}
	return dec, nil
}

// has one additional error checking criteria if nilOK flag was set to be true
func makeOptionalPtrDecoder(typ reflect.Type) (decoder, error) {
	// get the underlying element type and the corresponded decoder
	etype := typ.Elem()
	etypeinfo, err := cachedTypeInfo1(etype, tags{})
	if err != nil {
		return nil, err
	}

	dec := func(s *Stream, val reflect.Value) (err error) {
		kind, size, err := s.Kind()
		// criteria on checking if the value is empty
		if err != nil || size == 0 && kind != Byte {
			s.kind = -1                // rearm the kind
			val.Set(reflect.Zero(typ)) // set the value to be 0 with the type pointed by the pointer
			return err
		}
		newval := val

		// if the val pointed to nil, allocates space (allocate space in storage)
		if val.IsNil() {
			newval = reflect.New(etype)
		}
		// decode data and set val
		if err = etypeinfo.decoder(s, newval.Elem()); err == nil {
			val.Set(newval)
		}
		return err
	}
	return dec, nil
}

// this function is similar to makeOptionalPtrDecoder, the difference is that
// the empty value will not cause nil pointer. Therefore, no additional checking
// criteria
func makePtrDecoder(typ reflect.Type) (decoder, error) {
	// get the underlying element type and the corresponded decoder
	etype := typ.Elem()
	etypeinfo, err := cachedTypeInfo1(etype, tags{})
	if err != nil {
		return nil, err
	}

	dec := func(s *Stream, val reflect.Value) (err error) {
		newval := val
		if val.IsNil() {
			newval = reflect.New(etype)
		}
		if err = etypeinfo.decoder(s, newval.Elem()); err == nil {
			val.Set(newval)
		}
		return err
	}
	return dec, nil
}

// interface slice []interface{}: a slice whose element type is interface{}
var ifsliceType = reflect.TypeOf([]interface{}{})

func decodeInterface(s *Stream, val reflect.Value) error {
	// check if interface methods are exported, if there are, error
	if val.Type().NumMethod() != 0 {
		return fmt.Errorf("rlp: type %v is not RLP-serializable", val.Type())
	}

	// detailed information on encoded data
	kind, _, err := s.Kind()
	if err != nil {
		return err
	}

	// if the encoded data is List Kind, create interface slice
	// call listslice decoder and pass in the decodeInterface
	if kind == List {
		slice := reflect.New(ifsliceType).Elem()
		if err := decodeListSlice(s, slice, decodeInterface); err != nil {
			return err
		}
		val.Set(slice)
	} else {
		b, err := s.Bytes()
		if err != nil {
			return err
		}
		val.Set(reflect.ValueOf(b))
	}
	return nil
}

// decoder for storage with type slice
func decodeListSlice(s *Stream, val reflect.Value, elemdec decoder) error {
	size, err := s.List()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}

	// empty slice
	if size == 0 {
		val.Set(reflect.MakeSlice(val.Type(), 0, 0))
		return s.ListEnd()
	}
	if err := decodeSliceElems(s, val, elemdec); err != nil {
		return err
	}
	return s.ListEnd()
}

// manually increasing the cap and length of the storage array and decode the elements with
// the corresponding element decoder passed in
func decodeSliceElems(s *Stream, val reflect.Value, elemdec decoder) error {
	i := 0
	// forever loop that will break when finished reading everything from the reader
	for ; ; i++ {
		// manually increased the Cap of the slice
		// avoid automatically increasing the Cap which can cause the creation of a new slice
		if i >= val.Cap() {
			newcap := val.Cap() + val.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newv := reflect.MakeSlice(val.Type(), val.Len(), newcap)
			reflect.Copy(newv, val)
			val.Set(newv)
		}

		// manually increase the length of slice
		if i >= val.Len() {
			val.SetLen(i + 1)
		}

		// decode the slice elements with corresponding decoder
		if err := elemdec(s, val.Index(i)); err == EOL {
			break
		} else if err != nil {
			return addErrorContext(err, fmt.Sprint("[", i, "]"))
		}
	}

	// set the length of the storage slice to its actual length
	if i < val.Len() {
		val.SetLen(i)
	}
	return nil
}

// decoder for the array kind storage
func decodeListArray(s *Stream, val reflect.Value, elemdec decoder) error {
	// stores list information into stack, and do error checking
	if _, err := s.List(); err != nil {
		return wrapStreamError(err, val.Type())
	}

	// length of storage array
	vlen := val.Len()
	i := 0
	// decode the content, and put it into array, based on the index
	for ; i < vlen; i++ {
		if err := elemdec(s, val.Index(i)); err == EOL {
			break
		} else if err != nil {
			return addErrorContext(err, fmt.Sprint("[", i, "]"))
		}
	}
	if i < vlen {
		return &decodeError{msg: "input list has too few elements", typ: val.Type()}
	}
	return wrapStreamError(s.ListEnd(), val.Type())
}

// validates the encoded data to see if it is list kind
// stores the basic information into stack, which is the position and the size of the list data content
// returns the size of encoded data content
func (s *Stream) List() (size uint64, err error) {
	kind, size, err := s.Kind()
	if err != nil {
		return 0, err
	}
	if kind != List {
		return 0, ErrExpectedList
	}
	// information stored in the stack is used for error checking
	s.stack = append(s.stack, listpos{0, size})
	s.kind = -1
	s.size = 0
	return size, nil
}

// error checking after the list was decoded and pops out the list information from the stack
func (s *Stream) ListEnd() error {
	if len(s.stack) == 0 {
		return errNotInList
	}
	tos := s.stack[len(s.stack)-1]
	if tos.pos != tos.size {
		return errNotAtEOL
	}
	s.stack = s.stack[:len(s.stack)-1] // pop out information from the stack
	if len(s.stack) > 0 {
		s.stack[len(s.stack)-1].pos += tos.size
	}
	s.kind = -1
	s.size = 0
	return nil
}

// decoder for decoding byte array
func decodeByteArray(s *Stream, val reflect.Value) error {
	// getting detailed information on encoded data
	kind, size, err := s.Kind()

	if err != nil {
		return err
	}

	// getting the length of declared ByteArray
	vlen := val.Len()

	switch kind {
	// put a byte in a byte array
	case Byte:
		if vlen == 0 {
			return &decodeError{msg: "input string too long", typ: val.Type()}
		}
		if vlen > 1 {
			return &decodeError{msg: "input string too short", typ: val.Type()}
		}

		// get the content and stores in the index 0
		bv, _ := s.Uint()
		val.Index(0).SetUint(bv)

	// put string in a byte array
	case String:
		if uint64(vlen) < size {
			return &decodeError{msg: "input string too long", typ: val.Type()}
		}
		if uint64(vlen) > size {
			return &decodeError{msg: "input string too short", typ: val.Type()}
		}

		// transfer the byte array to byte slice and place string content inside
		slice := val.Slice(0, vlen).Interface().([]byte)
		if err := s.readFull(slice); err != nil {
			return err
		}

		// reject cases where single byte encoding should have been used
		if size == 1 && slice[0] < 128 {
			return wrapStreamError(ErrCanonSize, val.Type())
		}
	// byte array should not contain any list
	case List:
		return wrapStreamError(ErrExpectedString, val.Type())
	}
	return nil
}

// decoder used to decode storage with ByteSlice type
func decodeByteSlice(s *Stream, val reflect.Value) error {
	// b = byte slice contained string content
	b, err := s.Bytes()
	if err != nil {
		return wrapStreamError(err, val.Type())
	}
	val.SetBytes(b)
	return nil
}

// format streamError through decodeError structure
func wrapStreamError(err error, typ reflect.Type) error {
	switch err {
	case ErrCanonInt:
		return &decodeError{msg: "non-canonical integer (leading zero bytes)", typ: typ}
	case ErrCanonSize:
		return &decodeError{msg: "non-canonical size information", typ: typ}
	case ErrExpectedList:
		return &decodeError{msg: "expected input list", typ: typ}
	case ErrExpectedString:
		return &decodeError{msg: "expected input string or byte", typ: typ}
	case errUintOverflow:
		return &decodeError{msg: "input string too long", typ: typ}
	case errNotAtEOL:
		return &decodeError{msg: "input list has too many elements", typ: typ}
	}
	return err
}

// add specific error message context
func addErrorContext(err error, ctx string) error {
	if decErr, ok := err.(*decodeError); ok {
		decErr.ctx = append(decErr.ctx, ctx)
	}
	return err
}

// calls uint function with maxbits64
// returns the uint64 representation of encoded data content
func (s *Stream) Uint() (uint64, error) {
	return s.uint(64)
}

func (s *Stream) Bool() (bool, error) {
	num, err := s.uint(8)
	if err != nil {
		return false, err
	}
	switch num {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("rlp: invalid boolean value: %d", num)
	}
}

// this function read data from s.r and return the uint64 representation of the data
// maxbits represents maximum data allowed to be read from s.r, used for error checking
// content will be transferred to uint64 representation from hex representation
func (s *Stream) uint(maxbits int) (uint64, error) {
	// get the kind and size of encoded data
	// NOTE: after this function, the headers are removed, the only thing left is pure data
	kind, size, err := s.Kind()
	if err != nil {
		return 0, err
	}
	switch kind {
	case Byte:
		// a single byte cannot be 0, 0 should be represents with 0x80
		if s.byteval == 0 {
			return 0, ErrCanonInt
		}
		s.kind = -1 // rearm Kind
		return uint64(s.byteval), nil
	case String:
		// data size is greater than the max size the storage can contained
		if size > uint64(maxbits/8) {
			return 0, errUintOverflow
		}

		// v will be uint64 representation of the encode uint
		v, err := s.readUint(byte(size))
		switch {
		case err == ErrCanonSize:
			return 0, ErrCanonInt
		case err != nil:
			return 0, err
		case size > 0 && v < 128:
			return 0, ErrCanonSize
		default:
			return v, nil
		}
	default:
		return 0, ErrExpectedString
	}
}

// Bytes reads an RLP string and returns its contents as a byte slice.
// NOTE: the content here means data without string headers
func (s *Stream) Bytes() ([]byte, error) {
	kind, size, err := s.Kind()
	if err != nil {
		return nil, err
	}
	switch kind {
	case Byte:
		s.kind = -1 // re-initialize Kind
		return []byte{s.byteval}, nil
	case String:
		b := make([]byte, size)
		// read the string content, store in the byte slice
		if err = s.readFull(b); err != nil {
			return nil, err
		}
		if size == 1 && b[0] < 128 {
			return nil, ErrCanonSize
		}
		return b, nil
	default:
		return nil, ErrExpectedString
	}
}

// function on decoding Raw type variable
// NOTE: rawValue stores pre-encoded data
func (s *Stream) Raw() ([]byte, error) {
	// get the kind and size of encoded data s.r
	kind, size, err := s.Kind()
	if err != nil {
		return nil, err
	}
	// if it is a single byte, the value is stored in s.byteval, therefore can be returned directly
	if kind == Byte {
		s.kind = -1 // re-initialize s.Kind
		return []byte{s.byteval}, nil
	}

	// get the size of the string heads and make a byte slice with its' total length
	// which is data length + length of heads
	start := headsize(size)
	buf := make([]byte, uint64(start)+size)

	// stores the remaining data (data without headers) into buf
	if err := s.readFull(buf[start:]); err != nil {
		return nil, err
	}

	// based on data kind, place the headers in front of the buf
	if kind == String {
		puthead(buf, 0x80, 0xB7, size)
	} else {
		puthead(buf, 0xC0, 0xF7, size)
	}
	return buf, nil
}

// returns size of string heads.
func headsize(size uint64) int {
	// if the size is within 0 - 55, then the size of the string head will be 1 (0x80 + ... RULE #2)
	if size < 56 {
		return 1
	}
	// otherwise, size will be RULE #3
	return 1 + intsize(size)
}

// this function performed additional error checking criteria on information returned by readKind()
// returns kind && size of encoded data
func (s *Stream) Kind() (kind Kind, size uint64, err error) {
	// in this function, the soul purpose of using tos is for EOL error checking
	var tos *listpos

	// tos will pointed to the innermost list (the last one stored in stack)
	if len(s.stack) > 0 {
		tos = &s.stack[len(s.stack)-1]
	}
	if s.kind < 0 {
		s.kinderr = nil
		// if the starting position of the list == the ending position of the list
		// it indicates the end of list
		if tos != nil && tos.pos == tos.size {
			return 0, 0, EOL
		}
		// get the kind, size of the data, and possible error
		s.kind, s.size, s.kinderr = s.readKind()

		// additional error checking
		if s.kinderr == nil {
			if tos == nil {
				// at toplevel, check that the value is smaller
				// than the remaining input length
				if s.limited && s.size > s.remaining {
					s.kinderr = ErrValueTooLarge
				}
			} else {
				// inside a list, check that the value does not overflow the list
				if s.size > tos.size-tos.pos {
					s.kinderr = ErrElemTooLarge
				}
			}
		}
	}
	return s.kind, s.size, s.kinderr
}

// this function returns the kind of the data along with the length of the data
func (s *Stream) readKind() (kind Kind, size uint64, err error) {
	// get a byte that pops out from s.r
	b, err := s.readByte()
	// error handling, change all possible errors to io.EOF
	if err != nil {
		if len(s.stack) == 0 {
			switch err {
			// At toplevel, Adjust the error to actual EOF. io.EOF is used
			// by callers to determine when to stop decoding
			case io.ErrUnexpectedEOF:
				err = io.EOF
			case ErrValueTooLarge:
				err = io.EOF
			}
		}
		return 0, 0, err
	}
	s.byteval = 0 // initialize to 0 first
	switch {
	// indicates the encoded value is single byte, therefore, set the byteval to the actual byte value (ENCODING RULE #1)
	case b < 0x80:
		s.byteval = b
		return Byte, 0, nil
	// indicates the encoded value is a string, with length b-0x80 (Encoding RULE #2)
	case b < 0xB8:
		return String, uint64(b - 0x80), nil
	// Encoding RULE #3
	case b < 0xC0:
		size, err = s.readUint(b - 0xB7)
		if err == nil && size < 56 {
			err = ErrCanonSize
		}
		return String, size, err
	// Encoding Rule #4
	case b < 0xF8:
		return List, uint64(b - 0xC0), nil
	// Encoding Rule #5
	default:
		size, err = s.readUint(b - 0xF7)
		if err == nil && size < 56 {
			err = ErrCanonSize
		}
		return List, size, err
	}

}

/*
	This function reads the data from s.r
	The data read can be hex representation of the length of actual data
		* For Example: B80166 RULE 3, the data read will be 01 represents the length of the actual data got encoded, which 01
	The data read can also be the actual value of uint

	This function will convert the data read into uint64 format
*/
func (s *Stream) readUint(size byte) (uint64, error) {
	switch size {
	case 0:
		s.kind = -1
		return 0, nil
	// if the difference is 1, it means the length of the encoded data can be represented with 1 byte
	// the next byte will be the hex representation of the length
	case 1:
		// read the next byte, and return it as length
		b, err := s.readByte()
		return uint64(b), err

	// if the length needs to be represented is more than one byte
	// then the readFull will be used to read the entire hex representation of th length
	// and return it
	default:
		start := int(8 - size)
		// reserve the last few bytes to store the hex representation of length
		// fill the rest of bytes with 0s
		for i := 0; i < start; i++ {
			s.uintbuf[i] = 0
		}
		if err := s.readFull(s.uintbuf[start:]); err != nil {
			return 0, err
		}
		// first byte cannot be 0 (Encoding Rules, no leading 0s)
		if s.uintbuf[start] == 0 {
			return 0, ErrCanonSize
		}
		// return in BigEndian order (encoding rule: positive integer must be represent in Big Endian Format)
		return binary.BigEndian.Uint64(s.uintbuf), nil
	}
}

func (s *Stream) readFull(buf []byte) (err error) {
	// go to the willRead, check condition and modify remaining first
	if err := s.willRead(uint64(len(buf))); err != nil {
		return err
	}
	// read buffer size variable, which stores the hex representation of the length
	var nn, n int

	// possible error: not enough bytes to be read from reader, causing EOF error
	for n < len(buf) && err == nil {
		nn, err = s.r.Read(buf[n:])
		n += nn
	}
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// this function gets and returns byte pops out from s.r
func (s *Stream) readByte() (byte, error) {
	// since this is readByte functions, therefore, only willRead a byte each time
	if err := s.willRead(1); err != nil {
		return 0, err
	}

	// pops out a byte from r and return it
	b, err := s.r.ReadByte()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return b, err
}

// Check if number of bytes required to read is reasonable
// if it is not, return corresponding errors
// otherwise, modify s.remaining and s.stack
// input n = how many bytes will be read
func (s *Stream) willRead(n uint64) error {
	s.kind = -1 // rearm / re-initialize Kind
	if len(s.stack) > 0 {
		tos := s.stack[len(s.stack)-1]
		// read size cannot greater than the size of the list
		if n > tos.size-tos.pos {
			return ErrElemTooLarge
		}
		// change the list position
		s.stack[len(s.stack)-1].pos += n
	}
	if s.limited {

		if n > s.remaining {
			return ErrValueTooLarge
		}
		s.remaining -= n
	}
	return nil
}
