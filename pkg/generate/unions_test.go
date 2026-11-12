package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
)

func unionSchema() *schema.Schema {
	return &schema.Schema{Types: []*schema.Type{
		{Name: "AiNotificationsAuth", Kind: schema.KindUnion},
		{Name: "SearchResult", Kind: schema.KindUnion},
		{Name: "Template", Kind: schema.KindObject},
	}}
}

func typeConfig(t *testing.T, pkg *config.PackageConfig, name string) *config.TypeConfig {
	t.Helper()

	for i := range pkg.Types {
		if pkg.Types[i].Name == name {
			return &pkg.Types[i]
		}
	}

	return nil
}

func TestSkipUnionsWithGoType_HandWrittenTypeOfTheUnionsName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types_.go"), []byte("package p\n\ntype AiNotificationsAuth struct{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "entities", Path: dir}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	require.NotNil(t, typeConfig(t, pkg, "AiNotificationsAuth"))
	assert.True(t, typeConfig(t, pkg, "AiNotificationsAuth").SkipTypeCreate)
	assert.Nil(t, typeConfig(t, pkg, "SearchResult"), "a union the package does not have is generated")
}

func TestSkipUnionsWithGoType_HandWrittenInterfaceOfTheUnionsName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package p\n\ntype SearchResultInterface interface{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "catalog", Path: dir}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	search := typeConfig(t, pkg, "SearchResult")
	assert.True(t, search.SkipTypeCreate)
	assert.Equal(t, "SearchResultInterface", search.FieldTypeOverride, "the package has no SearchResult, so its fields have the type it does have")
}

func TestSkipUnionsWithGoType_HandWrittenTypeAndInterfaceKeepTheUnionsOwnName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package p\n\ntype SearchResult struct{}\n\ntype SearchResultInterface interface{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "catalog", Path: dir}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	search := typeConfig(t, pkg, "SearchResult")
	assert.True(t, search.SkipTypeCreate)
	assert.Empty(t, search.FieldTypeOverride, "the fields keep the type of the union's name")
}

func TestSkipUnionsWithGoType_SkippedInConfigWithOnlyAHandWrittenInterfaceGetsItsType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package p\n\ntype SearchResultInterface interface{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "catalog", Path: dir, Types: []config.TypeConfig{{Name: "SearchResult", SkipTypeCreate: true}}}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	assert.Equal(t, "SearchResultInterface", typeConfig(t, pkg, "SearchResult").FieldTypeOverride, "SearchResult is not declared, so the fields would have a type that is not there")
}

func TestSkipUnionsWithGoType_ConfiguredOverrideWinsOverTheHandWrittenInterface(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package p\n\ntype SearchResultInterface interface{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "catalog", Path: dir, Types: []config.TypeConfig{{Name: "SearchResult", FieldTypeOverride: "ai.SearchResult"}}}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	search := typeConfig(t, pkg, "SearchResult")
	assert.True(t, search.SkipTypeCreate)
	assert.Equal(t, "ai.SearchResult", search.FieldTypeOverride)
}

