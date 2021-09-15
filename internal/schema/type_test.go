//go:build unit
// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestType_GetQueryFieldsString(t *testing.T) {
	t.Parallel()

	// schema cached by 'make test-prep'
	s, err := Load("../../testdata/schema.json")
	require.NoError(t, err)

	cases := map[string]struct {
		TypeName      string
		Depth         int
		Mutation      bool
		ExcludeFields []string
	}{
		"AlertsNrqlCondition": {
			TypeName: "AlertsNrqlCondition",
			Depth:    2,
			Mutation: false,
		},
		"AlertsNrqlBaselineCondition": {
			TypeName: "AlertsNrqlBaselineCondition",
			Depth:    2,
			Mutation: false,
		},
		"AlertsNrqlOutlierCondition": {
			TypeName: "AlertsNrqlOutlierCondition",
			Depth:    2,
			Mutation: false,
		},
		"CloudLinkedAccount": {
			TypeName: "CloudLinkedAccount",
			Depth:    3,
			Mutation: false,
		},
		"CloudDisableIntegrationPayload": {
			TypeName: "CloudDisableIntegrationPayload",
			Depth:    1,
			Mutation: true,
		},
	}

	for n, tc := range cases {
		x, err := s.LookupTypeByName(tc.TypeName)
		require.NoError(t, err)

		xx := x.GetQueryStringFields(s, 0, tc.Depth, tc.Mutation, tc.ExcludeFields)
		// saveFixture(t, n, xx)
		expected := loadFixture(t, n)
		assert.Equal(t, expected, xx)
	}
}
