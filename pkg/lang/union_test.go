//go:build unit
// +build unit

package lang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func ofKind(kind schema.Kind, name string) schema.TypeRef {
	return schema.TypeRef{Kind: kind, Name: name}
}

func nonNullList(r schema.TypeRef) schema.TypeRef {
	list := schema.TypeRef{Kind: schema.KindList, OfType: &r}

	return schema.TypeRef{Kind: schema.KindNonNull, OfType: &list}
}

// catalogSchema has an Actor with a single and a list field of a union, and a
// query that returns the union.
func catalogSchema() *schema.Schema {
	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "actor", Type: ofKind(schema.KindObject, "Actor")},
			}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "result", Type: ofKind(schema.KindUnion, "SearchResult")},
				{Name: "results", Type: nonNullList(ofKind(schema.KindUnion, "SearchResult"))},
				{Name: "name", Type: ofKind(schema.KindScalar, "String")},
			}},
			{
				Name: "SearchResult", Kind: schema.KindUnion, Description: "Anything the catalog finds",
				PossibleTypes: []schema.TypeRef{ofKind(schema.KindObject, "Template"), ofKind(schema.KindObject, "Quickstart")},
			},
			{Name: "Template", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "id", Type: ofKind(schema.KindScalar, "ID")},
			}},
			{Name: "Quickstart", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "title", Type: ofKind(schema.KindScalar, "String")},
			}},
			{Name: "String", Kind: schema.KindScalar},
			{Name: "ID", Kind: schema.KindScalar},
		},
	}
}

func generateTypes(t *testing.T, s *schema.Schema, pkg *config.PackageConfig) ([]GoStruct, []GoInterface) {
	t.Helper()

	expanded, err := schema.ExpandTypes(s, pkg)
	require.NoError(t, err)

	structs, _, _, interfaces, err := GenerateGoTypesForPackage(s, &config.GeneratorConfig{}, pkg, expanded)
	require.NoError(t, err)

	return *structs, *interfaces
}

func structNamed(t *testing.T, structs []GoStruct, name string) GoStruct {
	t.Helper()

	for _, st := range structs {
		if st.Name == name {
			return st
		}
	}

	require.FailNow(t, "struct not generated", name)

	return GoStruct{}
}

func fieldNamed(t *testing.T, st GoStruct, name string) GoStructField {
	t.Helper()

	for _, f := range st.Fields {
		if f.Name == name {
			return f
		}
	}

	require.FailNow(t, "field not generated", "%s.%s", st.Name, name)

	return GoStructField{}
}

func catalogPackage(t *testing.T) *config.PackageConfig {
	t.Helper()

	return &config.PackageConfig{Name: "catalog", Path: t.TempDir(), Types: []config.TypeConfig{{Name: "Actor"}}}
}

func TestGenerateGoTypes_UnionBecomesAGoInterfaceItsMembersImplement(t *testing.T) {
	t.Parallel()

	structs, interfaces := generateTypes(t, catalogSchema(), catalogPackage(t))

	require.Len(t, interfaces, 1)
	assert.Equal(t, "SearchResult", interfaces[0].Name)
	assert.Equal(t, []string{"ImplementsSearchResult()"}, interfaces[0].Methods)
	assert.Equal(t, []GoInterfacePossibleType{
		{GraphQLName: "Template", GoName: "Template"},
		{GraphQLName: "Quickstart", GoName: "Quickstart"},
	}, interfaces[0].PossibleTypes)

	assert.Equal(t, []string{"SearchResult"}, structNamed(t, structs, "Template").Implements)
	assert.Equal(t, []string{"SearchResult"}, structNamed(t, structs, "Quickstart").Implements)

	for _, st := range structs {
		assert.NotEqual(t, "SearchResult", st.Name, "a union has no fields, so no struct is made for it")
	}
}

func TestGenerateGoTypes_UnionFieldsAreInterfaceFieldsWithAnUnmarshalJSON(t *testing.T) {
	t.Parallel()

	structs, _ := generateTypes(t, catalogSchema(), catalogPackage(t))

	actor := structNamed(t, structs, "Actor")
	assert.True(t, actor.SpecialUnmarshal)

	single := fieldNamed(t, actor, "Result")
	assert.Equal(t, "SearchResultInterface", single.Type)
	assert.Equal(t, "SearchResult", single.TypeName)
	assert.True(t, single.IsInterface)
	assert.False(t, single.IsList)

	list := fieldNamed(t, actor, "Results")
	assert.Equal(t, "[]SearchResultInterface", list.Type)
	assert.True(t, list.IsInterface)
	assert.True(t, list.IsList)

	assert.False(t, fieldNamed(t, actor, "Name").IsInterface)
}

