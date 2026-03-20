package bender

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMissingBindingErrorMessage(t *testing.T) {
	err := MissingBindingError{
		Key: keyOfType(typeOf[int](), "main"),
		Path: []Key{
			keyOfType(typeOf[string](), ""),
			keyOfType(typeOf[bool](), "x"),
		},
	}

	msg := err.Error()
	assert.Contains(t, msg, "missing binding")
	assert.Contains(t, msg, "resolution path")
}

func TestCycleErrorMessage(t *testing.T) {
	err := CycleError{Cycle: []Key{
		keyOfType(typeOf[int](), ""),
		keyOfType(typeOf[string](), "dep"),
	}}

	msg := err.Error()
	assert.Contains(t, msg, "dependency cycle")
	assert.Contains(t, msg, "int")
	assert.Contains(t, msg, "string")
}
