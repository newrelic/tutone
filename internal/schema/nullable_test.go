//go:build unit
// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeRef_IsNullable(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ref  TypeRef
		want bool
	}{
		"scalar":                    {ref(KindScalar, "String"), true},
		"non-null scalar":           {wrap(KindNonNull, ref(KindScalar, "String")), false},
		"list":                      {wrap(KindList, ref(KindScalar, "String")), true},
		"list of non-null":          {wrap(KindList, wrap(KindNonNull, ref(KindScalar, "String"))), true},
		"non-null list of non-null": {wrap(KindNonNull, wrap(KindList, wrap(KindNonNull, ref(KindScalar, "String")))), false},
		"nullable input object":     {ref(KindInputObject, "Filter"), true},
		"non-null input object":     {wrap(KindNonNull, ref(KindInputObject, "Filter")), false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.ref.IsNullable())
		})
	}
}
