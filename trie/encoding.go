package trie

// Trie keys are dealt with in three distinct encodings:
//
// KEYBYTES encoding contains the actual key and nothing else. This encoding is the
// input to most API functions.
//
// HEX encoding (char 0x00-0x0f, 0x10) contains one byte for each nibble of the key and
// an optional trailing 'terminator' byte of value 0x10 which indicates whether or not
// contains a value. Hex key encoding is used for nodes loaded in memory because it's
// convenient to access.
//
// COMPACT encoding (char 0x00-0x3f for first char, 0x00-0xff for the rest) is identical
// to the concepts in the Ethereum Yellow Paper (it's called "hex prefix encoding" there)
// and contains the bytes of the key and a flag. The high nibble of the first byte
// contains the flag; the lowest bit encoding the oddness of the length and the second-
// lowest encoding whether the node at the key is a value node. The low nibble of the
// first byte is zero in the case of an even number of nibbles and the first nibble in
// the case of an odd number. All remaining nibbles (now an even number) fit properly
// into the remaining bytes. Compact encoding is used for nodes stored on disk.

// raw data   	 memory 	 dxdb
// KeyBytes <->   Hex   <-> Compact

// Method included in this file:
// hexToCompact:		Hex 		--> Compact
// compactToHex:		Compact 	--> Hex
// keybytesToHex: 		KeyBytes 	--> Hex
// hexToKeybytes:		Hex 		--> KeyBytes

// hexToCompact encode the Hex byte to compact byte, return the compact byte.
// Hex: each byte is 0x00 - 0x15, 0x16 denote the end
// Compact: HP encoding of the hex byte
func hexToCompact(hex []byte) []byte {
	terminator := byte(0)
	if hasTerm(hex) {
		// The hex having terminator indicates it has a value, flag set to 1
		terminator = 1
		hex = hex[:len(hex)-1]
	}
	buf := make([]byte, len(hex)/2+1)
	// If flag is 1, the second lowest bit in the first nibble in the HP result is 1
	buf[0] = terminator << 5
	if len(hex)&1 == 1 {
		buf[0] |= 1 << 4 // length of hex is odd, last bit in first nibble set to 1
		buf[0] |= hex[0] // The low nibble of the byte is first nibble of the hex
		hex = hex[1:]    // hex now skip the first byte
	}
	decodeNibbles(hex, buf[1:])
	return buf
}

// compactToHex will decode the input compact bytes to hex bytes
func compactToHex(compact []byte) []byte {
	base := keybytesToHex(compact)
	// If the first nibble is smaller than 2, meaning flag is false, remove the last
	// byte which should be terminator 16.
	if base[0] < 2 {
		base = base[:len(base)-1]
	}
	// Remove the additional nibble added for even length hex.
	// 2 nibbles needs to be removed for even length hex
	// 1 nibbles needs to be removed for odd length hex
	chop := 2 - base[0]&1
	return base[chop:]
}

// keybytesToHex takes a string of bytes and format it to hex. Returned bytes always
// end with terminator 16.
// e.g. [21, 2f] -> [2, 1, 2, f, 16]
func keybytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}

// hexToKeybytes decode the hex nibbles to keybytes
// This can only be used for keys of even length (without terminator).
func hexToKeybytes(hex []byte) []byte {
	if hasTerm(hex) {
		hex = hex[:len(hex)-1]
	}
	if len(hex)&1 != 0 {
		panic("can't convert hex key of odd length")
	}
	key := make([]byte, len(hex)/2)
	decodeNibbles(hex, key)
	return key
}

// hasTerm returns whether a hex key has the terminator flag 0x10 (16).
// The terminator flag indicates whether the key has a value.
func hasTerm(s []byte) bool {
	return len(s) > 0 && s[len(s)-1] == 16
}

// decodeNibbles takes a even length hex nibbles (value 0-15 for each) nibble, and
// write it in 8 bit to another input bytes. The input nibbles should have exactly
// double length of bytes.
func decodeNibbles(nibbles []byte, bytes []byte) {
	for bi, ni := 0, 0; ni < len(nibbles); bi, ni = bi+1, ni+2 {
		bytes[bi] = nibbles[ni]<<4 | nibbles[ni+1]
	}
}

// prefixLen returns the length of the common prefix of a and b.
func prefixLen(a, b []byte) int {
	i, length := 0, len(a)
	if len(b) < length {
		length = len(b)
	}
	for ; i < length; i++ {
		if a[i] != b[i] {
			break
		}
	}
	return i
}
