// +build unit

package schema

import (
	"sort"
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

	cases := map[string]struct {
		Types         []config.TypeConfig
		Methods       []config.MethodConfig
		ExpectErr     bool
		ExpectReason  string
		ExpectedNames []string
	}{
		"simple": {
			Types: []config.TypeConfig{{
				Name: "AlertsPolicy",
			}},
			Methods: []config.MethodConfig{},
			ExpectedNames: []string{
				"AlertsPolicy",
				"ID",
				"Int",
				"AlertsIncidentPreference",
				"String",
			},
		},
	}

	for _, tc := range cases {
		results, err := ExpandTypes(s, tc.Types, tc.Methods)
		if tc.ExpectErr {
			require.NotNil(t, err)
			require.Equal(t, err.Error(), tc.ExpectReason)
		} else {
			require.Nil(t, err)
		}

		assert.Equal(t, len(*results), len(tc.ExpectedNames))

		names := []string{}
		for _, r := range *results {
			names = append(names, r.Name)
		}

		sort.Strings(names)
		sort.Strings(tc.ExpectedNames)

		assert.Equal(t, tc.ExpectedNames, names)
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
