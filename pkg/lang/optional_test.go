package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func optionalField() schema.Field {
	scalar := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: n} }
	wrap := func(kind schema.Kind, of schema.TypeRef) schema.TypeRef {
		return schema.TypeRef{Kind: kind, OfType: &of}
	}

	return schema.Field{
		Name: "items",
		Type: scalar("String"),
		Args: []schema.Field{
			{Name: "first", Type: scalar("Int")},
			{Name: "cursor", Type: scalar("String")},
			{Name: "filter", Type: wrap(schema.KindList, wrap(schema.KindNonNull, scalar("String")))},
			{Name: "ids", Type: wrap(schema.KindNonNull, wrap(schema.KindList, scalar("ID")))},
			{Name: "id", Type: scalar("ID")},
			{Name: "name", Type: wrap(schema.KindNonNull, scalar("String"))},
			{Name: "type", Type: scalar("String")},
		},
	}
}

// inputTypes maps each parameter, named as the template prints it, to its type.
func inputTypes(m GoMethod) map[string]string {
	out := map[string]string{}
	for _, in := range m.Signature.Input {
		out[lowerFirst(in.Name)] = in.Type
	}

	return out
}

func TestGoMethodForField_IncludeOptionalArguments(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{Types: []config.TypeConfig{{Name: "ID", FieldTypeOverride: "string"}}}

	t.Run("off keeps every parameter as it was", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(optionalField(), pkg, nil, methodArgOptions{})

		assert.Equal(t, map[string]string{
			"first": "int", "cursor": "string", "filter": "[]string", "iDs": "[]string",
			"iD": "string", "name": "string", "typeArg": "string",
		}, inputTypes(m))
		assert.Empty(t, m.UnsetParams)
	})

	t.Run("a nullable argument is a pointer, a list stays a slice, a required one is unchanged", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(optionalField(), pkg, nil, methodArgOptions{IncludeOptional: true})

		assert.Equal(t, map[string]string{
			"first": "*int", "cursor": "*string", "filter": "[]string", "iDs": "[]string",
			"iD": "*string", "name": "string", "typeArg": "*string",
		}, inputTypes(m))

		assert.Equal(t, []UnsetParam{
			{Param: "first", Key: "first"},
			{Param: "cursor", Key: "cursor"},
			{Param: "filter", Key: "filter"},
			{Param: "iD", Key: "id"},
			{Param: "typeArg", Key: "type"},
		}, m.UnsetParams, "only the nullable arguments, with the parameter as the template prints it")
	})

	t.Run("exclude_arguments still removes an argument", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(optionalField(), pkg, nil, methodArgOptions{IncludeOptional: true, Exclude: []string{"cursor", "type"}})

		assert.NotContains(t, inputTypes(m), "cursor")
		assert.NotContains(t, m.UnsetParams, UnsetParam{Param: "cursor", Key: "cursor"})
		assert.Len(t, m.QueryVars, 5)
	})

	t.Run("argument_go_types wins and is never made a double pointer", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(optionalField(), pkg, nil, methodArgOptions{
			IncludeOptional: true,
			GoTypes:         map[string]string{"first": "*int64", "cursor": "string", "id": "map[string]string"},
		})

		types := inputTypes(m)
		assert.Equal(t, "*int64", types["first"])
		assert.Equal(t, "string", types["cursor"], "an override that cannot be nil is as written")
		assert.Equal(t, "map[string]string", types["iD"])

		assert.Contains(t, m.UnsetParams, UnsetParam{Param: "first", Key: "first"})
		assert.Contains(t, m.UnsetParams, UnsetParam{Param: "iD", Key: "id"})
		assert.NotContains(t, m.UnsetParams, UnsetParam{Param: "cursor", Key: "cursor"}, "a string cannot be compared with nil")
	})

	t.Run("a type override that is a pointer already is not pointed to twice", func(t *testing.T) {
		t.Parallel()

		input := schema.Field{Name: "filter", Type: schema.TypeRef{Kind: schema.KindInputObject, Name: "FleetFilter"}}
		field := schema.Field{Name: "items", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}, Args: []schema.Field{input}}
		cfg := &config.PackageConfig{Types: []config.TypeConfig{{Name: "FleetFilter", FieldTypeOverride: "*FleetFilter"}}}

		m := goMethodForField(field, cfg, nil, methodArgOptions{IncludeOptional: true})

		assert.Equal(t, "*FleetFilter", m.Signature.Input[0].Type)
		assert.Equal(t, []UnsetParam{{Param: "filter", Key: "filter"}}, m.UnsetParams)
	})

	t.Run("an argument that is a path parameter is still that parameter", func(t *testing.T) {
		t.Parallel()

		scalar := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: n} }
		nonNull := schema.TypeRef{Kind: schema.KindNonNull, OfType: &schema.TypeRef{Kind: schema.KindScalar, Name: "Int"}}

		field := schema.Field{Name: "items", Type: scalar("String"), Args: []schema.Field{
			{Name: "accountId", Type: nonNull},
			{Name: "first", Type: scalar("Int")},
		}}
		path := map[string][]schema.Field{"account": {{Name: "id", Type: nonNull}}}

		m := goMethodForField(field, &config.PackageConfig{}, path, methodArgOptions{IncludeOptional: true})

		assert.Equal(t, []GoMethodInputType{{Name: "accountID", Type: "int"}, {Name: "First", Type: "*int"}}, m.Signature.Input)
		assert.Equal(t, []UnsetParam{{Param: "first", Key: "first"}}, m.UnsetParams)
	})
}

