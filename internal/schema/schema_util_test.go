// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tj/assert"

	"github.com/newrelic/tutone/internal/config"
)

func TestExpandTypes(t *testing.T) {
	t.Parallel()

	s, err := Load("../../schema.json")
	assert.NoError(t, err)

	config := []config.TypeConfig{
		{
			Name: "AlertsPolicy",
		},
	}

	results, err := ExpandTypes(s, config)
	assert.NoError(t, err)
	require.NotNil(t, results)
	assert.Equal(t, len(*results), 5)

	expectedNames := []string{
		"AlertsPolicy",
		"ID",
		"Int",
		"AlertsIncidentPreference",
		"String",
	}

	for _, r := range *results {
		hasString := stringInStrings(r.Name, expectedNames)
		assert.True(t, hasString)
	}
}

func stringInStrings(s string, ss []string) bool {
	for _, sss := range ss {
		if s == sss {
			return true
		}
	}

	return false
}
