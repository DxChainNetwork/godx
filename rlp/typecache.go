/*
Valid Tail:
type tailRaw struct {
	A    uint
	Tail []RawValue `rlp:"tail"`
}

Invalid Tail:
type invalidTail2 struct {
	A uint
	B string `rlp:"tail"`
}

Why fields must be exported:
	it is a method to check if the field is public or private
	private fields cannot be directly encoded, which will be ignored

*/

package rlp

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

var (
	typeCacheMutex sync.RWMutex                  // read write mutex that is used to protect typeCache while doing multiprocessing
	typeCache      = make(map[typekey]*typeinfo) // core data structure !!!!!
	// Mapped data type with decoder and encoder needed for this data type
)

type typeinfo struct {
	decoder // decoder used to decode data
	writer  // encoder used to encode data
}

type typekey struct {
	reflect.Type // type of the data
	tags         // special characters of a data
}

// NOTE: those are structure tags
type tags struct {
	nilOK bool // if empty input will result in nil pointer
	tail  bool // in a structure, if the last filed is a slice/array, then the tail can be true
	// the tag is defined in the structure passed in to the function. examples are provided in the beginning
	// of this file
	ignored bool // if a field has ignored at the end, it means this field do not need to be encoded
}

// used to store data after parsing the structure
type field struct {
	index int       // index of the field in the structure
	info  *typeinfo // decoder&writer used for this field
}

// Any function that is satisfied this requirement (same structure), will be type decoder or writer
// Purpose of first class function: functions can be stored into typeinfo structure. Example:
type decoder func(*Stream, reflect.Value) error
type writer func(reflect.Value, *encbuf) error

// Based on the Type and tags of data, return the decoder and encoder used for this data type
// if the decoder and encoder cannot be found for a certain type key, generate one
func cachedTypeInfo(typ reflect.Type, tags tags) (*typeinfo, error) {
	typeCacheMutex.RLock()
	info := typeCache[typekey{typ, tags}]
	typeCacheMutex.RUnlock()
	if info != nil {
		return info, nil
	}

	// Before generating typeinfo, another goroutine may already started to generate the typeinfo
	// Lock it for protection
	typeCacheMutex.Lock()

	// Unlock it after generated typeinfo corresponding to the type key, and return the typeinfo
	defer typeCacheMutex.Unlock()
	return cachedTypeInfo1(typ, tags)
}

func cachedTypeInfo1(typ reflect.Type, tags tags) (*typeinfo, error) {
	key := typekey{typ, tags}
	info := typeCache[key]

	// another goroutine may already generated typeinfo
	if info != nil {
		return info, nil
	}

	// initialize an empty typeinfo for this typekey, prevent recursive call (ex: encode.go, makeSliceWriter)
	typeCache[key] = new(typeinfo)
	info, err := genTypeInfo(typ, tags)

	// if failed to generate typeinfo, remove the empty typeinfo from the map
	if err != nil {
		delete(typeCache, key)
		return nil, err
	}

	*typeCache[key] = *info
	return typeCache[key], err
}

// calls makeDecoder and makeWriter function to generate encoder and decoder for the specific data type
func genTypeInfo(typ reflect.Type, tags tags) (info *typeinfo, err error) {
	info = new(typeinfo)
	if info.decoder, err = makeDecoder(typ, tags); err != nil {
		return nil, err
	}
	if info.writer, err = makeWriter(typ, tags); err != nil {
		return nil, err
	}
	return info, nil
}

// Get the tag from the structure, and assign proper value to them
// If found, check condition, then assign it to be true
func parseStructTag(typ reflect.Type, fi int) (tags, error) {
	// fi = field index
	f := typ.Field(fi) // returns i'th field of the structure
	var ts tags
	// there may multiple tags which are separated by comma
	for _, t := range strings.Split(f.Tag.Get("rlp"), ",") {
		switch t = strings.TrimSpace(t); t {
		case "":
		case "-":
			ts.ignored = true
		case "nil":
			ts.nilOK = true
		case "tail":
			ts.tail = true
			// Check if the filed with the tail tag is in the last position
			if fi != typ.NumField()-1 {
				return ts, fmt.Errorf(`rlp: invalid struct tag "tail" for %v.%s (must be on last field)`, typ, f.Name)
			}
			// Check if the filed with the tail tag is Slice Kind
			if f.Type.Kind() != reflect.Slice {
				return ts, fmt.Errorf(`rlp: invalid struct tag "tail" for %v.%s (field type is not slice)`, typ, f.Name)
			}
		default:
			return ts, fmt.Errorf("rlp: unknown struct tag %q on %v.%s", t, typ, f.Name)
		}
	}
	return ts, nil
}

// parse the structure, get each field's type, and assign writer&decoder to each type
// all information are stored inside field structure
func structFields(typ reflect.Type) (fields []field, err error) {
	// typ.NumField returns number of fields in the structure
	for i := 0; i < typ.NumField(); i++ {
		// PkgPath will be empty for exported field names (empty)
		if f := typ.Field(i); f.PkgPath == "" { // exported
			f := typ.Field(i)
			tags, err := parseStructTag(typ, i)
			if err != nil {
				return nil, err
			}
			//	// for field with ignored as tag, it means this field does not need to be encoded
			if tags.ignored {
				continue
			}
			// get the writer and decoder based on the field type
			info, err := cachedTypeInfo1(f.Type, tags)
			if err != nil {
				return nil, err
			}
			// stores field index and corresponding type encoder and decoder
			fields = append(fields, field{i, info})
		}
	}
	return fields, nil
}

// why using this comparison:
// all Uint type, including Uint, Uint32, Uint8, Uint63, Uintptr, are counted as uint kind
// NOTE: uintptr is not a pointer, it is an integer type that is large enough to hold the bit pattern of any pointer
func isUint(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}
