package generics

import "reflect"

func isZero[T any](value T) bool {
	return reflect.ValueOf(&value).Elem().IsZero()
}

func getSliceWithoutMatchingValues[T any](values []T, shouldBeDeleted func(T) bool) []T {
	result := []T{}
	for i, value := range values {
		if !shouldBeDeleted(value) {
			result = append(result, values[i])
		}
	}
	return result
}

func GetSliceWithoutZeroValues[T any](values []T) []T {
	return getSliceWithoutMatchingValues(values, isZero[T])
}
