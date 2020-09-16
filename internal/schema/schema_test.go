// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
)

func TestTypeQueryFields(t *testing.T) {
	t.Parallel()

	// schema cached by 'make test-prep'
	s, err := Load("../../testdata/schema.json")
	assert.NoError(t, err)

	cases := map[string]struct {
		TypeName string
		Result   string
	}{
		"AlertsNrqlCondition": {
			TypeName: "AlertsNrqlCondition",
			Result: alertsNrqlCondition + `
... on AlertsNrqlBaselineCondition {
` + PrefixLineTab(alertsNrqlBaselineCondition) + `
}
... on AlertsNrqlOutlierCondition {
` + PrefixLineTab(alertsNrqlOutlierCondition) + `
}
... on AlertsNrqlStaticCondition {
` + PrefixLineTab(alertsNrqlStaticCondition) + `
}`,
		},
		"AlertsNrqlBaselineCondition": {
			TypeName: "AlertsNrqlBaselineCondition",
			Result:   alertsNrqlBaselineCondition,
		},
		"AlertsNrqlOutlierCondition": {
			TypeName: "AlertsNrqlOutlierCondition",
			Result:   alertsNrqlOutlierCondition,
		},
		"AlertsNrqlStaticCondition": {
			TypeName: "AlertsNrqlStaticCondition",
			Result:   alertsNrqlStaticCondition,
		},
	}

	for _, tc := range cases {
		x, err := s.LookupTypeByName(tc.TypeName)
		require.NoError(t, err)

		xx := s.QueryFields(x)
		assert.Equal(t, tc.Result, xx)
	}

}

var (
	alertsNrqlCondition = `description
enabled
expiration {
	closeViolationsOnExpiration
	expirationDuration
	openViolationOnExpiration
}
id
name
nrql {
	evaluationOffset
	query
}
policyId
runbookUrl
signal {
	evaluationOffset
	fillOption
	fillValue
}
terms {
	operator
	priority
	threshold
	thresholdDuration
	thresholdOccurrences
}
type
violationTimeLimit`

	alertsNrqlBaselineCondition = `baselineDirection
description
enabled
expiration {
	closeViolationsOnExpiration
	expirationDuration
	openViolationOnExpiration
}
id
name
nrql {
	evaluationOffset
	query
}
policyId
runbookUrl
signal {
	evaluationOffset
	fillOption
	fillValue
}
terms {
	operator
	priority
	threshold
	thresholdDuration
	thresholdOccurrences
}
type
violationTimeLimit`

	alertsNrqlOutlierCondition = `description
enabled
expectedGroups
expiration {
	closeViolationsOnExpiration
	expirationDuration
	openViolationOnExpiration
}
id
name
nrql {
	evaluationOffset
	query
}
openViolationOnGroupOverlap
policyId
runbookUrl
signal {
	evaluationOffset
	fillOption
	fillValue
}
terms {
	operator
	priority
	threshold
	thresholdDuration
	thresholdOccurrences
}
type
violationTimeLimit`

	alertsNrqlStaticCondition = `description
enabled
expiration {
	closeViolationsOnExpiration
	expirationDuration
	openViolationOnExpiration
}
id
name
nrql {
	evaluationOffset
	query
}
policyId
runbookUrl
signal {
	evaluationOffset
	fillOption
	fillValue
}
terms {
	operator
	priority
	threshold
	thresholdDuration
	thresholdOccurrences
}
type
valueFunction
violationTimeLimit`
)
