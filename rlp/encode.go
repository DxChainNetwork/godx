/*
5 Encoding Rules:

RULE #1:
Single Byte within the range [0x00, 0x7f] (ASCII Table), the encoded value will be itself. If the single byte is a character, the encoded value will be hex representation getting from ASCII table

RULE #2:
For string with 0-55 bytes long (empty string -> string with 55 characters), or single byte that is larger than 0x7f, a single byte with value 0x80 + the length of the string must be placed in front of the encoded value, and the range of it will be [0x80, 0xb7]

RULE #3:
For string has more than 55 bytes, the way to encode follows the equation below:
Equation:
(b7 + number of bytes that can be used to represents the length of the string)(hex representation of string length)(followed by each character in hex representation)
Since a typical computer only support 8 bytes of binary string (64-bit processor), the range will be [0xb8 - 0xbf]

RULE #4:
For list with its total length (entire encoded length, which is the length of encoded value) in between 0 - 55 bytes, a single byte with value 0xc0 + the total length of the list must be placed in front of the encoded list, and the range of it will be [0xc0, 0xf7] Then, for each element inside the list, it must follows the previous rules.

RULE #5:
For list with its total length is more that 55 bytes long, the way to encode follows the equation below:
Equation:
(F7 + length of the string in bytes)(hex representation of the list length)(followed by each character in hex representation)

NOTE:
* RLP treat all type of data into two types, string and list of string
  !!! ByteArray / ByteSlice can be treated as STRING (IN GENERAL, NOT ETHEREUM PARTICULAR)

EmptyString => 0x80
EmptyList   => 0xC0
0           => 0x80

POINTERS:
For pointers, the value pointed by the pointer is encoded
For nil pointers, encode the zero value of the type
For nil pointers pointed to struct or array, empty list will be encoded (0xC0)

BigInt:
RLP cannot encode negative negative BigInt
For BigInt with value 0, encode it as empty string (0x80)
Otherwise, treat it as string and call encodeString method

Terminologies:
What are list headers: Bytes needed to encode a list. For example: the list header for
uint[5, 6] is C2

Actual Encoded Data = List Headers + Actual Encoded Data

Encoder Interface:
* For any variable that implements Encoder interface, it means the user is allowed to define his own encoding
rules because if a variable implement Encoder interface, then the writer/encoder returned by makeWriter function
will be EncodeRLP method, which is implemented by user.

Functions:
* Public Functions
	- Encode
	- EncodeToBytes
	- makeWriter  -> This function only called in typecache.go to store encoder

* Encode Function
	- When this function is called separately, it means custom io.writer function must be implemented
	- This function can also be used along with the EncodeRLP. In that case, io.writer do not need to be defined

* EncodeToBytes Function
	- This function is mostly used to encode data
	- The main difference between this function and Encode function (described above) is that Encode function
	allows custom writer function to be passed in.
	- This function can be used along with Encode and EncodeRLP function to encode variables implement Encoder interface

Encoding Flow:
* EncodeToBytes -> encode -> CacheTypeInfo -> CacheTypeInfo1 -> genTypeInfo -> makeWriter
	- encode: encode data, value will be stored in encbuf object
	- CacheTypeInfo: based on the information (data type -> reflect.type & special characteristics of data -> tags)
  	passed in, check if the decoder&encoder (stores in typeinfo) corresponding to this tpe is already generated
	- CacheTypeInfo1: this function will first check if the typeinfo is already created by another routine, if not
	then call genTypeInfo to generate decoder and encoder
	- genTypeInfo: calls makeDecoder and makeWriter function to generate encoder and decoder for the specific data type
	and stores in typeinfo data structure
	- makeWriter: based on the type, tags, and kind of a variable, return a function that is used to encode data

Supported Data Type Encoding:
	* rawValue -> stores data that are pre-encoded (defined in typecache.go)
		- store the bytes directly to encbuf.str
	* Encoder Interface
		- Call EncodeRLP method defined by the user
	* Non-Pointer Encoder Interface
		- Call EncodeRLP method defined by the user if data passed in is addressable, which means the address of the variable can be obtained
		- If it is not addressable, EncoderRLP function cannot be called because EncodeRLP is a pointer method
	* Interface
		- If the value is Nil, then treat it as empty list
		- Otherwise, get the type of its' underlying element, get the encoder, and encode it
	* bigInt Pointer -- encode as string, non-negative only
		- Transfer it from reflect.val type to bigInt type (in pointer format)
		- If it points to Nil, encode it as empty string
		- Otherwise, call writeBigInt function
	* bigInt No Pointer -- encode as string, non-negative only
		- Transfer it from reflect.val type to bigInt type
		- Call writeBigInt and pass in the address (which means the pointer format)
	* Uint
		- Applying the same rules as encoded string
		- However, only RULE #1 or RULE #2 will be applied because the max size of uint is 8 bytes (64 bits)
	* Boolean
		- If false, encode it as 0x80. Otherwise, encode it as  0x01
	* String -- Follows the rules listed above
	* Byte Slice -- Encode as string
	* Byte Array -- Encode as string
		- Slice is addressable, therefore, transfer array to addressable first if it is not addressable
		- Transfer Byte Array to Byte Slice first
		- Reason: Byte Slice is equivalent to string, not byte array
	* Standard Slice or Array -- Encode as list, follows the rules listed above
	* structure -- Encode each field (NOTE: tags play a big role here)
	* Pointer -- Encode the data pointed by pointer

Functions used for determining list headers:
* list: Getting the starting position of the list. Getting the current number of list headers
* listEnd: Getting the length of encoded data (lheads.size) after encoded data inside the list.
           and setting the number of list headers based on the lheads.size (RULE #4, RULE #5)

*/