// The fields of a union the package has only as <Name>Interface have that type, in the
// structs and in what the methods return, and nothing is generated that the package
// declares: the generated code refers to no undefined type and declares none twice.
func TestSkipUnionsWithGoType_HandWrittenInterfaceIsTheTypeOfTheGeneratedFieldsAndReturns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package p\n\ntype SearchResultInterface interface{ ImplementsSearchResult() }\n"), 0o644))

	s := &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: ref(schema.KindObject, "Actor")}}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "catalog", Type: ref(schema.KindObject, "Catalog")},
			}},
			{Name: "Catalog", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "result", Type: ref(schema.KindUnion, "SearchResult")},
				{Name: "results", Type: schema.TypeRef{Kind: schema.KindList, OfType: &schema.TypeRef{Kind: schema.KindUnion, Name: "SearchResult"}}},
			}},
			{Name: "SearchResult", Kind: schema.KindUnion, PossibleTypes: []schema.TypeRef{ref(schema.KindObject, "Template")}},
			{Name: "Template", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: ref(schema.KindScalar, "Int")}}},
			{Name: "Int", Kind: schema.KindScalar},
		},
	}

	pkg := &config.PackageConfig{
		Name:  "catalog",
		Path:  dir,
		Types: []config.TypeConfig{{Name: "Catalog"}},
		Queries: []config.Query{{
			Path:      []string{"actor", "catalog"},
			Endpoints: []config.EndpointConfig{{Name: "result", MaxQueryFieldDepth: 2}, {Name: "results", MaxQueryFieldDepth: 2}},
		}},
	}

	require.NoError(t, skipUnionsWithGoType(s, pkg))

	expanded, err := schema.ExpandTypes(s, pkg)
	require.NoError(t, err)

	structs, _, _, interfaces, err := lang.GenerateGoTypesForPackage(s, &config.GeneratorConfig{}, pkg, expanded)
	require.NoError(t, err)

	assert.Empty(t, *interfaces, "the package declares SearchResultInterface")

	var catalog *lang.GoStruct

	for i := range *structs {
		switch (*structs)[i].Name {
		case "Catalog":
			catalog = &(*structs)[i]
		case "Template":
			assert.Fail(t, "a member of a union the package supplies is not generated because of it")
		}
	}

	require.NotNil(t, catalog)
	assert.False(t, catalog.SpecialUnmarshal, "no Unmarshal<Name>Interface is known to exist")

	types := map[string]string{}
	for _, f := range catalog.Fields {
		types[f.Name] = f.Type
		assert.False(t, f.IsInterface, f.Name)
	}

	assert.Equal(t, map[string]string{"Result": "SearchResultInterface", "Results": "[]SearchResultInterface"}, types)

	methods, err := lang.GenerateGoMethodQueriesForPackage(s, &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)

	returns := map[string][]string{}
	for _, m := range *methods {
		returns[m.MethodName] = m.Signature.Return
	}

	assert.Equal(t, map[string][]string{
		"GetResult":  {"SearchResultInterface", "error"},
		"GetResults": {"[]SearchResultInterface", "error"},
	}, returns)
}

func TestSkipUnionsWithGoType_ConfiguredOverride(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name: "notifications",
		Path: t.TempDir(),
		Types: []config.TypeConfig{
			{Name: "AiNotificationsAuth", FieldTypeOverride: "ai.AiNotificationsAuth"},
			{Name: "SearchResult"},
		},
	}

	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	auth := typeConfig(t, pkg, "AiNotificationsAuth")
	assert.True(t, auth.SkipTypeCreate)
	assert.Equal(t, "ai.AiNotificationsAuth", auth.FieldTypeOverride, "the override is kept")
	assert.False(t, typeConfig(t, pkg, "SearchResult").SkipTypeCreate)
}

func TestSkipUnionsWithGoType_GeneratedFilesDoNotCount(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte("// Code generated by tutone: DO NOT EDIT\npackage p\n\ntype SearchResultInterface interface{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "catalog", Path: dir}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	assert.Nil(t, typeConfig(t, pkg, "SearchResult"), "the generator's own output from the last run is replaced, not kept")
}

func TestSkipUnionsWithGoType_NonUnionOfTheSameNameIsLeftAlone(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package p\n\ntype Template struct{}\n"), 0o644))

	pkg := &config.PackageConfig{Name: "catalog", Path: dir}
	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	assert.Empty(t, pkg.Types)
}

func TestSkipUnionsWithGoType_DoesNotChangeTheCallersConfig(t *testing.T) {
	t.Parallel()

	types := []config.TypeConfig{{Name: "AiNotificationsAuth", FieldTypeOverride: "ai.AiNotificationsAuth"}}
	pkg := &config.PackageConfig{Name: "notifications", Path: t.TempDir(), Types: types}

	require.NoError(t, skipUnionsWithGoType(unionSchema(), pkg))

	assert.False(t, types[0].SkipTypeCreate, "the shared slice is copied before it is changed")
}

func TestSkipUnionsWithGoType_UnparseableHandWrittenFileIsAnError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package p\nfunc ("), 0o644))

	err := skipUnionsWithGoType(unionSchema(), &config.PackageConfig{Name: "catalog", Path: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken.go")
}
