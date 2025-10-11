package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2 params (reasonable defaults)
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password empty")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	encoded := fmt.Sprintf("$argon2id$v=19$t=%d$m=%d$p=%d$%s$%s", argonTime, argonMemory, argonThreads, b64Salt, b64Hash)
	return encoded, nil
}

func ComparePasswordAndHash(password, encodedHash string) (bool, error) {
	if password == "" || encodedHash == "" {
		return false, errors.New("empty password or hash")
	}

	parts := strings.Split(encodedHash, "$")
	// expected: ["", "argon2id", "v=19", "t=1", "m=65536", "p=4", "salt", "hash"]
	if len(parts) != 8 || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}
	if parts[2] != "v=19" {
		return false, errors.New("unsupported argon2 version")
	}

	t64, err := strconv.ParseUint(strings.TrimPrefix(parts[3], "t="), 10, 32)
	if err != nil {
		return false, err
	}
	m64, err := strconv.ParseUint(strings.TrimPrefix(parts[4], "m="), 10, 32)
	if err != nil {
		return false, err
	}
	p64, err := strconv.ParseUint(strings.TrimPrefix(parts[5], "p="), 10, 8)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[6])
	if err != nil {
		return false, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[7])
	if err != nil {
		return false, err
	}

	computed := argon2.IDKey([]byte(password), salt, uint32(t64), uint32(m64), uint8(p64), uint32(len(hash)))
	if subtle.ConstantTimeCompare(computed, hash) == 1 {
		return true, nil
	}
	return false, nil
}
