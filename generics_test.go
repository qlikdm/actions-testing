package generics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoDuplicateValues(t *testing.T) {
	actual := GetSliceWithoutDuplicates([]string{"one", "two", "three"})
	expected := []string{"one", "two", "three"}
	assert.Equal(t, actual, expected)
}

func TestSkipDuplicateValues(t *testing.T) {
	actual := GetSliceWithoutDuplicates([]string{"one", "two", "one", "one", "three"})
	expected := []string{"one", "two", "three"}
	assert.Equal(t, actual, expected)
}

func TestNoZeroValues(t *testing.T) {
	actual := GetSliceWithoutZeroValues([]int{1, 2, 2, -1, 4})
	expected := []int{1, 2, 2, -1, 4}
	assert.Equal(t, actual, expected)
}

func TestSkipZeroValues(t *testing.T) {
	actual := GetSliceWithoutZeroValues([]int{1, 2, 0, 2, -1, 0, 4})
	expected := []int{1, 2, 2, -1, 4}
	assert.Equal(t, actual, expected)
}

func TestSkipZeroValuedStrings(t *testing.T) {
	actual := GetSliceWithoutZeroValues([]string{"one", "", "two", "two", "", "three"})
	expected := []string{"one", "two", "two", "three"}
	assert.Equal(t, actual, expected)
}
