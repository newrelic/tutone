//go:build unit
// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
)

func ref(kind Kind, name string) TypeRef { return TypeRef{Kind: kind, Name: name} }

func wrap(kind Kind, of TypeRef) TypeRef { return TypeRef{Kind: kind, OfType: &of} }

func TestTypeRef_IsLeaf(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ref  TypeRef
		want bool
	}{
		"scalar":                     {ref(KindScalar, "String"), true},
		"enum":                       {ref(KindENUM, "Status"), true},
		"non-null scalar":            {wrap(KindNonNull, ref(KindScalar, "Boolean")), true},
		"list of scalars":            {wrap(KindList, ref(KindScalar, "ID")), true},
		"non-null list of enums":     {wrap(KindNonNull, wrap(KindList, wrap(KindNonNull, ref(KindENUM, "Status")))), true},
		"object":                     {ref(KindObject, "Payload"), false},
		"non-null object":            {wrap(KindNonNull, ref(KindObject, "Payload")), false},
		"list of objects":            {wrap(KindList, ref(KindObject, "Payload")), false},
		"interface":                  {ref(KindInterface, "Entity"), false},
		"list of interfaces":         {wrap(KindList, ref(KindInterface, "Entity")), false},
		"input object":               {ref(KindInputObject, "Input"), false},
		"a reference without a kind": {TypeRef{}, false},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.ref.IsLeaf())
		})
	}
}

func TestTypeRef_GetGoTypeName(t *testing.T) {
	t.Parallel()

	goName := func(r TypeRef) string { return r.GetGoTypeName() }

	assert.Equal(t, "AiNotificationsOAuthURLResponse", goName(ref(KindObject, "AiNotificationsOAuthUrlResponse")))
	assert.Equal(t, "NRDBResultContainer", goName(wrap(KindList, wrap(KindNonNull, ref(KindObject, "NrdbResultContainer")))), "wrappers are looked through")
	assert.Equal(t, "Entity", goName(ref(KindInterface, "Entity")))
}

// leafSchema has an object, scalars and an enum reachable from actor and from actor.things.
func leafSchema() *Schema {
	payload := &Type{Name: "Payload", Kind: KindObject, Fields: []Field{{Name: "id", Type: ref(KindScalar, "ID")}}}

	return &Schema{Types: []*Type{
		{Name: "RootQueryType", Kind: KindObject, Fields: []Field{{Name: "actor", Type: ref(KindObject, "Actor")}}},
		{Name: "Actor", Kind: KindObject, Fields: []Field{
			{Name: "things", Type: ref(KindObject, "Things")},
			{Name: "version", Type: ref(KindScalar, "String")},
		}},
		{Name: "Things", Kind: KindObject, Fields: []Field{
			{Name: "payload", Type: ref(KindObject, "Payload")},
			{Name: "health", Type: ref(KindScalar, "String")},
			{Name: "state", Type: wrap(KindNonNull, ref(KindENUM, "State"))},
			{Name: "token", Type: ref(KindScalar, "Secret"), Args: []Field{
				{Name: "scope", Type: wrap(KindNonNull, ref(KindScalar, "String"))},
			}},
		}},
		payload,
		{Name: "State", Kind: KindENUM},
		{Name: "String", Kind: KindScalar},
		{Name: "Secret", Kind: KindScalar},
		{Name: "ID", Kind: KindScalar},
	}}
}

func TestSchema_GetQueryStringForMutation_ShapeFollowsArgumentsAndReturnType(t *testing.T) {
	t.Parallel()

	s := leafSchema()
	id := wrap(KindNonNull, ref(KindScalar, "ID"))

	cases := map[string]struct {
		mutation Field
		want     string
	}{
		"object with arguments": {
			Field{Name: "thingCreate", Type: ref(KindObject, "Payload"), Args: []Field{{Name: "id", Type: id}}},
			"mutation(\n\t$id: ID!,\n) { thingCreate(\n\tid: $id,\n) {\n\tid\n} }",
		},
		"object without arguments has no empty parentheses": {
			Field{Name: "thingRegister", Type: ref(KindObject, "Payload")},
			"mutation { thingRegister {\n\tid\n} }",
		},
		"scalar with arguments has no selection": {
			Field{Name: "thingDelete", Type: ref(KindScalar, "String"), Args: []Field{{Name: "id", Type: id}}},
			"mutation(\n\t$id: ID!,\n) { thingDelete(\n\tid: $id,\n) }",
		},
		"scalar without arguments": {
			Field{Name: "thingPing", Type: wrap(KindNonNull, ref(KindScalar, "Boolean"))},
			"mutation { thingPing }",
		},
		"list of IDs": {
			Field{Name: "thingAdd", Type: wrap(KindList, ref(KindScalar, "ID")), Args: []Field{{Name: "ids", Type: wrap(KindNonNull, wrap(KindList, id))}}},
			"mutation(\n\t$ids: [ID!]!,\n) { thingAdd(\n\tids: $ids,\n) }",
		},
		"enum": {
			Field{Name: "thingState", Type: ref(KindENUM, "State")},
			"mutation { thingState }",
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := s.GetQueryStringForMutation(&tc.mutation, config.MutationConfig{Name: tc.mutation.Name, MaxQueryFieldDepth: 1})

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSchema_GetQueryStringForEndpoint_ScalarAndEnumEndpointsHaveNoSelection(t *testing.T) {
	t.Parallel()

	s := leafSchema()
	path := []string{"actor", "things"}

	typePath, err := s.LookupQueryTypesByFieldPath(path)
	require.NoError(t, err)

	cases := map[string]struct {
		endpoint config.EndpointConfig
		want     string
	}{
		"object keeps its selection": {
			config.EndpointConfig{Name: "payload", MaxQueryFieldDepth: 1},
			"query { actor { things { payload {\n\tid\n} } } }",
		},
		"scalar": {
			config.EndpointConfig{Name: "health", MaxQueryFieldDepth: 2},
			"query { actor { things { health } } }",
		},
		"non-null enum": {
			config.EndpointConfig{Name: "state", MaxQueryFieldDepth: 2},
			"query { actor { things { state } } }",
		},
		"scalar with a required argument": {
			config.EndpointConfig{Name: "token", MaxQueryFieldDepth: 2},
			"query(\n\t$scope: String!,\n) { actor { things { token(\n\tscope: $scope,\n) } } }",
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, s.GetQueryStringForEndpoint(typePath, path, tc.endpoint))
		})
	}
}

func TestSchema_GetQueryStringForEndpoint_ScalarEndpointOnTheFirstLevel(t *testing.T) {
	t.Parallel()

	s := leafSchema()
	path := []string{"actor"}

	typePath, err := s.LookupQueryTypesByFieldPath(path)
	assert.NoError(t, err)

	got := s.GetQueryStringForEndpoint(typePath, path, config.EndpointConfig{Name: "version", MaxQueryFieldDepth: 1})

	assert.Equal(t, "query { actor { version } }", got)
}
