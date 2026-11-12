//go:build unit
// +build unit

package schema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
)

func unionTestSchema() *Schema {
	scalar := func(name string) TypeRef { return TypeRef{Kind: KindScalar, Name: name} }
	obj := func(name string) TypeRef { return TypeRef{Kind: KindObject, Name: name} }
	union := func(name string) TypeRef { return TypeRef{Kind: KindUnion, Name: name} }
	nonNullListOf := func(r TypeRef) TypeRef {
		list := TypeRef{Kind: KindList, OfType: &r}
		return TypeRef{Kind: KindNonNull, OfType: &list}
	}

	return &Schema{Types: []*Type{
		{Name: "Catalog", Kind: KindObject, Fields: []Field{
			{Name: "result", Type: union("SearchResult")},
			{Name: "results", Type: nonNullListOf(union("SearchResult"))},
			{Name: "name", Type: scalar("String")},
		}},
		{
			Name: "SearchResult", Kind: KindUnion,
			PossibleTypes: []TypeRef{obj("Template"), obj("Quickstart"), obj("Needy")},
		},
		{Name: "Template", Kind: KindObject, Fields: []Field{
			{Name: "id", Type: scalar("ID")},
			{Name: "owner", Type: obj("Owner")},
		}},
		{Name: "Quickstart", Kind: KindObject, Fields: []Field{
			{Name: "id", Type: scalar("ID")},
			{Name: "title", Type: scalar("String")},
		}},
		// Every field of Needy takes a required argument, so none can be selected.
		{Name: "Needy", Kind: KindObject, Fields: []Field{
			{Name: "page", Type: scalar("String"), Args: []Field{
				{Name: "number", Type: TypeRef{Kind: KindNonNull, OfType: &TypeRef{Kind: KindScalar, Name: "Int"}}},
			}},
		}},
		{Name: "Owner", Kind: KindObject, Fields: []Field{{Name: "email", Type: scalar("String")}}},
		{Name: "String", Kind: KindScalar},
		{Name: "ID", Kind: KindScalar},
		{Name: "Int", Kind: KindScalar},
	}}
}

func TestTypeRef_IsUnion(t *testing.T) {
	t.Parallel()

	wrapped := func(kind Kind, inner TypeRef) TypeRef { return TypeRef{Kind: kind, OfType: &inner} }

	cases := map[string]struct {
		ref  TypeRef
		want bool
	}{
		"union":                   {TypeRef{Kind: KindUnion, Name: "U"}, true},
		"non-null list of union":  {wrapped(KindNonNull, wrapped(KindList, TypeRef{Kind: KindUnion, Name: "U"})), true},
		"interface":               {TypeRef{Kind: KindInterface, Name: "I"}, false},
		"object":                  {TypeRef{Kind: KindObject, Name: "O"}, false},
		"list of scalar":          {wrapped(KindList, TypeRef{Kind: KindScalar, Name: "String"}), false},
		"non-null list of object": {wrapped(KindNonNull, wrapped(KindList, TypeRef{Kind: KindObject, Name: "O"})), false},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.ref.IsUnion())
		})
	}
}

func TestType_GetQueryStringFields_Union(t *testing.T) {
	t.Parallel()

	s := unionTestSchema()
	catalog, err := s.LookupTypeByName("Catalog")
	require.NoError(t, err)

	got := catalog.GetQueryStringFields(s, 0, 3, false, nil)

	// A union has no fields of its own: the selection is __typename and one inline
	// fragment per member, in the order the schema lists them, each asking for
	// __typename too. Needy has nothing to select, which leaves its fragment's
	// second line blank.
	fragments := []string{
		"\t... on Template {",
		"\t\t__typename",
		"\t\tid",
		"\t\towner {",
		"\t\t\temail",
		"\t\t}",
		"\t}",
		"\t... on Quickstart {",
		"\t\t__typename",
		"\t\tid",
		"\t\ttitle",
		"\t}",
		"\t... on Needy {",
		"\t\t__typename",
		"\t\t",
		"\t}",
	}

	selection := func(name string) []string {
		return append(append([]string{name + " {", "\t__typename"}, fragments...), "}")
	}

	want := strings.Join(append(append([]string{"name"}, selection("result")...), selection("results")...), "\n")
	assert.Equal(t, want, got)
}

func TestType_GetQueryStringFields_UnionBeyondMaxDepthIsLeftOut(t *testing.T) {
	t.Parallel()

	s := unionTestSchema()
	catalog, err := s.LookupTypeByName("Catalog")
	require.NoError(t, err)

	assert.Equal(t, "name", catalog.GetQueryStringFields(s, 0, 0, false, nil))
}

func TestType_GetQueryStringFields_UnionMemberMissingFromTheSchemaIsSkipped(t *testing.T) {
	t.Parallel()

	s := unionTestSchema()
	union, err := s.LookupTypeByName("SearchResult")
	require.NoError(t, err)

	union.PossibleTypes = append(union.PossibleTypes, TypeRef{Kind: KindObject, Name: "Missing"})

	catalog, err := s.LookupTypeByName("Catalog")
	require.NoError(t, err)

	got := catalog.GetQueryStringFields(s, 0, 3, false, nil)
	assert.NotContains(t, got, "Missing")
	assert.Contains(t, got, "... on Quickstart {")
}

func expandedNames(t *testing.T, s *Schema, pkg *config.PackageConfig) []string {
	t.Helper()

	expanded, err := ExpandTypes(s, pkg)
	require.NoError(t, err)

	var names []string
	for _, e := range *expanded {
		names = append(names, e.Name)
	}

	return names
}

func TestExpandTypes_UnionExpandsItsMembers(t *testing.T) {
	t.Parallel()

	s := unionTestSchema()
	s.MutationType = &Type{Name: "RootMutationType", Kind: KindObject}

	names := expandedNames(t, s, &config.PackageConfig{Types: []config.TypeConfig{{Name: "SearchResult"}}})

	assert.ElementsMatch(t, []string{"SearchResult", "Template", "Quickstart", "Needy", "Owner", "String", "ID", "Int"}, names)
}

func TestExpandTypes_SkippedUnionExpandsNothing(t *testing.T) {
	t.Parallel()

	s := unionTestSchema()
	s.MutationType = &Type{Name: "RootMutationType", Kind: KindObject}

	names := expandedNames(t, s, &config.PackageConfig{Types: []config.TypeConfig{{Name: "SearchResult", SkipTypeCreate: true}}})

	assert.Empty(t, names)
}

func TestExpandTypes_UnionReachedThroughAField(t *testing.T) {
	t.Parallel()

	s := unionTestSchema()
	s.MutationType = &Type{Name: "RootMutationType", Kind: KindObject}

	names := expandedNames(t, s, &config.PackageConfig{Types: []config.TypeConfig{{Name: "Catalog"}}})

	assert.Contains(t, names, "SearchResult")
	assert.Contains(t, names, "Quickstart")
}
