//go:build unit
// +build unit

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToSnakeCase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Input    string
		Expected string
	}{
		{"", ""},
		{"already_snake", "already_snake"},
		{"A", "a"},
		{"AA", "aa"},
		{"AaAa", "aa_aa"},
		{"HTTPRequest", "http_request"},
		{"BatteryLifeValue", "battery_life_value"},
		{"Id0Value", "id0_value"},
		{"ID0Value", "id0_value"},
	}
	for _, c := range cases {
		result := ToSnakeCase(c.Input)

		require.Equal(t, c.Expected, result)
	}
}

func TestStringInStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Str      string
		Arry     []string
		Expected bool
	}{
		{"foo", []string{}, false},
		{"foo", []string{"foo"}, true},
		{"foo", []string{"foo", "bar"}, true},
		{"bar", []string{"foo", "bar"}, true},
		{"baz", []string{"foo", "bar"}, false},
		{"", []string{"foo", "bar", "baz"}, false},
	}

	for x := range cases {
		result := StringInStrings(cases[x].Str, cases[x].Arry)
		assert.Equal(t, cases[x].Expected, result)
	}
}
