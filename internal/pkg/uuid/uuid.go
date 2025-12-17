// Package uuid provides UUID v7 generation utilities
package uuid

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// GenerateUUIDv7 generates a new UUID version 7
// UUIDv7 is time-ordered and sortable
func GenerateUUIDv7() string {
	var uuid [16]byte

	// Get current timestamp in milliseconds since Unix epoch
	now := time.Now().UnixMilli()

	// First 6 bytes: timestamp (48 bits)
	binary.BigEndian.PutUint64(uuid[0:8], uint64(now)<<16)

	// Next 2 bytes: version (7) and 12 bits of rand_a
	rand.Read(uuid[6:8])
	uuid[6] = (uuid[6] & 0x0f) | 0x70 // Set version to 7

	// Last 8 bytes: 62 bits of randomness
	rand.Read(uuid[8:])

	// Set variant to RFC 4122
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	// Format as standard UUID string
	return stringify(uuid)
}

func stringify(uuid [16]byte) string {
	var buf [36]byte
	hexEncode(buf[0:8], uuid[0:4])
	buf[8] = '-'
	hexEncode(buf[9:13], uuid[4:6])
	buf[13] = '-'
	hexEncode(buf[14:18], uuid[6:8])
	buf[18] = '-'
	hexEncode(buf[19:23], uuid[8:10])
	buf[23] = '-'
	hexEncode(buf[24:], uuid[10:])
	return string(buf[:])
}

func hexEncode(dst, src []byte) {
	const hextable = "0123456789abcdef"
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
}
