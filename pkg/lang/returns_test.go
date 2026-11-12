package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func leafOf(kind schema.Kind, name string) schema.TypeRef {
	return schema.TypeRef{Kind: kind, Name: name}
}

func wrapped(kind schema.Kind, of schema.TypeRef) schema.TypeRef {
	return schema.TypeRef{Kind: kind, OfType: &of}
}

func TestSetReturnAlias(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		returnType string
		wantAlias  bool
	}{
		"list of strings":                 {"[]string", true},
		"bool":                            {"bool", true},
		"string":                          {"string", true},
		"pointer":                         {"*Thing", true},
		"list of objects":                 {"[]Thing", true},
		"type from another package":       {"nrtime.EpochSeconds", true},
		"object":                          {"Thing", false},
		"named scalar":                    {"EpochMilliseconds", false},
		"interface":                       {"EntityInterface", false},
		"an identifier that is not typed": {"thing", true},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := GoMethod{Name: "ThingAdd", Signature: GoMethodSignature{Return: []string{tc.returnType, "error"}}}

			setReturnAlias(&m)

			assert.Equal(t, "error", m.Signature.Return[1])

			if !tc.wantAlias {
				assert.Equal(t, tc.returnType, m.Signature.Return[0])
				assert.Empty(t, m.ReturnAlias)
				assert.Empty(t, m.ReturnType)

				return
			}

			assert.Equal(t, "tutoneResultThingAdd", m.ReturnAlias)
			assert.Equal(t, m.ReturnAlias, m.Signature.Return[0], "the template prints the alias")
			assert.Equal(t, tc.returnType, m.ReturnType)
		})
	}
}

// returnsSchema has mutations that return a scalar, a list of IDs, an enum, an object and
// an object without arguments.
func returnsSchema() *schema.Schema {
	id := wrapped(schema.KindNonNull, leafOf(schema.KindScalar, "ID"))

	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "thingAdd", Type: wrapped(schema.KindList, leafOf(schema.KindScalar, "ID")), Args: []schema.Field{
				{Name: "ids", Type: wrapped(schema.KindNonNull, wrapped(schema.KindList, id))},
			}},
			{Name: "thingPing", Type: wrapped(schema.KindNonNull, leafOf(schema.KindScalar, "Boolean"))},
			{Name: "thingState", Type: leafOf(schema.KindENUM, "ThingState")},
			{Name: "thingStamp", Type: leafOf(schema.KindScalar, "EpochSeconds")},
			{Name: "thingRegister", Type: leafOf(schema.KindObject, "Thing")},
			{Name: "thingCreate", Type: leafOf(schema.KindObject, "Thing"), Args: []schema.Field{
				{Name: "name", Type: leafOf(schema.KindScalar, "String")},
			}},
		}},
		Types: []*schema.Type{
			{Name: "Thing", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: leafOf(schema.KindScalar, "String")}}},
			{Name: "ThingState", Kind: schema.KindENUM},
			{Name: "ID", Kind: schema.KindScalar},
			{Name: "Boolean", Kind: schema.KindScalar},
			{Name: "String", Kind: schema.KindScalar},
			{Name: "EpochSeconds", Kind: schema.KindScalar},
		},
	}
}

func TestGenerateGoMethodMutations_ReturnTypes(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name: "things",
		Types: []config.TypeConfig{
			{Name: "ID", FieldTypeOverride: "string"},
			{Name: "EpochSeconds", FieldTypeOverride: "nrtime.EpochSeconds"},
		},
		Mutations: []config.MutationConfig{{Name: "thing.*", MaxQueryFieldDepth: 1}},
	}

	methods, err := GenerateGoMethodMutationsForPackage(returnsSchema(), &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)

	byName := map[string]GoMethod{}
	for _, m := range *methods {
		byName[m.Name] = m
	}

	cases := map[string]struct {
		returnType string
		alias      bool
		query      string
	}{
		"ThingAdd":      {"[]string", true, "mutation(\n\t$ids: [ID!]!,\n) { thingAdd(\n\tids: $ids,\n) }"},
		"ThingPing":     {"bool", true, "mutation { thingPing }"},
		"ThingState":    {"ThingState", false, "mutation { thingState }"},
		"ThingStamp":    {"nrtime.EpochSeconds", true, "mutation { thingStamp }"},
		"ThingRegister": {"Thing", false, "mutation { thingRegister {\n\tid\n} }"},
		"ThingCreate":   {"Thing", false, "mutation(\n\t$name: String,\n) { thingCreate(\n\tname: $name,\n) {\n\tid\n} }"},
	}

	require.Len(t, byName, len(cases))

	for name, tc := range cases {
		m := byName[name]

		assert.Equal(t, tc.query, m.QueryString, name)

		if tc.alias {
			assert.Equal(t, tc.returnType, m.ReturnType, name)
			assert.Equal(t, "tutoneResult"+name, m.Signature.Return[0], name)
		} else {
			assert.Equal(t, tc.returnType, m.Signature.Return[0], name)
			assert.Empty(t, m.ReturnAlias, name)
		}
	}

	assert.True(t, byName["ThingAdd"].Signature.ReturnSlice)
}

