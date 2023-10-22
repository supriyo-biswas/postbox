package utils

import (
	"crypto/rand"
	"encoding/base64"
)

func RandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", nil
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
