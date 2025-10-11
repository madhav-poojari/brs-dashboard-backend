package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const base36Alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// GenerateUserID returns USR00 + 5 base36 chars
func GenerateUserID() (string, error) {
	const suffixLen = 5
	// generate a random integer between 0 and 36^5 - 1
	max := big.NewInt(0).Exp(big.NewInt(36), big.NewInt(suffixLen), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	// convert to base36 string padded to suffixLen
	s := ""
	for i := 0; i < suffixLen; i++ {
		rem := new(big.Int)
		n.DivMod(n, big.NewInt(36), rem)
		s = string(base36Alphabet[int(rem.Int64())]) + s
	}
	return fmt.Sprintf("USR00%s", s), nil
}
