package neogate

import (
	"crypto/rand"
	"math/big"
	"reflect"
)

// isTypeNone returns whether T is type None
func isTypeNone[T any](t T) bool {
	return reflect.TypeOf(t) == reflect.TypeOf(None{})
}

func GenerateToken(tkLength int32) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, tkLength)

	length := big.NewInt(int64(len(letters)))

	for i := range s {

		number, _ := rand.Int(rand.Reader, length)
		s[i] = letters[number.Int64()]
	}

	return string(s)
}
