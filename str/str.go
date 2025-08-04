package str

import "math/rand"

func RandomString(n int) string {
	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	s := []byte{}
	for i := 0; i < n; i++ {
		s = append(s, chars[rand.Intn(len(chars))]) // nolint:gosec
	}
	return string(s)
}