func TestArgumentlessMutationResponseStructs(t *testing.T) {
	t.Parallel()

	newPkg := func(generators ...string) *config.PackageConfig {
		return &config.PackageConfig{
			Name:       "things",
			Generators: generators,
			Types:      []config.TypeConfig{{Name: "EpochSeconds", FieldTypeOverride: "nrtime.EpochSeconds"}},
			Mutations:  []config.MutationConfig{{Name: "thing.*", MaxQueryFieldDepth: 1}},
		}
	}

	t.Run("only mutations that send no variables get a struct", func(t *testing.T) {
		t.Parallel()

		structs, err := argumentlessMutationResponseStructs(returnsSchema(), newPkg("typegen", "nerdgraphclient"))
		require.NoError(t, err)

		got := map[string]GoStruct{}
		for _, st := range structs {
			got[st.Name] = st
		}

		require.Len(t, got, 4, "ThingAdd and ThingCreate send variables, so the template declares theirs")

		// A return type that is an identifier is the name of the field the method reads.
		register := got["ThingRegisterQueryResponse"]
		require.Len(t, register.Fields, 1)
		assert.Equal(t, GoStructField{Name: "Thing", Type: "Thing", TypeName: "Thing", Tags: "`json:\"ThingRegister\"`", TagKey: "thingRegister"}, register.Fields[0])

		state := got["ThingStateQueryResponse"]
		assert.Equal(t, "ThingState", state.Fields[0].Name)

		// One that is not is held in the field every rewritten method reads.
		ping := got["ThingPingQueryResponse"]
		assert.Equal(t, GoStructField{Name: MutationResultField, Type: "bool", TypeName: "bool", Tags: "`json:\"ThingPing\"`", TagKey: "thingPing"}, ping.Fields[0])

		stamp := got["ThingStampQueryResponse"]
		assert.Equal(t, GoStructField{Name: MutationResultField, Type: "nrtime.EpochSeconds", TypeName: "nrtime.EpochSeconds", Tags: "`json:\"ThingStamp\"`", TagKey: "thingStamp"}, stamp.Fields[0])
	})

	t.Run("a package without the client generator has no use for them", func(t *testing.T) {
		t.Parallel()

		structs, err := argumentlessMutationResponseStructs(returnsSchema(), newPkg("typegen"))
		require.NoError(t, err)
		assert.Empty(t, structs)
	})

	t.Run("a package without mutations has none", func(t *testing.T) {
		t.Parallel()

		pkg := newPkg("typegen", "nerdgraphclient")
		pkg.Mutations = nil

		structs, err := argumentlessMutationResponseStructs(returnsSchema(), pkg)
		require.NoError(t, err)
		assert.Empty(t, structs)
	})

	t.Run("a kept mutation has none", func(t *testing.T) {
		t.Parallel()

		pkg := newPkg("typegen", "nerdgraphclient")
		pkg.Mutations = []config.MutationConfig{{Name: "thingPing", KeepExisting: true}}

		structs, err := argumentlessMutationResponseStructs(returnsSchema(), pkg)
		require.NoError(t, err)
		assert.Empty(t, structs)
	})

	t.Run("a mutation whose only variable is excluded sends none", func(t *testing.T) {
		t.Parallel()

		s := returnsSchema()
		pkg := newPkg("typegen", "nerdgraphclient")
		pkg.Mutations = []config.MutationConfig{{Name: "thingCreate", MaxQueryFieldDepth: 1, ExcludeArguments: []string{"name"}}}

		structs, err := argumentlessMutationResponseStructs(s, pkg)
		require.NoError(t, err)
		require.Len(t, structs, 1)
		assert.Equal(t, "ThingCreateQueryResponse", structs[0].Name)
	})
}

