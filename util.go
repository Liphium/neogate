package neogate

import (
	"crypto/rand"
	"math/big"
)

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

type AdapterSendError struct {
	AdapterErrors map[string]error // adapterId -> error
}

func (err *AdapterSendError) Error() string {
	return "some adapters failed to send, convert to AdapterSendError to see specifics"
}
