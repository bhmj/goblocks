package str

import "math/rand/v2"

func RandomString(n int) string {
	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	s := make([]byte, n)
	for i := range s {
		s[i] = chars[rand.IntN(len(chars))] //nolint:gosec
	}
	return string(s)
}
