package str

import "math/rand/v2"

func RandomString(n int) string {
	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	s := []byte{}
	for range n {
		s = append(s, chars[rand.IntN(len(chars))]) //nolint:gosec
	}
	return string(s)
}