package rlp

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sync"
)

var (
	// the reason to define these two variables is easy usability
	EmptyString = []byte{0x80}
	EmptyList   = []byte{0xC0}

	// to get the type of the interface type Encoder
	// new(Encoder) allocated spaces for Encoder variable pointed to the location
	// .Elem refers to the underlying value of the pointer
	// together, it refers to the type of the interface
	encoderInterface = reflect.TypeOf(new(Encoder)).Elem()
	big0             = big.NewInt(0)
)

// encbufs pool, stores a set of encbuf objects
var encbufPool = sync.Pool{
	// why allocates 9 slots?
	// the max number for identifier (string/list) is 8 bytes
	New: func() interface{} { return &encbuf{sizebuf: make([]byte, 9)} },
}

type encReader struct {
	buf    *encbuf // the buffer we're reading from. this is nil when we're at EOF.
	lhpos  int     // index of list header that we're reading
	strpos int     // current position in string buffer
	piece  []byte  // next piece to be read
}

// Encoder interface allow user to define custom encoding rules or to encode private fields
// because for encoding encoder interface, encoder will be using EncodeRLP methods
type Encoder interface {
	// the writer passed into EncodeRLP will be func (w *encbuf) Write(b []byte) if it is called through EncodeToBytes function
	// writer can be defined by user if it is not called through EncodeToBytes function
	// EncodeRLP method is user defined, which will only be used if data belongs to Encoder Interface
	EncodeRLP(io.Writer) error
}

// the structure where stores the data after encoded
type encbuf struct {
	str     []byte      // stores encoded data, except list headers
	lheads  []*listhead // contains detailed information of all list headers
	lhsize  int         // number of list headers
	sizebuf []byte      // temporary buffer that is used to store the byte (ex: RULE #2 0x80)
}

// listhead data structure defines the list header details
type listhead struct {
	offset int // index of this header in the string data
	size   int // total size of encoded data (including list headers if the encoded data contains list elements)
	// it will not include the current list headers. For example, size = 3 for ["fff"] (0x836666)
}

// The main difference between Encode and EncodeToBytes is that Encode function allows custom io.writer function to be passed in.
// NOTE: io.Writer is a data/variable that implements writer function
func Encode(w io.Writer, val interface{}) error {
	// Check if writer object passed in is the type *encbuf, if it is, the value needs to be encoded will be passed in
	// Outer is the *encbuf object
	if outer, ok := w.(*encbuf); ok {
		// Why this did not need to handle list headers?
		// since the data passed in is *encbuf, therefore, it means the function is called through EncodeToBytes
		// where the list headers will be handled there
		return outer.encode(val)
	}

	// Otherwise, get *encbuf object from the pool
	eb := encbufPool.Get().(*encbuf)
	defer encbufPool.Put(eb)

	// Init lhize, str, and lheads
	eb.reset()

	if err := eb.encode(val); err != nil {
		return err
	}
	return eb.toWriter(w)
}

