package utils

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"sync"

	"lukechampine.com/blake3"
)

var ErrInvalidSecretAlg = errors.New("invalid secret algorithm")

const (
	ALGO_BLAKE3 = "$b3$"
)

var blake3Pool = sync.Pool{
	New: func() interface{} {
		return blake3.New(32, nil)
	},
}

func HashSecret(input string) string {
	h := blake3Pool.Get().(*blake3.Hasher)
	h.Reset()
	defer blake3Pool.Put(h)

	h.Write([]byte(input))
	return ALGO_BLAKE3 + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func VerifySecret(input string, hash string) (bool, error) {
	if !strings.HasPrefix(hash, ALGO_BLAKE3) {
		return false, ErrInvalidSecretAlg
	}

	h := blake3Pool.Get().(*blake3.Hasher)
	h.Reset()
	defer blake3Pool.Put(h)

	h.Write([]byte(input))
	d, err := base64.StdEncoding.DecodeString(hash[len(ALGO_BLAKE3):])
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare(h.Sum(nil), d) == 1, nil
}