func TestGenerateGoTypes_IncludesTheArgumentlessMutationResponses(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name:       "things",
		Generators: []string{"typegen", "nerdgraphclient"},
		Types:      []config.TypeConfig{{Name: "Thing"}},
		Mutations:  []config.MutationConfig{{Name: "thingRegister", MaxQueryFieldDepth: 1}},
	}

	s := returnsSchema()

	expanded, err := schema.ExpandTypes(s, pkg)
	require.NoError(t, err)

	structs, _, _, _, err := GenerateGoTypesForPackage(s, &config.GeneratorConfig{}, pkg, expanded)
	require.NoError(t, err)

	var names []string
	for _, st := range *structs {
		names = append(names, st.Name)
	}

	assert.Contains(t, names, "ThingRegisterQueryResponse")
}

// abstractResultsSchema has mutations that return a union, a list of it and an interface
// without arguments, and one that returns the union and has an argument.
func abstractResultsSchema() *schema.Schema {
	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "searchOne", Type: leafOf(schema.KindUnion, "SearchResult")},
			{Name: "searchAll", Type: wrapped(schema.KindList, leafOf(schema.KindUnion, "SearchResult"))},
			{Name: "entityOne", Type: leafOf(schema.KindInterface, "Entity")},
			{Name: "searchFor", Type: leafOf(schema.KindUnion, "SearchResult"), Args: []schema.Field{
				{Name: "term", Type: leafOf(schema.KindScalar, "String")},
			}},
		}},
		Types: []*schema.Type{
			{Name: "SearchResult", Kind: schema.KindUnion, PossibleTypes: []schema.TypeRef{leafOf(schema.KindObject, "Template")}},
			{Name: "Entity", Kind: schema.KindInterface, Fields: []schema.Field{
				{Name: "id", Type: leafOf(schema.KindScalar, "String")},
			}, PossibleTypes: []schema.TypeRef{leafOf(schema.KindObject, "Template")}},
			{Name: "Template", Kind: schema.KindObject, Interfaces: []schema.TypeRef{leafOf(schema.KindInterface, "Entity")}, Fields: []schema.Field{
				{Name: "id", Type: leafOf(schema.KindScalar, "String")},
			}},
			{Name: "String", Kind: schema.KindScalar},
		},
	}
}

func TestGenerateGoTypes_ArgumentlessMutationResponseDecodesAnInterfaceOrUnionResult(t *testing.T) {
	t.Parallel()

	newPkg := func(types ...config.TypeConfig) *config.PackageConfig {
		return &config.PackageConfig{
			Name:       "search",
			Generators: []string{"typegen", "nerdgraphclient"},
			Types:      types,
			Mutations:  []config.MutationConfig{{Name: "search.*", MaxQueryFieldDepth: 2}, {Name: "entityOne", MaxQueryFieldDepth: 2}},
		}
	}

	generate := func(t *testing.T, pkg *config.PackageConfig) map[string]GoStruct {
		t.Helper()

		s := abstractResultsSchema()

		expanded, err := schema.ExpandTypes(s, pkg)
		require.NoError(t, err)

		structs, _, _, _, err := GenerateGoTypesForPackage(s, &config.GeneratorConfig{}, pkg, expanded)
		require.NoError(t, err)

		byName := map[string]GoStruct{}
		for _, st := range *structs {
			byName[st.Name] = st
		}

		return byName
	}

	t.Run("the generated interface decodes it by __typename", func(t *testing.T) {
		t.Parallel()

		structs := generate(t, newPkg())

		// The rendered method reads the field named after its return type.
		single := structs["SearchOneQueryResponse"]
		assert.True(t, single.SpecialUnmarshal)
		assert.Equal(t, []GoStructField{{
			Name: "SearchResultInterface", Type: "SearchResultInterface", TypeName: "SearchResult",
			Tags: "`json:\"SearchOne\"`", TagKey: "searchOne", IsInterface: true,
		}}, single.Fields)

		// A list is not an identifier, so it is held in the result field.
		list := structs["SearchAllQueryResponse"]
		assert.True(t, list.SpecialUnmarshal)
		assert.Equal(t, []GoStructField{{
			Name: MutationResultField, Type: "[]SearchResultInterface", TypeName: "SearchResult",
			Tags: "`json:\"SearchAll\"`", TagKey: "searchAll", IsInterface: true, IsList: true,
		}}, list.Fields)

		entity := structs["EntityOneQueryResponse"]
		assert.True(t, entity.SpecialUnmarshal)
		assert.Equal(t, "Entity", entity.Fields[0].TypeName)
		assert.Equal(t, "entityOne", entity.Fields[0].TagKey)
		assert.True(t, entity.Fields[0].IsInterface)

		assert.NotContains(t, structs, "SearchForQueryResponse", "the template declares the struct of a mutation with variables")
	})

	t.Run("a union the package supplies is decoded as the type it gives", func(t *testing.T) {
		t.Parallel()

		structs := generate(t, newPkg(config.TypeConfig{Name: "SearchResult", SkipTypeCreate: true, FieldTypeOverride: "ai.SearchResult"}))

		single := structs["SearchOneQueryResponse"]
		assert.False(t, single.SpecialUnmarshal)
		assert.False(t, single.Fields[0].IsInterface)
		assert.Equal(t, MutationResultField, single.Fields[0].Name)
		assert.Equal(t, "ai.SearchResult", single.Fields[0].Type)

		assert.False(t, structs["SearchAllQueryResponse"].SpecialUnmarshal)
		assert.True(t, structs["EntityOneQueryResponse"].SpecialUnmarshal, "an interface the package does generate is still decoded")
	})

	t.Run("an interface the package supplies has no function to decode it", func(t *testing.T) {
		t.Parallel()

		entity := generate(t, newPkg(config.TypeConfig{Name: "Entity", SkipTypeCreate: true}))["EntityOneQueryResponse"]

		assert.False(t, entity.SpecialUnmarshal, "Unmarshal<Name>Interface would not exist")
		assert.True(t, entity.Fields[0].IsInterface)
	})
}