func TestGenerateGoTypes_UnionWithAConfiguredGoTypeGeneratesNothingForIt(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]config.TypeConfig{
		"field_type_override": {Name: "SearchResult", FieldTypeOverride: "ai.SearchResult"},
		"skip_type_create":    {Name: "SearchResult", SkipTypeCreate: true},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg := catalogPackage(t)
			pkg.Types = append(pkg.Types, tc)

			structs, interfaces := generateTypes(t, catalogSchema(), pkg)

			assert.Empty(t, interfaces)

			actor := structNamed(t, structs, "Actor")
			assert.False(t, actor.SpecialUnmarshal)
			assert.False(t, fieldNamed(t, actor, "Result").IsInterface)
			assert.NotContains(t, fieldNamed(t, actor, "Result").Type, "Interface")
		})
	}
}

func TestGenerateGoTypes_UnionFieldKeepsTheConfiguredTypeVerbatim(t *testing.T) {
	t.Parallel()

	pkg := catalogPackage(t)
	pkg.Types = append(pkg.Types, config.TypeConfig{Name: "SearchResult", FieldTypeOverride: "ai.SearchResult"})

	structs, _ := generateTypes(t, catalogSchema(), pkg)

	assert.Equal(t, "ai.SearchResult", fieldNamed(t, structNamed(t, structs, "Actor"), "Result").Type)
	assert.Equal(t, "[]ai.SearchResult", fieldNamed(t, structNamed(t, structs, "Actor"), "Results").Type)
}

func TestGenerateGoTypes_UnionMemberTheConfigSkipsIsLeftOutOfTheUnion(t *testing.T) {
	t.Parallel()

	pkg := catalogPackage(t)
	pkg.Types = append(pkg.Types, config.TypeConfig{Name: "Quickstart", SkipTypeCreate: true})

	structs, interfaces := generateTypes(t, catalogSchema(), pkg)

	require.Len(t, interfaces, 1)
	assert.Equal(t, []GoInterfacePossibleType{{GraphQLName: "Template", GoName: "Template"}}, interfaces[0].PossibleTypes)

	for _, st := range structs {
		assert.NotEqual(t, "Quickstart", st.Name)
	}
}

func TestGenerateGoTypes_UnionInterfaceMethodsComeFromTheConfig(t *testing.T) {
	t.Parallel()

	pkg := catalogPackage(t)
	pkg.Types = append(pkg.Types, config.TypeConfig{Name: "SearchResult", InterfaceMethods: []string{"Describe() string"}})

	_, interfaces := generateTypes(t, catalogSchema(), pkg)

	require.Len(t, interfaces, 1)
	assert.Equal(t, []string{"ImplementsSearchResult()", "Describe() string"}, interfaces[0].Methods)
}

func TestImplementUnions_MemberOfTwoUnionsImplementsBoth(t *testing.T) {
	t.Parallel()

	structs := []GoStruct{{Name: "Shared", Implements: []string{"Node"}}}
	unions := []GoInterface{
		{Name: "First", PossibleTypes: []GoInterfacePossibleType{{GoName: "Shared"}}},
		{Name: "Second", PossibleTypes: []GoInterfacePossibleType{{GoName: "Shared"}}},
	}

	implementUnions(structs, unions)

	assert.Equal(t, []string{"Node", "First", "Second"}, structs[0].Implements)
}

func TestImplementUnions_DoesNotAddAnInterfaceTwice(t *testing.T) {
	t.Parallel()

	structs := []GoStruct{{Name: "Member", Implements: []string{"Union"}}}
	unions := []GoInterface{{Name: "Union", PossibleTypes: []GoInterfacePossibleType{{GoName: "Member"}}}}

	implementUnions(structs, unions)

	assert.Equal(t, []string{"Union"}, structs[0].Implements)
}

