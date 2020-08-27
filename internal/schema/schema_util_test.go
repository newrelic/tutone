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

	// schema cached by 'make test-prep'
	s, err := Load("../../testdata/schema.json")
	assert.NoError(t, err)

	typeConfig := []config.TypeConfig{
		{
			Name: "AlertsPolicy",
		},
	}

	methodConfig := []config.MethodConfig{}

	results, err := ExpandTypes(s, typeConfig, methodConfig)
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
