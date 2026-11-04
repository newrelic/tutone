//go:build unit
// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/tutone/internal/config"
)

func TestSchema_GetQueryStringForMutation_ExcludeArguments(t *testing.T) {
	t.Parallel()

	scalar := func(name string) TypeRef { return TypeRef{Kind: KindScalar, Name: name} }

	payload := &Type{
		Name:   "DeletePayload",
		Kind:   KindObject,
		Fields: []Field{{Name: "id", Type: scalar("ID")}},
	}

	mutation := &Field{
		Name: "entityManagementDelete",
		Type: TypeRef{Kind: KindObject, Name: "DeletePayload"},
		Args: []Field{
			{Name: "id", Type: scalar("ID")},
			{Name: "version", Type: scalar("Int")},
		},
	}

	s := &Schema{Types: []*Type{payload}}

	withAll := s.GetQueryStringForMutation(mutation, config.MutationConfig{Name: mutation.Name, MaxQueryFieldDepth: 1})
	assert.Contains(t, withAll, "$version: Int")
	assert.Contains(t, withAll, "version: $version")

	left := s.GetQueryStringForMutation(mutation, config.MutationConfig{
		Name:               mutation.Name,
		MaxQueryFieldDepth: 1,
		ExcludeArguments:   []string{"version"},
	})
	assert.NotContains(t, left, "version")
	assert.Contains(t, left, "$id: ID")
	assert.Contains(t, left, "id: $id")
}

func TestSchema_GetQueryStringForEndpoint_ExcludeArguments(t *testing.T) {
	t.Parallel()

	scalar := func(name string) TypeRef { return TypeRef{Kind: KindScalar, Name: name} }

	root := &Type{Name: "RootQueryType", Kind: KindObject, Fields: []Field{{Name: "accounts", Type: TypeRef{Kind: KindObject, Name: "Accounts"}}}}
	accounts := &Type{Name: "Accounts", Kind: KindObject, Fields: []Field{{
		Name: "managedAccounts",
		Type: TypeRef{Kind: KindObject, Name: "Managed"},
		Args: []Field{{Name: "isCanceled", Type: scalar("Boolean")}},
	}}}
	managed := &Type{Name: "Managed", Kind: KindObject, Fields: []Field{{Name: "id", Type: scalar("Int")}}}

	s := &Schema{Types: []*Type{root, accounts, managed}}
	path := []*Type{accounts}

	base := config.EndpointConfig{Name: "managedAccounts", MaxQueryFieldDepth: 1, IncludeArguments: []string{"isCanceled"}}
	assert.Contains(t, s.GetQueryStringForEndpoint(path, []string{"accounts"}, base), "isCanceled")

	excluded := base
	excluded.ExcludeArguments = []string{"isCanceled"}
	assert.NotContains(t, s.GetQueryStringForEndpoint(path, []string{"accounts"}, excluded), "isCanceled")
}
