// Package hash provides hashing utilities.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256 computes the SHA256 hash of data and returns it as a hex string.
func SHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA256String computes the SHA256 hash of a string.
func SHA256String(s string) string {
	return SHA256([]byte(s))
}

// SHA256Short returns the first n characters of a SHA256 hash.
func SHA256Short(data []byte, n int) string {
	h := SHA256(data)
	if n > len(h) {
		return h
	}
	return h[:n]
}

// ChunkID generates a deterministic chunk ID from path, start, and end.
func ChunkID(path string, start, end int) string {
	data := []byte(path + ":" + string(rune(start)) + ":" + string(rune(end)))
	return SHA256Short(data, 16)
}

// DocumentID generates a deterministic document ID from path and content hash.
func DocumentID(path, contentHash string) string {
	data := []byte(path + ":" + contentHash)
	return SHA256Short([]byte(data), 16)
}
