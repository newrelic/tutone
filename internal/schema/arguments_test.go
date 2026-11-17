//go:build unit
// +build unit

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
)

// argumentSchema has, under the root, a field with an argument and two levels of
// containers that take arguments, and an endpoint at the bottom that takes more.
//
//	query { shop(region) { aisle(floor!, wing) { items(first, cursor, tags!) } } }
func argumentSchema() *Schema {
	strRef := ref(KindScalar, "String")
	intRef := ref(KindScalar, "Int")

	return &Schema{Types: []*Type{
		{Name: "RootQueryType", Kind: KindObject, Fields: []Field{
			{Name: "shop", Type: ref(KindObject, "Shop"), Args: []Field{{Name: "region", Type: strRef}}},
		}},
		{Name: "Shop", Kind: KindObject, Fields: []Field{
			{Name: "aisle", Type: ref(KindObject, "Aisle"), Args: []Field{
				{Name: "floor", Type: wrap(KindNonNull, intRef)},
				{Name: "wing", Type: strRef},
			}},
		}},
		{Name: "Aisle", Kind: KindObject, Fields: []Field{
			{Name: "items", Type: ref(KindObject, "Item"), Args: []Field{
				{Name: "first", Type: intRef},
				{Name: "cursor", Type: strRef},
				{Name: "tags", Type: wrap(KindNonNull, wrap(KindList, wrap(KindNonNull, strRef)))},
			}},
		}},
		{Name: "Item", Kind: KindObject, Fields: []Field{{Name: "id", Type: strRef}}},
		{Name: "String", Kind: KindScalar},
		{Name: "Int", Kind: KindScalar},
	}}
}

func TestSchema_GetInputFieldsForQueryPath_IncludesTheRootField(t *testing.T) {
	t.Parallel()

	got := argumentSchema().GetInputFieldsForQueryPath([]string{"shop", "aisle"})

	require.Contains(t, got, "shop", "the first field of the path is a field of the root query type")
	require.Contains(t, got, "aisle")
	assert.Equal(t, "region", got["shop"][0].Name)
	assert.Len(t, got["aisle"], 2)
}

func TestSchema_GetQueryStringForEndpoint_DeclaresAndSendsEveryArgument(t *testing.T) {
	t.Parallel()

	s := argumentSchema()
	path := []string{"shop", "aisle"}

	typePath, err := s.LookupQueryTypesByFieldPath(path)
	require.NoError(t, err)

	got := s.GetQueryStringForEndpoint(typePath, path, config.EndpointConfig{Name: "items", MaxQueryFieldDepth: 1})

	assert.Equal(t, "query(\n"+
		"\t$shopRegion: String,\n"+
		"\t$aisleFloor: Int!,\n"+
		"\t$aisleWing: String,\n"+
		"\t$first: Int,\n"+
		"\t$cursor: String,\n"+
		"\t$tags: [String!]!,\n"+
		") { shop(region: $shopRegion) { aisle(floor: $aisleFloor, wing: $aisleWing) { items(\n"+
		"\tfirst: $first,\n"+
		"\tcursor: $cursor,\n"+
		"\ttags: $tags,\n"+
		") {\n"+
		"\tid\n"+
		"} } } }", got,
		"a path field names all of its arguments in one call, and every argument is declared with the type the schema gives it")
}

func TestSchema_GetQueryStringForEndpoint_ExcludedArgumentIsNotDeclared(t *testing.T) {
	t.Parallel()

	s := argumentSchema()
	path := []string{"shop", "aisle"}

	typePath, err := s.LookupQueryTypesByFieldPath(path)
	require.NoError(t, err)

	got := s.GetQueryStringForEndpoint(typePath, path, config.EndpointConfig{
		Name: "items", MaxQueryFieldDepth: 1, ExcludeArguments: []string{"cursor"},
	})

	assert.NotContains(t, got, "cursor")
	assert.Contains(t, got, "$first: Int,")
}

func TestSchema_BuildQueryArgsForEndpoint_EveryArgument(t *testing.T) {
	t.Parallel()

	s := argumentSchema()
	aisle, err := s.LookupTypeByName("Aisle")
	require.NoError(t, err)

	assert.Equal(t, []QueryArg{
		{Key: "first", Value: "Int"},
		{Key: "cursor", Value: "String"},
		{Key: "tags", Value: "[String!]!"},
	}, s.BuildQueryArgsForEndpoint(aisle, []string{"items"}))
}
