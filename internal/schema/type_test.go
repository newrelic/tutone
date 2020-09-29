// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
)

func TestType_GetQueryFieldsString(t *testing.T) {
	t.Parallel()

	// schema cached by 'make test-prep'
	s, err := Load("../../testdata/schema.json")
	require.NoError(t, err)

	cases := map[string]struct {
		TypeName string
		Depth    int
	}{
		"AlertsNrqlCondition": {
			TypeName: "AlertsNrqlCondition",
			Depth:    2,
		},
		"AlertsNrqlBaselineCondition": {
			TypeName: "AlertsNrqlBaselineCondition",
			Depth:    2,
		},
		"AlertsNrqlOutlierCondition": {
			TypeName: "AlertsNrqlOutlierCondition",
			Depth:    2,
		},
		"CloudLinkedAccount": {
			TypeName: "CloudLinkedAccount",
			Depth:    3,
		},
	}

	for n, tc := range cases {
		x, err := s.LookupTypeByName(tc.TypeName)
		require.NoError(t, err)

		xx := x.GetQueryStringFields(s, 0, tc.Depth)
		// saveFixture(t, n, xx)
		expected := loadFixture(t, n)
		assert.Equal(t, expected, xx)
	}
}