// function that gets empty encbuf data structure and pass into encode function
func EncodeToBytes(val interface{}) ([]byte, error) {
	eb := encbufPool.Get().(*encbuf)
	defer encbufPool.Put(eb)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return nil, err
	}
	return eb.toBytes(), nil
}

// EncodeToReader returns a reader from which the RLP encoding of val
// can be read. The returned size is the total size of the encoded
// data. (encoded data + list headers)
func EncodeToReader(val interface{}) (size int, r io.Reader, err error) {
	eb := encbufPool.Get().(*encbuf)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return 0, nil, err
	}
	return eb.size(), &encReader{buf: eb}, nil
}

// Creates writer function (encoder), based on the give data type
// case 2 and case 3 referred that the receiver type of the EncodeRLP must be pointer
func makeWriter(typ reflect.Type, ts tags) (writer, error) {
	kind := typ.Kind()
	switch {
	// rawValue stores values that are pre-encoded
	case typ == rawValueType:
		return writeRawValue, nil
	// if the type implements the encoderInterface (Encoder)
	// the value can be pointer or not, as long as it implements the interface
	// by implementing the interface, it means the type belongs to interface
	case typ.Implements(encoderInterface):
		return writeEncoder, nil
	// if interface methods' receiver is a pointer type, then the kind will be pointer
	// kind of the value is not pointer --> Data != *Ptr
	// pointer type of the data implements the interface --> func (*dataType) EncodeRLP
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(encoderInterface):
		return writeEncoderNoPtr, nil
	// if the data kind is an interface, generate writer for its' underlying value type
	case kind == reflect.Interface:
		return writeInterface, nil
	// check if the value type is bigint pointer type
	// if it is assignable will return true
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		return writeBigIntPtr, nil
	// if the value type is bigInt
	case typ.AssignableTo(bigInt):
		return writeBigIntNoPtr, nil
	// allow both uint and uintptr kind data
	case isUint(kind):
		return writeUint, nil
	case kind == reflect.Bool:
		return writeBool, nil
	case kind == reflect.String:
		return writeString, nil
	case kind == reflect.Slice && isByte(typ.Elem()):
		return writeBytes, nil
	// array with each element contained has type byte, ex: [3]byte{'1', 'a', 0x80}
	case kind == reflect.Array && isByte(typ.Elem()):
		return writeByteArray, nil
	case kind == reflect.Slice || kind == reflect.Array:
		// it returns a function that returns a writer function and error
		return makeSliceWriter(typ, ts)
	case kind == reflect.Struct:
		return makeStructWriter(typ)
	case kind == reflect.Ptr:
		return makePtrWriter(typ)
	default:
		return nil, fmt.Errorf("rlp: type %v is not RLP serializable", typ)
	}
}

// this function acts like toBytes, however, it is used for Encode function with user defined writer
func (w *encbuf) toWriter(out io.Writer) (err error) {
	strpos := 0
	for _, head := range w.lheads {
		// starting position of the list header is not located in the beginning of the encoded value (encbuf.str)
		// it means w.str has encoded data
		if head.offset-strpos > 0 {
			// write the data before header to underlying data stream and returns number of elements written in
			// if there are encoded data, then call custom write function to write the encoded data into the user defined data structure
			n, err := out.Write(w.str[strpos:head.offset])
			strpos += n
			if err != nil {
				return err
			}
		}
		// return the header, and stores in enc
		enc := head.encode(w.sizebuf)

		// handles by user defined Write function
		if _, err = out.Write(enc); err != nil {
			return err
		}
	}
	// if there are data left
	if strpos < len(w.str) {
		_, err = out.Write(w.str[strpos:])
	}
	return err
}

// function that combine encoded data and list headers (NOTE: list headers are separated from encoded data)
func (w *encbuf) toBytes() []byte {
	// create an empty byte array with size encoded data length + number of encoded list headers
	// which is equal to the actual size of encoded data because Actual Encoded Data = List Headers + Actual Encoded Data
	out := make([]byte, w.size())
	strpos := 0
	pos := 0
	for _, head := range w.lheads {
		n := copy(out[pos:], w.str[strpos:head.offset])
		pos += n
		strpos += n

		enc := head.encode(out[pos:])
		pos += len(enc)
	}
	copy(out[pos:], w.str[strpos:])
	return out
}

