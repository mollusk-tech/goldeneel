package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

var defaultParams = Argon2Params{Time: 1, Memory: 64 * 1024, Threads: 4, KeyLen: 32}

func generateSalt(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func HashPassword(plain string) (string, error) {
	salt, err := generateSalt(16)
	if err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, defaultParams.Time, defaultParams.Memory, defaultParams.Threads, defaultParams.KeyLen)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", defaultParams.Memory, defaultParams.Time, defaultParams.Threads, hex.EncodeToString(salt), hex.EncodeToString(key)), nil
}

func VerifyPassword(encoded, plain string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return false, errors.New("invalid hash format")
	}
	alg := parts[0]
	if alg != "argon2id" {
		return false, errors.New("unsupported algorithm")
	}
	// parts[1] is v=19 (ignored)
	params := parts[2]
	saltHex := parts[3]
	keyHex := parts[4]

	var m uint32
	var t uint32
	var p uint8
	_, err := fmt.Sscanf(params, "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return false, err
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false, err
	}
	expected, err := hex.DecodeString(keyHex)
	if err != nil {
		return false, err
	}
	key := argon2.IDKey([]byte(plain), salt, t, m, p, uint32(len(expected)))
	if len(key) != len(expected) {
		return false, nil
	}
	var ok = true
	for i := range key {
		if key[i] != expected[i] {
			ok = false
		}
	}
	return ok, nil
}