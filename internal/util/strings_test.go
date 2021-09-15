//go:build unit
// +build unit

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToSnakeCase(t *testing.T) {
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