// this function writes list header into buffer
// buffer must be at least 9 bytes long
// size represents the length of the list (entire encoded length)
// in decoding, the function also writes string headers
func puthead(buf []byte, smalltag, largetag byte, size uint64) int {
	// RULE #4
	if size < 56 {
		buf[0] = smalltag + byte(size)
		return 1
	}
	// RULE #5
	sizesize := putint(buf[1:], size)
	buf[0] = largetag + byte(sizesize)
	return sizesize + 1
}

// return the list header in the form of []byte (list header refers to 0xC0 etc. RULE 4, RULE 5)
func (head *listhead) encode(buf []byte) []byte {
	// it is neither buf[0] or buf[0, sizesize]
	return buf[:puthead(buf, 0xC0, 0xF7, uint64(head.size))]
}

func (w *encbuf) encode(val interface{}) error {
	// Create reflect.Value instance (a copy of the value, read only)
	rval := reflect.ValueOf(val)

	// Based on type of the value and tags info, return typeinfo
	// included the way to decode the type
	ti, err := cachedTypeInfo(rval.Type(), tags{})
	if err != nil {
		return err
	}
	return ti.writer(rval, w)
}

func (w *encbuf) Write(b []byte) (int, error) {
	w.str = append(w.str, b...)
	return len(b), nil
}

// Initialize encbuf object
func (w *encbuf) reset() {
	w.lhsize = 0
	if w.str != nil {
		w.str = w.str[:0]
	}
	if w.lheads != nil {
		w.lheads = w.lheads[:0]
	}
}

// writer(encoder) for rawValueType data
func writeRawValue(val reflect.Value, w *encbuf) error {
	// .Bytes() must be used for appending purpose
	// without .Bytes(), the data cannot be parsed using ...
	w.str = append(w.str, val.Bytes()...)
	return nil
}

// writer for struct that implemented Encoder interface
func writeEncoder(val reflect.Value, w *encbuf) error {
	// returns val's current value as interface Encoder, and call EncodeRLP(w) function
	// EncodeRLP returns: error type
	// All logic on encoding will be in user defined EncodeRLP
	/*
		val.Interface().(Encoder) -- transfer the data type to the interface type passed in, in this case (Encoder)
		WHY USE val.Interface().(Encoder) to transfer types to Encoder
		REASON: currently, the type of the val is reflect.Value, therefore,
				cannot use EncodeRLP(w) method
	*/
	return val.Interface().(Encoder).EncodeRLP(w)
}

// the value passed in is not pointer, however, it's pointer type implemented the encoder interfaces
func writeEncoderNoPtr(val reflect.Value, w *encbuf) error {
	// if the value is not addressable, return error
	// by not addressable, it means the address of val cannot be obtained
	if !val.CanAddr() {
		return fmt.Errorf("rlp: game over: unaddressable value of type %v, EncodeRLP is pointer method", val.Type())
	}
	// same as writeEncoder, val.Addr() returns pointer type of the value
	return val.Addr().Interface().(Encoder).EncodeRLP(w)
}

func writeInterface(val reflect.Value, w *encbuf) error {
	// if the interface is Nil, write 0xC0 to str
	if val.IsNil() {
		w.str = append(w.str, 0xC0)
		return nil
	}

	// get the underlying value of the interface
	// and find the writer corresponding to the
	// underlying value type
	eval := val.Elem()
	ti, err := cachedTypeInfo(eval.Type(), tags{})
	if err != nil {
		return err
	}
	return ti.writer(eval, w)
}

func writeBigIntPtr(val reflect.Value, w *encbuf) error {
	// Transfer the value back to the big.Int type instead of reflect.Value type
	ptr := val.Interface().(*big.Int)
	if ptr == nil {
		w.str = append(w.str, 0x80)
		return nil
	}
	return writeBigInt(ptr, w)
}

func writeBigIntNoPtr(val reflect.Value, w *encbuf) error {
	i := val.Interface().(big.Int)
	return writeBigInt(&i, w)
}

