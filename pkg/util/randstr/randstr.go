package randstr

import (
	"crypto/rand"
	"encoding/hex"
)

// Bytes generates n random bytes
func Bytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Hex generates a random hex string with length of n
func Hex(n int) string { return hex.EncodeToString(Bytes(n)) }
