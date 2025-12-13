package neogate

import "reflect"

// isTypeNone returns whether T is type None
func isTypeNone[T any](t T) bool {
	return reflect.TypeOf(t) == reflect.TypeOf(None{})
}