func TestGoMethodForField_UnionReturnIsAnInterface(t *testing.T) {
	t.Parallel()

	field := schema.Field{Name: "search", Type: ofKind(schema.KindUnion, "SearchResult")}

	plain := goMethodForField(field, &config.PackageConfig{}, nil, methodArgOptions{})
	assert.Equal(t, []string{"SearchResultInterface", "error"}, plain.Signature.Return)

	list := goMethodForField(schema.Field{Name: "search", Type: nonNullList(ofKind(schema.KindUnion, "SearchResult"))}, &config.PackageConfig{}, nil, methodArgOptions{})
	assert.Equal(t, []string{"[]SearchResultInterface", "error"}, list.Signature.Return)
	assert.True(t, list.Signature.ReturnSlice)

	overridden := goMethodForField(field, &config.PackageConfig{Types: []config.TypeConfig{{Name: "SearchResult", FieldTypeOverride: "ai.SearchResult"}}}, nil, methodArgOptions{})
	assert.Equal(t, []string{"ai.SearchResult", "error"}, overridden.Signature.Return)
}

func TestKeptNames_UnionKeepsItsInterfaceAndUnmarshalFunction(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{Types: []config.TypeConfig{{Name: "SearchResult", KeepExisting: true}}}
	expanded := []*schema.Type{{Name: "SearchResult", Kind: schema.KindUnion}}

	assert.ElementsMatch(t,
		[]string{"SearchResult", "SearchResultInterface", "UnmarshalSearchResultInterface"},
		KeptNames(pkg, &expanded))
}

// entitySchema models the NGEP shape: the types on the way to an endpoint are not
// expanded, and the interface the endpoint returns is.
func entitySchema() *schema.Schema {
	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "actor", Type: ofKind(schema.KindObject, "Actor")},
			}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "management", Type: ofKind(schema.KindObject, "Management")},
			}},
			{Name: "Management", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "entity", Type: ofKind(schema.KindInterface, "ManagedEntity")},
				{Name: "search", Type: ofKind(schema.KindObject, "SearchResult")},
			}},
			{Name: "ManagedEntity", Kind: schema.KindInterface, Fields: []schema.Field{
				{Name: "id", Type: ofKind(schema.KindScalar, "ID")},
			}, PossibleTypes: []schema.TypeRef{ofKind(schema.KindObject, "McpServer")}},
			{Name: "McpServer", Kind: schema.KindObject, Interfaces: []schema.TypeRef{ofKind(schema.KindInterface, "ManagedEntity")}, Fields: []schema.Field{
				{Name: "id", Type: ofKind(schema.KindScalar, "ID")},
			}},
			{Name: "SearchResult", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "count", Type: ofKind(schema.KindScalar, "Int")},
			}},
			{Name: "ID", Kind: schema.KindScalar},
			{Name: "Int", Kind: schema.KindScalar},
		},
	}
}

func entityPackage(t *testing.T) *config.PackageConfig {
	t.Helper()

	return &config.PackageConfig{
		Name: "mcp",
		Path: t.TempDir(),
		Queries: []config.Query{{
			Path:      []string{"actor", "management"},
			Endpoints: []config.EndpointConfig{{Name: "entity", MaxQueryFieldDepth: 2}, {Name: "search", MaxQueryFieldDepth: 2}},
		}},
	}
}

func TestGenerateGoTypes_ResponseStructWithASingleInterfaceFieldGetsAnUnmarshalJSON(t *testing.T) {
	t.Parallel()

	structs, interfaces := generateTypes(t, entitySchema(), entityPackage(t))

	require.Len(t, interfaces, 1)

	management := structNamed(t, structs, "Management")
	assert.True(t, management.SpecialUnmarshal)

	entity := fieldNamed(t, management, "Entity")
	assert.Equal(t, "ManagedEntityInterface", entity.Type)
	assert.True(t, entity.IsInterface)
	assert.False(t, entity.IsList)

	assert.False(t, structNamed(t, structs, "Actor").SpecialUnmarshal, "Actor has no interface field")
}

func TestGenerateGoTypes_ResponseStructWhoseInterfaceThePackageSuppliesGetsNoUnmarshalJSON(t *testing.T) {
	t.Parallel()

	pkg := entityPackage(t)
	pkg.Types = []config.TypeConfig{{Name: "ManagedEntity", SkipTypeCreate: true}}

	structs, interfaces := generateTypes(t, entitySchema(), pkg)

	assert.Empty(t, interfaces)

	management := structNamed(t, structs, "Management")
	assert.True(t, fieldNamed(t, management, "Entity").IsInterface, "GetEntity reads it")
	assert.False(t, management.SpecialUnmarshal, "Unmarshal<Name>Interface would not exist")
}