func TestGenerateGoMethods_IncludeOptionalArgumentsIsPerOperation(t *testing.T) {
	t.Parallel()

	obj := func(name string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindObject, Name: name} }
	scalar := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: n} }

	s := &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "thingUpdate", Type: obj("Thing"), Args: []schema.Field{
				{Name: "id", Type: schema.TypeRef{Kind: schema.KindNonNull, OfType: &schema.TypeRef{Kind: schema.KindScalar, Name: "ID"}}},
				{Name: "version", Type: scalar("Int")},
			}},
			{Name: "thingDelete", Type: obj("Thing"), Args: []schema.Field{{Name: "version", Type: scalar("Int")}}},
		}},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "things", Type: scalar("String"), Args: []schema.Field{{Name: "cursor", Type: scalar("String")}}},
				{Name: "others", Type: scalar("String"), Args: []schema.Field{{Name: "cursor", Type: scalar("String")}}},
			}},
			{Name: "Thing", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: scalar("String")}}},
			{Name: "ID", Kind: schema.KindScalar},
			{Name: "Int", Kind: schema.KindScalar},
			{Name: "String", Kind: schema.KindScalar},
		},
	}

	pkg := &config.PackageConfig{
		Name: "things",
		Queries: []config.Query{{Path: []string{"actor"}, Endpoints: []config.EndpointConfig{
			{Name: "things", IncludeOptionalArguments: true},
			{Name: "others"},
		}}},
		Mutations: []config.MutationConfig{
			{Name: "thingUpdate", IncludeOptionalArguments: true},
			{Name: "thingDelete"},
		},
	}

	queries, err := GenerateGoMethodQueriesForPackage(s, &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)
	require.Len(t, *queries, 2)

	byName := map[string]GoMethod{}
	for _, m := range *queries {
		byName[m.Name] = m
	}

	assert.Equal(t, "*string", byName["Things"].Signature.Input[0].Type)
	assert.Equal(t, []UnsetParam{{Param: "cursor", Key: "cursor"}}, byName["Things"].UnsetParams)
	assert.Contains(t, byName["Things"].QueryString, "$cursor: String", "the optional argument is declared")

	assert.Equal(t, "string", byName["Others"].Signature.Input[0].Type, "an endpoint without the key is generated as before")
	assert.Empty(t, byName["Others"].UnsetParams)
	assert.NotContains(t, byName["Others"].QueryString, "$cursor")

	mutations, err := GenerateGoMethodMutationsForPackage(s, &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)

	byName = map[string]GoMethod{}
	for _, m := range *mutations {
		byName[m.Name] = m
	}

	assert.Equal(t, []GoMethodInputType{{Name: "ID", Type: "int"}, {Name: "Version", Type: "*int"}}, byName["ThingUpdate"].Signature.Input)
	assert.Equal(t, []UnsetParam{{Param: "version", Key: "version"}}, byName["ThingUpdate"].UnsetParams)
	assert.Contains(t, byName["ThingUpdate"].QueryString, "$version: Int", "a mutation declares every argument already")

	assert.Equal(t, "int", byName["ThingDelete"].Signature.Input[0].Type)
	assert.Empty(t, byName["ThingDelete"].UnsetParams)
}

func TestNarrowedInterfaces(t *testing.T) {
	t.Parallel()

	expanded := []*schema.Type{
		{Name: "EntityManagementEntity", Kind: schema.KindInterface},
		{Name: "Other", Kind: schema.KindInterface},
		{Name: "AUnion", Kind: schema.KindUnion},
		{Name: "Plain", Kind: schema.KindObject},
	}

	pkg := &config.PackageConfig{Types: []config.TypeConfig{
		{Name: "EntityManagementEntity", IncludeImplementations: []string{"EntityManagementAiAgentEntity"}},
		{Name: "AUnion", IncludeImplementations: []string{"X"}},
		{Name: "Plain", IncludeImplementations: []string{"X"}},
		{Name: "Other"},
	}}

	assert.Equal(t, []string{"EntityManagementEntity"}, NarrowedInterfaces(pkg, &expanded), "interfaces only, and only those narrowed")
	assert.Empty(t, NarrowedInterfaces(pkg, nil))
	assert.Empty(t, NarrowedInterfaces(&config.PackageConfig{}, &expanded))
}
