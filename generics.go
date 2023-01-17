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

func GetSliceWithoutDuplicates[T comparable](values []T) []T {
	valuesAsMap := map[T]bool{}
	return getSliceWithoutMatchingValues(values, func(value T) bool {
		if _, keyExists := valuesAsMap[value]; !keyExists {
			valuesAsMap[value] = true
			return false
		}
		return true
	})
}
