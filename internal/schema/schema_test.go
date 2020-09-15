// +build unit

package schema

import (
	"fmt"
	"strings"
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
		// Methods       []config.MethodConfig
		// ExpectErr     bool
		// ExpectReason  string
		// ExpectedNames []string
	}{
		"AlertsNrqlCondition": {
			TypeName: "AlertsNrqlCondition",
			Result: alertsNrqlCondition + `
... on AlertsNrqlBaselineCondition {
` + prefixLineTab(alertsNrqlBaselineCondition) + `
}
... on AlertsNrqlOutlierCondition {
` + prefixLineTab(alertsNrqlOutlierCondition) + `
}
... on AlertsNrqlStaticCondition {
` + prefixLineTab(alertsNrqlStaticCondition) + `
}`,
		},
		// "AlertsNrqlConditionsSearchResultSet": {
		// 	TypeName: "AlertsNrqlConditionsSearchResultSet",
		// 	Result:   "yes",
		// },
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

// Add a \t character to the beginning of each line.
func prefixLineTab(s string) string {
	var lines []string

	for _, t := range strings.Split(s, "\n") {
		lines = append(lines, fmt.Sprintf("\t%s", t))
	}

	return strings.Join(lines, "\n")
}
