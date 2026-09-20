package check

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// crockford is base32 without I, L, O, and U, so a transcribed identifier
// cannot turn into a different one.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID returns one identifier: a 48-bit millisecond timestamp followed by 80
// random bits, in Crockford base32.
//
// Documents are cited by this value for the life of the repository, so the
// random half comes from crypto/rand rather than from a sequence a second
// process could repeat.
func ULID() (string, error) {
	var raw [16]byte
	binary.BigEndian.PutUint64(raw[:8], uint64(time.Now().UnixMilli())<<16)
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", err
	}

	hi := binary.BigEndian.Uint64(raw[:8])
	lo := binary.BigEndian.Uint64(raw[8:])
	out := make([]byte, 26)
	for i := range out {
		out[i] = crockford[quintet(hi, lo, uint(125-5*i))]
	}
	return string(out), nil
}

// quintet returns the five bits of a 128-bit value at shift, counting from the
// low bit. The first character of a ULID reaches past bit 127, where the value
// has no bits and reads as zero.
func quintet(hi, lo uint64, shift uint) byte {
	var v uint64
	switch {
	case shift >= 64:
		v = hi >> (shift - 64)
	case shift+5 <= 64:
		v = lo >> shift
	default:
		v = (lo >> shift) | (hi << (64 - shift))
	}
	return byte(v & 0x1f)
}
