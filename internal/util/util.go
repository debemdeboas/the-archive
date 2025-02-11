package util

import (
	"crypto/sha256"
	"encoding/hex"
)

func ContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
