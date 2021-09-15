//go:build unit
// +build unit

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplateFuncs(t *testing.T) {
	t.Parallel()

	// List of custom funcs we add
	customFuncs := []string{
		"hasField",
	}

	tf := GetTemplateFuncs()

	for _, x := range customFuncs {
		assert.Contains(t, tf, x)
	}
}

func TestHasField(t *testing.T) {
	t.Parallel()

	// Make an object with fields
	testStruct := struct {
		name string
	}{
		"some name",
	}

	// Use that object here
	cases := []struct {
		object    interface{}
		fieldName string
		result    bool
	}{
		{testStruct, "name", true},
		{testStruct, "foo", false},
		// With pointers
		{&testStruct, "name", true},
		{&testStruct, "foo", false},
		// Non-structs
		{"string", "name", false},
		{42, "name", false},
		{false, "name", false},
	}

	for x := range cases {
		ret := hasField(cases[x].object, cases[x].fieldName)
		assert.Equal(t, cases[x].result, ret)
	}
}