func TestGoMethodForField_AnArgumentThatNamesAPathParameterIsThatParameter(t *testing.T) {
	t.Parallel()

	nonNull := func(name string) schema.TypeRef { return wrapped(schema.KindNonNull, leafOf(schema.KindScalar, name)) }

	pathFields := map[string][]schema.Field{"account": {{Name: "id", Type: nonNull("Int")}}}
	pkg := &config.PackageConfig{}

	endpoint := func(args ...schema.Field) schema.Field {
		return schema.Field{Name: "details", Type: leafOf(schema.KindObject, "Details"), Args: args}
	}

	t.Run("it is taken once and sent for both", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(endpoint(
			schema.Field{Name: "accountId", Type: nonNull("Int")},
			schema.Field{Name: "region", Type: nonNull("String")},
		), pkg, pathFields, methodArgOptions{})

		assert.Equal(t, []GoMethodInputType{{Name: "accountID", Type: "int"}, {Name: "Region", Type: "string"}}, m.Signature.Input)
		assert.Equal(t, []QueryVar{
			{Key: "accountID", Value: "accountID", Type: "int"},
			{Key: "accountId", Value: "accountID", Type: "Int"},
			{Key: "region", Value: "Region", Type: "String"},
		}, m.QueryVars)
		require.NoError(t, checkUniqueParameters(m))
	})

	t.Run("a different type is a different parameter, which is reported", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(endpoint(schema.Field{Name: "accountId", Type: nonNull("String")}), pkg, pathFields, methodArgOptions{})

		assert.Len(t, m.Signature.Input, 2)

		err := checkUniqueParameters(m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"accountID"`)
		assert.Contains(t, err.Error(), "exclude_arguments")
	})

	t.Run("an excluded argument is not sent", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(endpoint(schema.Field{Name: "accountId", Type: nonNull("Int")}), pkg, pathFields, methodArgOptions{Exclude: []string{"accountId"}})

		assert.Equal(t, []GoMethodInputType{{Name: "accountID", Type: "int"}}, m.Signature.Input)
		assert.Equal(t, []QueryVar{{Key: "accountID", Value: "accountID", Type: "int"}}, m.QueryVars)
	})

	t.Run("a mutation has no path, so its arguments are never shared", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(endpoint(
			schema.Field{Name: "accountId", Type: nonNull("Int")},
			schema.Field{Name: "other", Type: nonNull("Int")},
		), pkg, nil, methodArgOptions{Rename: map[string]string{"other": "AccountID"}})

		assert.Len(t, m.Signature.Input, 2)
		assert.Error(t, checkUniqueParameters(m))
	})
}

func TestCheckUniqueParameters(t *testing.T) {
	t.Parallel()

	method := func(names ...string) GoMethod {
		m := GoMethod{Name: "GetThing"}
		for _, n := range names {
			m.Signature.Input = append(m.Signature.Input, GoMethodInputType{Name: n, Type: "int"})
		}

		return m
	}

	require.NoError(t, checkUniqueParameters(method()))
	require.NoError(t, checkUniqueParameters(method("accountID", "region")))

	// The templates lower-case the first letter, so these two are one parameter.
	err := checkUniqueParameters(method("AccountID", "accountID"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"GetThing"`)
	assert.Contains(t, err.Error(), `"accountID"`)
}