func TestGenerateGoMethodQueries_GetEntityOfASkippedInterfaceReadsAFieldItsResponseStructHas(t *testing.T) {
	t.Parallel()

	pkg := entityPackage(t)
	pkg.Types = []config.TypeConfig{{Name: "ManagedEntity", SkipTypeCreate: true}}

	structs, _ := generateTypes(t, entitySchema(), pkg)

	methods, err := GenerateGoMethodQueriesForPackage(entitySchema(), &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)
	require.Len(t, *methods, 2)

	for _, m := range *methods {
		assert.Equal(t, []string{"Actor", "Management"}, m.Signature.ReturnPath)
		assert.Equal(t, m.Signature.Return[0], responseField(t, structs, m).Type, "%s returns what the field it reads holds", m.MethodName)
	}

	assert.Equal(t, "GetEntity", (*methods)[0].MethodName)
	assert.Equal(t, []string{"ManagedEntityInterface", "error"}, (*methods)[0].Signature.Return)
}

// suppliedSchema has the endpoint actor.thing.value, which returns a type called Supplied
// of the given kind, and a field of that type on actor that no endpoint reads.
func suppliedSchema(kind schema.Kind) *schema.Schema {
	supplied := &schema.Type{Name: "Supplied", Kind: kind}
	if kind == schema.KindObject || kind == schema.KindInterface {
		supplied.Fields = []schema.Field{{Name: "id", Type: ofKind(schema.KindScalar, "ID")}}
	}

	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "actor", Type: ofKind(schema.KindObject, "Actor")},
			}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "thing", Type: ofKind(schema.KindObject, "Thing")},
				{Name: "owner", Type: ofKind(kind, "Supplied")},
			}},
			{Name: "Thing", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "value", Type: ofKind(kind, "Supplied")},
			}},
			supplied,
			{Name: "ID", Kind: schema.KindScalar},
		},
	}
}

// responseField follows the fields a query method reads from its response, from the
// response struct to the endpoint's field, and returns the last. A struct that lacks the
// next field fails the test, as the method would not compile.
func responseField(t *testing.T, structs []GoStruct, m GoMethod) GoStructField {
	t.Helper()

	path := append(append([]string(nil), m.Signature.ReturnPath...), m.FieldName)
	st := structNamed(t, structs, m.ResponseObjectType)

	var field GoStructField

	for i, name := range path {
		field = fieldNamed(t, st, name)

		if i < len(path)-1 {
			st = structNamed(t, structs, field.Type)
		}
	}

	return field
}

func TestGenerateGoTypes_ResponseStructsHaveTheFieldOfAnEndpointWhoseTypeThePackageSupplies(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind          schema.Kind
		config        config.TypeConfig
		wantType      string
		wantInterface bool
	}{
		"skipped interface": {
			schema.KindInterface, config.TypeConfig{Name: "Supplied", SkipTypeCreate: true},
			"SuppliedInterface", true,
		},
		"interface of another package": {
			schema.KindInterface, config.TypeConfig{Name: "Supplied", SkipTypeCreate: true, FieldTypeOverride: "ai.Supplied"},
			"ai.SuppliedInterface", true,
		},
		"skipped union": {
			schema.KindUnion, config.TypeConfig{Name: "Supplied", SkipTypeCreate: true},
			"Supplied", false,
		},
		"union of another package": {
			schema.KindUnion, config.TypeConfig{Name: "Supplied", SkipTypeCreate: true, FieldTypeOverride: "ai.Supplied"},
			"ai.Supplied", false,
		},
		"skipped object": {
			schema.KindObject, config.TypeConfig{Name: "Supplied", SkipTypeCreate: true},
			"Supplied", false,
		},
		"scalar of another package": {
			schema.KindScalar, config.TypeConfig{Name: "Supplied", SkipTypeCreate: true, FieldTypeOverride: "nrtime.Supplied"},
			"nrtime.Supplied", false,
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg := &config.PackageConfig{
				Name:    "things",
				Path:    t.TempDir(),
				Types:   []config.TypeConfig{tc.config},
				Queries: []config.Query{{Path: []string{"actor", "thing"}, Endpoints: []config.EndpointConfig{{Name: "value", MaxQueryFieldDepth: 2}}}},
			}

			s := suppliedSchema(tc.kind)
			structs, interfaces := generateTypes(t, s, pkg)

			assert.Empty(t, interfaces)

			methods, err := GenerateGoMethodQueriesForPackage(s, &config.GeneratorConfig{}, pkg)
			require.NoError(t, err)
			require.Len(t, *methods, 1)

			value := responseField(t, structs, (*methods)[0])
			assert.Equal(t, tc.wantType, value.Type)
			assert.Equal(t, value.Type, (*methods)[0].Signature.Return[0], "the method returns what the field it reads holds")
			assert.Equal(t, tc.wantInterface, value.IsInterface)
			assert.False(t, structNamed(t, structs, "Thing").SpecialUnmarshal, "no Unmarshal<Name>Interface is generated for it")

			actor := structNamed(t, structs, "Actor")
			require.Len(t, actor.Fields, 1, "owner is not read by an endpoint, so the response struct, which is limited to the way to them, leaves it out")
			assert.Equal(t, "Thing", actor.Fields[0].Name)
		})
	}
}