func writeBigInt(i *big.Int, w *encbuf) error {
	// if i < 0, error
	if cmp := i.Cmp(big0); cmp == -1 {
		return fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if cmp == 0 {
		// if i == 0, write 0x80
		w.str = append(w.str, 0x80)
		// if i > 0, encoding the big endian bytes representation of the value
	} else {
		w.encodeString(i.Bytes()) // this int.Bytes() transfer the value into byte slice
	}
	return nil
}

// writer function to encode data with type Uint
func writeUint(val reflect.Value, w *encbuf) error {
	i := val.Uint() // transfer back from type reflect.Value to Uint
	if i == 0 {
		w.str = append(w.str, 0x80)
	} else if i < 128 {
		// Single byte within in 0x00 - 0x7F, RULE #1
		w.str = append(w.str, byte(i))
	} else {
		// RULE #2. REASON: Uint MAX: 8 byte
		s := putint(w.sizebuf[1:], i) // this returns how many bytes can be used to represent the uint
		// also, it split uint into bytes and placed into sizebuf
		w.sizebuf[0] = 0x80 + byte(s)
		w.str = append(w.str, w.sizebuf[:s+1]...)
	}
	return nil
}

// writer function to encode data with type Boolean
func writeBool(val reflect.Value, w *encbuf) error {
	if val.Bool() {
		// if true, 0x01, if false, 0x80. 0x80 represents empty
		w.str = append(w.str, 0x01)
	} else {
		w.str = append(w.str, 0x80)
	}
	return nil
}

// function that takes string and do encoding. Implemented RULE #1 Encoding Method
// Difference between writeString and encodeString:
// encodeString is used for byteslice, and writeString is used for String
func writeString(val reflect.Value, w *encbuf) error {
	s := val.String()
	if len(s) == 1 && s[0] <= 0x7F {
		w.str = append(w.str, s[0])
	} else {
		w.encodeStringHeader(len(s))
		w.str = append(w.str, s...)
	}
	return nil
}

// Encode bytes slice data type.
func writeBytes(val reflect.Value, w *encbuf) error {
	w.encodeString(val.Bytes())
	return nil
}

// writer function to encode array with each element has type byte.
// this function transfer the ByteArray to slice, and then encoding using string method
// why make it addressable? slice is addressable
func writeByteArray(val reflect.Value, w *encbuf) error {
	// Check if the value is addressable. Value only addressable when the address of the variable can be obtained
	// slice REQUIRES the value to be addressable
	if !val.CanAddr() {
		// first, allocated space for ByteArray, copy = values underlying by ByteArray
		// without .Elem, it will be pointer, .Elem dereference it and returns the value type pointed by the pointer
		copy := reflect.New(val.Type()).Elem()
		// set the original values back to the new addressable ByteArray
		copy.Set(val)
		val = copy
	}
	size := val.Len()                   // length of the ByteArray
	slice := val.Slice(0, size).Bytes() // val.Slice is pointer pointed to the slice.Bytes() revealed the values in the address (dereference)
	w.encodeString(slice)
	return nil
}

func makeSliceWriter(typ reflect.Type, ts tags) (writer, error) {
	// get the writer for underlying value type. For instance: string
	etypeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}
	// defines slice writer
	writer := func(val reflect.Value, w *encbuf) error {
		// if the slice is not the tail, define lheads and lhsize
		if !ts.tail {
			// NOTE: w.list() will execute first once get into this if statement. It should not be treated as defer function
			// Then, w.listEnd() will run after function return (defer)
			defer w.listEnd(w.list())
		}

		// Get slice's length and encode each element using its' encoder
		// results will be something like this: ENCODE []{"ff", "ff"} -> 826666826666
		// now, the only thing missing is the code indicate the list type
		// which will be inserted using w.listEnd(w.list()) function above
		vlen := val.Len()
		for i := 0; i < vlen; i++ {
			if err := etypeinfo.writer(val.Index(i), w); err != nil {
				return err
			}
		}
		return nil
	}
	return writer, nil
}

// writer to encode structure kind data
func makeStructWriter(typ reflect.Type) (writer, error) {
	// get a list of structure field's indexes, along with needed writers&decoders for each field
	// ex: 0 {writer, decoder}, 1 {writer, decoder}
	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}
	writer := func(val reflect.Value, w *encbuf) error {
		// structure will be treated as list, therefore, it calls w.list() function at beginning
		lh := w.list()
		for _, f := range fields {
			// encode each fields and place all encoded values in w.str
			if err := f.info.writer(val.Field(f.index), w); err != nil {
				return err
			}
		}
		// and calls listEnd() at the end
		w.listEnd(lh)
		return nil
	}
	return writer, nil
}