func TestGenerateGoMethodQueries_AccountPathEndpointWithItsOwnAccountIdArgument(t *testing.T) {
	t.Parallel()

	nonNull := func(name string) schema.TypeRef { return wrapped(schema.KindNonNull, leafOf(schema.KindScalar, name)) }
	obj := func(name string) schema.TypeRef { return leafOf(schema.KindObject, name) }

	s := &schema.Schema{Types: []*schema.Type{
		{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
		{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "account", Type: obj("Account"), Args: []schema.Field{{Name: "id", Type: nonNull("Int")}}},
		}},
		{Name: "Account", Kind: schema.KindObject, Fields: []schema.Field{{Name: "cloud", Type: obj("Cloud")}}},
		{Name: "Cloud", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "details", Type: obj("Details"), Args: []schema.Field{
				{Name: "accountId", Type: nonNull("Int")},
				{Name: "other", Type: nonNull("Int")},
			}},
		}},
		{Name: "Details", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: leafOf(schema.KindScalar, "String")}}},
		{Name: "Int", Kind: schema.KindScalar},
		{Name: "String", Kind: schema.KindScalar},
	}}

	pkg := &config.PackageConfig{Name: "cloud", Queries: []config.Query{{
		Path:      []string{"actor", "account", "cloud"},
		Endpoints: []config.EndpointConfig{{Name: "details", MaxQueryFieldDepth: 1}},
	}}}

	methods, err := GenerateGoMethodQueriesForPackage(s, &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)
	require.Len(t, *methods, 1)

	m := (*methods)[0]
	assert.Equal(t, []GoMethodInputType{{Name: "accountID", Type: "int"}, {Name: "Other", Type: "int"}}, m.Signature.Input)
	assert.Contains(t, m.QueryString, "account(id: $accountID)")
	assert.Contains(t, m.QueryString, "accountId: $accountId")
}

func TestConstrainedResponseStructs_TypeWhoseGoNameDiffersFromItsGraphQLName(t *testing.T) {
	t.Parallel()

	obj := func(name string) schema.TypeRef { return leafOf(schema.KindObject, name) }

	s := &schema.Schema{MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject}, Types: []*schema.Type{
		{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
		{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{{Name: "notifications", Type: obj("Notifications")}}},
		{Name: "Notifications", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "oAuthUrl", Type: obj("OAuthUrlResponse")},
			{Name: "other", Type: obj("NotExpanded")},
		}},
		{Name: "OAuthUrlResponse", Kind: schema.KindObject, Fields: []schema.Field{{Name: "url", Type: leafOf(schema.KindScalar, "String")}}},
		{Name: "NotExpanded", Kind: schema.KindObject},
		{Name: "String", Kind: schema.KindScalar},
	}}

	pkg := &config.PackageConfig{
		Name:    "notifications",
		Types:   []config.TypeConfig{{Name: "OAuthUrlResponse"}},
		Queries: []config.Query{{Path: []string{"actor", "notifications"}, Endpoints: []config.EndpointConfig{{Name: "oAuthUrl", MaxQueryFieldDepth: 1}}}},
	}

	expanded, err := schema.ExpandTypes(s, pkg)
	require.NoError(t, err)

	structs, err := constrainedResponseStructs(s, pkg, expanded)
	require.NoError(t, err)

	var notifications *GoStruct

	for i := range structs {
		if structs[i].Name == "Notifications" {
			notifications = &structs[i]
		}
	}

	require.NotNil(t, notifications)

	// Its Go name is OAuthURLResponse, which is what the package declares, so the
	// method's read of resp.Actor.Notifications.OAuthURL has a field to read.
	require.Len(t, notifications.Fields, 1)
	assert.Equal(t, "OAuthURL", notifications.Fields[0].Name)
	assert.Equal(t, "OAuthURLResponse", notifications.Fields[0].Type)
}
