package store

import "crypto/rand"

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[b[i]%byte(len(alphabet))]
	}
	return string(b)
}