// writer to encode pointer kind data
func makePtrWriter(typ reflect.Type) (writer, error) {
	// get the writer and decoder for underlying value type pointed by the pointer
	etypeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}

	// nil pointer handler
	var nilfunc func(*encbuf) error

	// get the underlying value data kind pointed by the pointer
	kind := typ.Elem().Kind()

	// defines nil pointer handler
	switch {
	// check if the pointer pointed to byte array
	case kind == reflect.Array && isByte(typ.Elem().Elem()):
		// []byte{} -> encoded value will be 0x80, encoded as string
		nilfunc = func(w *encbuf) error {
			w.str = append(w.str, 0x80)
			return nil
		}
	// check if the pointer pointed to a structure or array
	case kind == reflect.Struct || kind == reflect.Array:
		// []string{} -> encoded as list
		nilfunc = func(w *encbuf) error {
			w.listEnd(w.list())
			return nil
		}
	default:
		// if the underlying type is not array, structure, or byte array
		// place zero to the writer and encoded based on the type
		zero := reflect.Zero(typ.Elem())
		nilfunc = func(w *encbuf) error {
			return etypeinfo.writer(zero, w)
		}
	}
	writer := func(val reflect.Value, w *encbuf) error {
		// if the pointer is nil
		if val.IsNil() {
			return nilfunc(w)
		}
		// if not, return the writer corresponding to the pointer underlie value
		return etypeinfo.writer(val.Elem(), w)
	}
	return writer, err
}

// this function defines list starting information
// including the index of the list header (offset)
func (w *encbuf) list() *listhead {
	// offset defines the index of the list headers
	// size defines encoded data size including the headers (this is the piece of information needed to calculate the list header: RULE4 & RULE5)
	lh := &listhead{offset: len(w.str), size: w.lhsize}
	w.lheads = append(w.lheads, lh)
	return lh
}

func (w *encbuf) listEnd(lh *listhead) {
	// NOTE: lh if pointed to the w.lheads[i]
	// when lh.size changed, the w.lheads[i].size also changes
	// this returns total length of encoded data included list headers (by list headers, it means if there are data contained in the list is a list type
	// the number of headers will be included as well. However, the current list header will not be included)
	// for example: ["ff"], the lh.size will be 3 (836666)
	lh.size = w.size() - lh.offset - lh.size
	if lh.size < 56 {
		w.lhsize++
	} else {
		w.lhsize += 1 + intsize(uint64(lh.size))
	}
}

// return length of the encoded data + w.lhsize (number of headers) -> the length of actual encoded data
// this function defines the length of encoded list data
func (w *encbuf) size() int {
	return len(w.str) + w.lhsize
}

// times it takes to shift i means byte can be used to represent the i
func intsize(i uint64) (size int) {
	for size = 1; ; size++ {
		if i >>= 8; i == 0 {
			return size
		}
	}
}

// function takes byte slice and encode it using string method. Implemented RULE #1 Encoding Method
func (w *encbuf) encodeString(b []byte) {
	// if it is single byte, and it is smaller than 0x7F
	// return itself RULE #1
	if len(b) == 1 && b[0] <= 0x7F {
		w.str = append(w.str, b[0])
	} else {
		// Insert headers into w.str first
		w.encodeStringHeader(len(b))
		w.str = append(w.str, b...)
	} // if the data is not single byte, apply RULE#2 and RULE#3
}

// This function implemented RULE#2 and RULE#3
// for data that is greater than a byte or not within ASCII Chart range
func (w *encbuf) encodeStringHeader(size int) {
	if size < 56 {
		// add the header byte to indicate it is a string type
		w.str = append(w.str, 0x80+byte(size)) // RULE #2
	} else {
		// add the header byte to indicate it is a string type and the length is greater than 55
		sizesize := putint(w.sizebuf[1:], uint64(size))
		w.sizebuf[0] = 0xB7 + byte(sizesize)
		w.str = append(w.str, w.sizebuf[:sizesize+1]...)
	} // RULE #3
}