func TestUnmarshalsInterfaceFields(t *testing.T) {
	t.Parallel()

	generated := []GoInterface{{Name: "ManagedEntity"}}

	cases := map[string]struct {
		fields []GoStructField
		want   bool
	}{
		"no fields":                    {nil, false},
		"no interface field":           {[]GoStructField{{Name: "ID", TypeName: "string"}}, false},
		"generated interface":          {[]GoStructField{{Name: "Entity", TypeName: "ManagedEntity", IsInterface: true}}, true},
		"interface the package lacks":  {[]GoStructField{{Name: "Entity", TypeName: "Entity", IsInterface: true}}, false},
		"one of two lacks its decoder": {[]GoStructField{{Name: "A", TypeName: "ManagedEntity", IsInterface: true}, {Name: "B", TypeName: "Entity", IsInterface: true}}, false},
		"external interface":           {[]GoStructField{{Name: "A", TypeName: "ai.Auth", IsInterface: true}}, false},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, unmarshalsInterfaceFields(tc.fields, generated))
		})
	}
}

func TestOmitDeclaredUnmarshalers(t *testing.T) {
	t.Parallel()

	structs := []GoStruct{
		{Name: "Declared", SpecialUnmarshal: true},
		{Name: "Open", SpecialUnmarshal: true},
		{Name: "Plain"},
	}

	omitDeclaredUnmarshalers(structs, map[string]bool{"Declared.UnmarshalJSON": true, "Open.String": true, "Plain.UnmarshalJSON": true})

	assert.False(t, structs[0].SpecialUnmarshal)
	assert.True(t, structs[1].SpecialUnmarshal, "another method of the type is no reason to omit it")
	assert.False(t, structs[2].SpecialUnmarshal)
}

func TestGenerateGoTypes_UnmarshalJSONDeclaredByHandIsNotGenerated(t *testing.T) {
	t.Parallel()

	pkg := entityPackage(t)

	require.NoError(t, os.WriteFile(filepath.Join(pkg.Path, "extra.go"), []byte(`package mcp

import "encoding/json"

func (x *Management) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, nil) }
`), 0o644))

	structs, _ := generateTypes(t, entitySchema(), pkg)

	assert.False(t, structNamed(t, structs, "Management").SpecialUnmarshal)
}

func TestGenerateGoTypes_UnmarshalJSONInAGeneratedFileDoesNotCountAsHandWritten(t *testing.T) {
	t.Parallel()

	pkg := entityPackage(t)

	require.NoError(t, os.WriteFile(filepath.Join(pkg.Path, "types.go"), []byte(`// Code generated by tutone: DO NOT EDIT
package mcp

import "encoding/json"

func (x *Management) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, nil) }
`), 0o644))

	structs, _ := generateTypes(t, entitySchema(), pkg)

	assert.True(t, structNamed(t, structs, "Management").SpecialUnmarshal)
}

func TestGenerateGoTypes_HandWrittenFileThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()

	pkg := entityPackage(t)

	require.NoError(t, os.WriteFile(filepath.Join(pkg.Path, "broken.go"), []byte("package mcp\nfunc ("), 0o644))

	expanded, err := schema.ExpandTypes(entitySchema(), pkg)
	require.NoError(t, err)

	_, _, _, _, err = GenerateGoTypesForPackage(entitySchema(), &config.GeneratorConfig{}, pkg, expanded)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken.go")
}