// Returns the number of bytes used to represent length of the string/list.
// Also, stores each byte into sizebuf in BIG ENDIAN fashion (most significant in lowest address)
func putint(b []byte, i uint64) (size int) {
	switch {
	// if length of string is smaller than 1000 0000 (1 << 8), then it can be represent by 1 byte
	case i < (1 << 8):
		b[0] = byte(i)
		return 1
	// if length of string is smaller than 1 << 16, then it can be represent by 2 byte
	case i < (1 << 16):
		// upper byte
		b[0] = byte(i >> 8)
		// lower byte
		b[1] = byte(i)
		return 2
	case i < (1 << 24):
		b[0] = byte(i >> 16)
		b[1] = byte(i >> 8)
		b[2] = byte(i)
		return 3
	case i < (1 << 32):
		b[0] = byte(i >> 24)
		b[1] = byte(i >> 16)
		b[2] = byte(i >> 8)
		b[3] = byte(i)
		return 4
	case i < (1 << 40):
		b[0] = byte(i >> 32)
		b[1] = byte(i >> 24)
		b[2] = byte(i >> 16)
		b[3] = byte(i >> 8)
		b[4] = byte(i)
		return 5
	case i < (1 << 48):
		b[0] = byte(i >> 40)
		b[1] = byte(i >> 32)
		b[2] = byte(i >> 24)
		b[3] = byte(i >> 16)
		b[4] = byte(i >> 8)
		b[5] = byte(i)
		return 6
	case i < (1 << 56):
		b[0] = byte(i >> 48)
		b[1] = byte(i >> 40)
		b[2] = byte(i >> 32)
		b[3] = byte(i >> 24)
		b[4] = byte(i >> 16)
		b[5] = byte(i >> 8)
		b[6] = byte(i)
		return 7
	default:
		b[0] = byte(i >> 56)
		b[1] = byte(i >> 48)
		b[2] = byte(i >> 40)
		b[3] = byte(i >> 32)
		b[4] = byte(i >> 24)
		b[5] = byte(i >> 16)
		b[6] = byte(i >> 8)
		b[7] = byte(i)
		return 8
	}
}

// Check if the data type is byte
func isByte(typ reflect.Type) bool {
	//Why it cannot belongs to encoderInterface?
	// If it belongs to encoderInterface, it should be called by another method
	return typ.Kind() == reflect.Uint8 && !typ.Implements(encoderInterface)
}

// NOTE: the data is read piece by piece, however, it is still depend on the size of the byte slice passed in
func (r *encReader) Read(b []byte) (n int, err error) {
	for {
		if r.piece = r.next(); r.piece == nil {
			// Put the encode buffer back into the pool at EOF when it
			// is first encountered. Subsequent calls still return EOF
			// as the error but the buffer is no longer valid.
			if r.buf != nil {
				encbufPool.Put(r.buf)
				r.buf = nil
			}
			return n, io.EOF
		}
		nn := copy(b[n:], r.piece)
		n += nn
		if nn < len(r.piece) {
			// piece didn't fit, see you next time.
			r.piece = r.piece[nn:]
			return n, nil
		}
		r.piece = nil
	}
}

// next returns the next piece of data to be read.
// it returns nil at EOF.
func (r *encReader) next() []byte {
	switch {
	// if there are nothing in the pool
	case r.buf == nil:
		return nil

	// if there are still data remaining in the r.piece
	case r.piece != nil:
		// There is still data available for reading.
		return r.piece

	// if the list header that i currently reading is smaller than total number of list headers
	// simplify: not finished with reading list headers
	case r.lhpos < len(r.buf.lheads):
		// We're before the last list header.
		// get the current list header's information
		head := r.buf.lheads[r.lhpos]
		// data must be filled in first, then the list headers can be placed after
		// purpose of offset
		sizebefore := head.offset - r.strpos
		if sizebefore > 0 {
			// String data before header.
			p := r.buf.str[r.strpos:head.offset]
			r.strpos += sizebefore
			return p
		}
		r.lhpos++
		return head.encode(r.buf.sizebuf)

	// if the reading position is smaller than size of encoded data
	// read all data after
	case r.strpos < len(r.buf.str):
		// String data at the end, after all list headers.
		p := r.buf.str[r.strpos:]
		r.strpos = len(r.buf.str)
		return p

	default:
		return nil
	}
}
