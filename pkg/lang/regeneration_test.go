package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func TestMergeStructsByName(t *testing.T) {
	t.Parallel()

	t.Run("combines fields from both copies and keeps first-seen order", func(t *testing.T) {
		t.Parallel()

		got, err := mergeStructsByName([]GoStruct{
			{Name: "Actor", Fields: []GoStructField{{Name: "Cloud", Type: "Cloud"}}},
			{Name: "Other"},
			{Name: "Actor", Description: "late", Fields: []GoStructField{{Name: "Account", Type: "Account"}}},
		})
		require.NoError(t, err)
		require.Len(t, got, 2)

		assert.Equal(t, "Actor", got[0].Name)
		assert.Equal(t, "Other", got[1].Name)
		assert.Equal(t, "late", got[0].Description)
		require.Len(t, got[0].Fields, 2)
		assert.Equal(t, "Cloud", got[0].Fields[0].Name)
		assert.Equal(t, "Account", got[0].Fields[1].Name)
	})

	t.Run("a field present in both copies appears once", func(t *testing.T) {
		t.Parallel()

		got, err := mergeStructsByName([]GoStruct{
			{Name: "Actor", Fields: []GoStructField{{Name: "Cloud", Type: "Cloud"}}},
			{Name: "Actor", Fields: []GoStructField{{Name: "Cloud", Type: "Cloud"}}},
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Len(t, got[0].Fields, 1)
	})

	t.Run("conflicting field types are an error that names the field", func(t *testing.T) {
		t.Parallel()

		_, err := mergeStructsByName([]GoStruct{
			{Name: "Actor", Fields: []GoStructField{{Name: "Cloud", Type: "Cloud"}}},
			{Name: "Actor", Fields: []GoStructField{{Name: "Cloud", Type: "OtherCloud"}}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"Cloud"`)
		assert.Contains(t, err.Error(), `"Actor"`)
	})

	t.Run("flags and interfaces are combined", func(t *testing.T) {
		t.Parallel()

		got, err := mergeStructsByName([]GoStruct{
			{Name: "A", Implements: []string{"I"}},
			{Name: "A", Implements: []string{"I", "J"}, SpecialUnmarshal: true, GenerateGetters: true},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"I", "J"}, got[0].Implements)
		assert.True(t, got[0].SpecialUnmarshal)
		assert.True(t, got[0].GenerateGetters)
	})

	t.Run("no duplicates is a no-op", func(t *testing.T) {
		t.Parallel()

		in := []GoStruct{{Name: "A"}, {Name: "B"}}
		got, err := mergeStructsByName(in)
		require.NoError(t, err)
		assert.Equal(t, in, got)
	})
}

func TestBreakRecursiveTypes(t *testing.T) {
	t.Parallel()

	typeOf := func(structs []GoStruct, name, field string) string {
		for _, s := range structs {
			if s.Name != name {
				continue
			}

			for _, f := range s.Fields {
				if f.Name == field {
					return f.Type
				}
			}
		}

		return "<missing>"
	}

	t.Run("a struct containing itself gets a pointer", func(t *testing.T) {
		t.Parallel()

		s := []GoStruct{{Name: "Node", Fields: []GoStructField{{Name: "Next", Type: "Node"}}}}
		breakRecursiveTypes(s)
		assert.Equal(t, "*Node", typeOf(s, "Node", "Next"))
	})

	t.Run("two structs containing each other: exactly one side becomes a pointer", func(t *testing.T) {
		t.Parallel()

		s := []GoStruct{
			{Name: "CollaborationThread", Fields: []GoStructField{{Name: "Comment", Type: "CollaborationComment"}}},
			{Name: "CollaborationComment", Fields: []GoStructField{{Name: "Thread", Type: "CollaborationThread"}}},
		}
		breakRecursiveTypes(s)

		a := typeOf(s, "CollaborationThread", "Comment")
		b := typeOf(s, "CollaborationComment", "Thread")

		pointers := 0
		for _, ty := range []string{a, b} {
			if ty[0] == '*' {
				pointers++
			}
		}

		assert.Equal(t, 1, pointers, "got %q and %q", a, b)
	})

	t.Run("a longer cycle is broken once", func(t *testing.T) {
		t.Parallel()

		s := []GoStruct{
			{Name: "A", Fields: []GoStructField{{Name: "B", Type: "B"}}},
			{Name: "B", Fields: []GoStructField{{Name: "C", Type: "C"}}},
			{Name: "C", Fields: []GoStructField{{Name: "A", Type: "A"}}},
		}
		breakRecursiveTypes(s)

		assert.Equal(t, "B", typeOf(s, "A", "B"))
		assert.Equal(t, "C", typeOf(s, "B", "C"))
		assert.Equal(t, "*A", typeOf(s, "C", "A"))
	})

	t.Run("slices, pointers and interface fields are left alone", func(t *testing.T) {
		t.Parallel()

		s := []GoStruct{{Name: "Node", Fields: []GoStructField{
			{Name: "Children", Type: "[]Node", IsList: true},
			{Name: "Parent", Type: "*Node"},
			{Name: "Any", Type: "NodeInterface", IsInterface: true},
		}}}
		breakRecursiveTypes(s)

		assert.Equal(t, "[]Node", typeOf(s, "Node", "Children"))
		assert.Equal(t, "*Node", typeOf(s, "Node", "Parent"))
		assert.Equal(t, "NodeInterface", typeOf(s, "Node", "Any"))
	})

	t.Run("an acyclic graph is untouched", func(t *testing.T) {
		t.Parallel()

		s := []GoStruct{
			{Name: "A", Fields: []GoStructField{{Name: "B", Type: "B"}}},
			{Name: "B", Fields: []GoStructField{{Name: "S", Type: "string"}}},
		}
		breakRecursiveTypes(s)

		assert.Equal(t, "B", typeOf(s, "A", "B"))
	})

	t.Run("the same input always picks the same field", func(t *testing.T) {
		t.Parallel()

		build := func() []GoStruct {
			return []GoStruct{
				{Name: "X", Fields: []GoStructField{{Name: "Y", Type: "Y"}}},
				{Name: "Y", Fields: []GoStructField{{Name: "X", Type: "X"}}},
			}
		}

		first, second := build(), build()
		breakRecursiveTypes(first)
		breakRecursiveTypes(second)

		assert.Equal(t, first, second)
	})
}

func TestSafeParamName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "TypeArg", safeParamName("Type"))
	assert.Equal(t, "typeArg", safeParamName("type"))
	assert.Equal(t, "RangeArg", safeParamName("Range"))
	assert.Equal(t, "Name", safeParamName("Name"))
	assert.Equal(t, "relationtype", safeParamName("relationtype"))
}

func TestGoMethodForField_KeywordAndRenamedArguments(t *testing.T) {
	t.Parallel()

	scalar := func(name string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: name} }

	field := schema.Field{
		Name: "entityRelationshipUserDefinedCreateOrReplace",
		Type: scalar("String"),
		Args: []schema.Field{
			{Name: "type", Type: scalar("String")},
			{Name: "source", Type: scalar("String")},
		},
	}

	pkg := &config.PackageConfig{}

	t.Run("a keyword argument gets a legal name and keeps its GraphQL key", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(field, pkg, nil, methodArgOptions{})
		require.Len(t, m.Signature.Input, 2)
		assert.Equal(t, "TypeArg", m.Signature.Input[0].Name)
		assert.Equal(t, "type", m.QueryVars[0].Key, "the GraphQL variable must keep its real name")
		assert.Equal(t, "TypeArg", m.QueryVars[0].Value)
	})

	t.Run("a configured name wins", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(field, pkg, nil, methodArgOptions{Rename: map[string]string{"type": "relationtype"}})
		assert.Equal(t, "relationtype", m.Signature.Input[0].Name)
		assert.Equal(t, "type", m.QueryVars[0].Key)
	})

	t.Run("a configured name that is itself a keyword is still made safe", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(field, pkg, nil, methodArgOptions{Rename: map[string]string{"type": "range"}})
		assert.Equal(t, "rangeArg", m.Signature.Input[0].Name)
	})
}

func TestCheckArgumentConfig_Rename(t *testing.T) {
	t.Parallel()

	muts := []schema.Field{{Name: "m", Args: []schema.Field{{Name: "type"}}}}

	assert.NoError(t, checkArgumentConfig(config.MutationConfig{Name: "m", RenameArguments: map[string]string{"type": "relationtype"}}, muts))

	err := checkArgumentConfig(config.MutationConfig{Name: "m", RenameArguments: map[string]string{"tpye": "x"}}, muts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"tpye"`)
}

func TestCheckDuplicateMethodNames(t *testing.T) {
	t.Parallel()

	name := func(m GoMethod) string { return m.MethodName }

	require.NoError(t, checkDuplicateMethodNames("p", []GoMethod{{MethodName: "GetA"}, {MethodName: "GetB"}}, name))

	err := checkDuplicateMethodNames("p", []GoMethod{{MethodName: "GetDestinations"}, {MethodName: "GetDestinations"}}, name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetDestinations")
	assert.Contains(t, err.Error(), "method_name")
}

func TestEndpointResponseName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "destinationsResponse", endpointResponseName(config.EndpointConfig{Name: "destinations"}))
	assert.Equal(t, "getDestinationsAccountResponse", endpointResponseName(config.EndpointConfig{Name: "destinations", MethodName: "GetDestinationsAccount"}))
}

func TestPackageNamingDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	var g GolangGenerator

	g.SetPackageNaming(&config.PackageConfig{Name: "agentApplications"})
	assert.Equal(t, "agentApplications", g.PackageName)
	assert.Equal(t, "agentApplications", g.FileName)
	assert.Equal(t, "agentapplications", g.GoPackageName)
	assert.Equal(t, "AgentApplications", g.ClientTypeName)

	g.SetPackageNaming(&config.PackageConfig{
		Name:           "scorecards",
		FileName:       "servicearchintelligence",
		GoPackageName:  "servicearchintelligence",
		ClientTypeName: "ServiceArchIntelligence",
	})
	assert.Equal(t, "scorecards", g.PackageName)
	assert.Equal(t, "servicearchintelligence", g.FileName)
	assert.Equal(t, "servicearchintelligence", g.GoPackageName)
	assert.Equal(t, "ServiceArchIntelligence", g.ClientTypeName)
}

func TestCheckEndpointArguments(t *testing.T) {
	t.Parallel()

	nonNull := func(name string) schema.TypeRef {
		inner := schema.TypeRef{Kind: schema.KindScalar, Name: name}
		return schema.TypeRef{Kind: schema.KindNonNull, OfType: &inner}
	}

	field := schema.Field{Name: "managedAccounts", Args: []schema.Field{
		{Name: "isCanceled", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "Boolean"}},
		{Name: "id", Type: nonNull("Int")},
	}}

	require.NoError(t, checkEndpointArguments(config.EndpointConfig{Name: "managedAccounts"}, field))
	require.NoError(t, checkEndpointArguments(config.EndpointConfig{Name: "managedAccounts", ExcludeArguments: []string{"isCanceled"}}, field))

	err := checkEndpointArguments(config.EndpointConfig{Name: "managedAccounts", ExcludeArguments: []string{"isCancelled"}}, field)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"isCancelled"`)

	err = checkEndpointArguments(config.EndpointConfig{Name: "managedAccounts", ExcludeArguments: []string{"id"}}, field)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires")
}

func TestGoMethodForField_EndpointExcludeArguments(t *testing.T) {
	t.Parallel()

	field := schema.Field{
		Name: "managedAccounts",
		Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"},
		Args: []schema.Field{{Name: "isCanceled", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "Boolean"}}},
	}

	with := goMethodForField(field, &config.PackageConfig{}, nil, methodArgOptions{})
	assert.Len(t, with.Signature.Input, 1)

	without := goMethodForField(field, &config.PackageConfig{}, nil, methodArgOptions{Exclude: []string{"isCanceled"}})
	assert.Empty(t, without.Signature.Input, "the caller no longer passes it")
	assert.Empty(t, without.QueryVars, "and it is not sent")
}

func TestEndpointContextMethodName(t *testing.T) {
	t.Parallel()

	// The names a query method gets from an endpoint's settings, through the same
	// code the generator uses.
	cases := map[string]struct {
		endpoint        config.EndpointConfig
		method, context string
	}{
		"defaults":         {config.EndpointConfig{Name: "list"}, "GetList", "GetListWithContext"},
		"method name only": {config.EndpointConfig{Name: "destinations", MethodName: "GetDestinationsAccount"}, "GetDestinationsAccount", "GetDestinationsAccountWithContext"},
		"both set": {
			config.EndpointConfig{Name: "destinations", MethodName: "GetDestinationsAccount", ContextMethodName: "GetDestinationsWithContextAccount"},
			"GetDestinationsAccount", "GetDestinationsWithContextAccount",
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := &schema.Schema{Types: []*schema.Type{
				{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: schema.TypeRef{Kind: schema.KindObject, Name: "Actor"}}}},
				{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{{Name: tc.endpoint.Name, Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}}}},
				{Name: "String", Kind: schema.KindScalar},
			}}

			pkg := &config.PackageConfig{Name: "p", Queries: []config.Query{{Path: []string{"actor"}, Endpoints: []config.EndpointConfig{tc.endpoint}}}}

			methods, err := GenerateGoMethodQueriesForPackage(s, &config.GeneratorConfig{}, pkg)
			require.NoError(t, err)
			require.Len(t, *methods, 1)

			assert.Equal(t, tc.method, (*methods)[0].MethodName)
			assert.Equal(t, tc.context, (*methods)[0].ContextMethodName)
		})
	}
}

func TestArgumentGoTypes(t *testing.T) {
	t.Parallel()

	scalar := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: n} }

	field := schema.Field{
		Name: "entitySearch",
		Type: scalar("String"),
		Args: []schema.Field{
			{Name: "cursor", Type: scalar("String")},
			{Name: "query", Type: scalar("String")},
		},
	}

	m := goMethodForField(field, &config.PackageConfig{}, nil, methodArgOptions{GoTypes: map[string]string{"cursor": "*string"}})
	require.Len(t, m.Signature.Input, 2)
	assert.Equal(t, "*string", m.Signature.Input[0].Type, "the overridden argument")
	assert.Equal(t, "string", m.Signature.Input[1].Type, "other arguments are untouched")
	assert.Equal(t, "cursor", m.QueryVars[0].Key, "the GraphQL variable keeps its name")

	// Typos fail for both mutations and endpoints.
	err := checkArgumentConfig(config.MutationConfig{Name: "m", ArgumentGoTypes: map[string]string{"cursr": "*string"}},
		[]schema.Field{field})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cursr"`)

	err = checkEndpointArguments(config.EndpointConfig{Name: "entitySearch", ArgumentGoTypes: map[string]string{"cursr": "*string"}}, field)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cursr"`)

	require.NoError(t, checkEndpointArguments(config.EndpointConfig{Name: "entitySearch", ArgumentGoTypes: map[string]string{"cursor": "*string"}}, field))
}

// keepSchema has two query paths that both end at a `destinations` field, and two mutations.
func keepSchema() *schema.Schema {
	obj := func(name string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindObject, Name: name} }
	str := schema.TypeRef{Kind: schema.KindScalar, Name: "String"}

	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "thingCreate", Type: obj("Thing"), Args: []schema.Field{{Name: "name", Type: str}}},
			{Name: "thingDelete", Type: obj("Thing"), Args: []schema.Field{{Name: "id", Type: str}}},
		}},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "account", Type: obj("AccountScope")},
				{Name: "organization", Type: obj("OrganizationScope")},
			}},
			{Name: "AccountScope", Kind: schema.KindObject, Fields: []schema.Field{{Name: "destinations", Type: obj("Destination")}}},
			{Name: "OrganizationScope", Kind: schema.KindObject, Fields: []schema.Field{{Name: "destinations", Type: obj("Destination")}}},
			{Name: "Destination", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: str}}},
			{Name: "Thing", Kind: schema.KindObject, Fields: []schema.Field{{Name: "id", Type: str}}},
			{Name: "String", Kind: schema.KindScalar},
		},
	}
}

func destinationsPackage(account, organization config.EndpointConfig) *config.PackageConfig {
	return &config.PackageConfig{Name: "notifications", Queries: []config.Query{
		{Path: []string{"actor", "account"}, Endpoints: []config.EndpointConfig{account}},
		{Path: []string{"actor", "organization"}, Endpoints: []config.EndpointConfig{organization}},
	}}
}

func TestGenerateGoMethodQueries_KeepExistingEndpoints(t *testing.T) {
	t.Parallel()

	plain := config.EndpointConfig{Name: "destinations"}
	kept := config.EndpointConfig{Name: "destinations", KeepExisting: true}

	t.Run("two endpoints with one name are still an error", func(t *testing.T) {
		t.Parallel()

		_, err := GenerateGoMethodQueriesForPackage(keepSchema(), &config.GeneratorConfig{}, destinationsPackage(plain, plain))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"GetDestinations"`)
	})

	t.Run("kept endpoints are not generated and are not duplicates", func(t *testing.T) {
		t.Parallel()

		methods, err := GenerateGoMethodQueriesForPackage(keepSchema(), &config.GeneratorConfig{}, destinationsPackage(kept, kept))
		require.NoError(t, err)
		assert.Empty(t, *methods)
	})

	t.Run("a kept endpoint does not collide with the one that is generated", func(t *testing.T) {
		t.Parallel()

		methods, err := GenerateGoMethodQueriesForPackage(keepSchema(), &config.GeneratorConfig{}, destinationsPackage(kept, plain))
		require.NoError(t, err)
		require.Len(t, *methods, 1)
		assert.Equal(t, "GetDestinations", (*methods)[0].MethodName)
		assert.Contains(t, (*methods)[0].QueryString, "organization", "the one generated is the organization path")
	})

	t.Run("a kept endpoint is not checked, so its stale settings cannot fail the run", func(t *testing.T) {
		t.Parallel()

		stale := config.EndpointConfig{Name: "destinations", KeepExisting: true, ExcludeArguments: []string{"gone"}}

		_, err := GenerateGoMethodQueriesForPackage(keepSchema(), &config.GeneratorConfig{}, destinationsPackage(stale, kept))
		require.NoError(t, err)
	})
}

func TestConstrainedResponseStructs_KeptEndpointKeepsItsResponseStruct(t *testing.T) {
	t.Parallel()

	kept := config.EndpointConfig{Name: "destinations", KeepExisting: true}
	pkg := destinationsPackage(kept, kept)

	// The hand-written methods still read destinationsResponse, so it is generated.
	structs, err := constrainedResponseStructs(keepSchema(), pkg, &[]*schema.Type{})
	require.NoError(t, err)

	var names []string
	for _, s := range structs {
		names = append(names, s.Name)
	}

	assert.Contains(t, names, "destinationsResponse")
}

func TestGenerateGoMethodMutations_KeepExisting(t *testing.T) {
	t.Parallel()

	names := func(methods *[]GoMethod) []string {
		var out []string
		for _, m := range *methods {
			out = append(out, m.Name)
		}

		return out
	}

	t.Run("a kept mutation is left out", func(t *testing.T) {
		t.Parallel()

		pkg := &config.PackageConfig{Name: "things", Mutations: []config.MutationConfig{
			{Name: "thingCreate", KeepExisting: true},
			{Name: "thingDelete"},
		}}

		methods, err := GenerateGoMethodMutationsForPackage(keepSchema(), &config.GeneratorConfig{}, pkg)
		require.NoError(t, err)
		assert.Equal(t, []string{"ThingDelete"}, names(methods))
	})

	t.Run("all kept leaves nothing to generate, which is not an error", func(t *testing.T) {
		t.Parallel()

		pkg := &config.PackageConfig{Name: "things", Mutations: []config.MutationConfig{
			{Name: "thingCreate", KeepExisting: true},
			{Name: "thingDelete", KeepExisting: true},
		}}

		methods, err := GenerateGoMethodMutationsForPackage(keepSchema(), &config.GeneratorConfig{}, pkg)
		require.NoError(t, err)
		assert.Nil(t, methods)
	})

	t.Run("a mutation that matches nothing is still an error", func(t *testing.T) {
		t.Parallel()

		pkg := &config.PackageConfig{Name: "things", Mutations: []config.MutationConfig{
			{Name: "thingCreate", KeepExisting: true},
			{Name: "nothingLikeIt"},
		}}

		_, err := GenerateGoMethodMutationsForPackage(keepSchema(), &config.GeneratorConfig{}, pkg)
		require.Error(t, err)
	})

	t.Run("a kept mutation is not checked, and does not count as a duplicate", func(t *testing.T) {
		t.Parallel()

		pkg := &config.PackageConfig{Name: "things", Mutations: []config.MutationConfig{
			{Name: "thingCreate", KeepExisting: true, ExcludeArguments: []string{"gone"}},
			{Name: "thingCreate"},
		}}

		methods, err := GenerateGoMethodMutationsForPackage(keepSchema(), &config.GeneratorConfig{}, pkg)
		require.NoError(t, err)
		assert.Equal(t, []string{"ThingCreate"}, names(methods))
	})
}

func TestKeptNames(t *testing.T) {
	t.Parallel()

	expanded := &[]*schema.Type{
		{Name: "Alpha", Kind: schema.KindObject},
		{Name: "Beta", Kind: schema.KindObject},
		{Name: "Nrql", Kind: schema.KindScalar},
		{Name: "Color", Kind: schema.KindENUM},
		{Name: "Item", Kind: schema.KindInterface},
		{Name: "Gamma", Kind: schema.KindENUM},
	}

	pkg := &config.PackageConfig{Types: []config.TypeConfig{
		{Name: "Alpha", KeepExisting: true},
		{Name: "Beta"},
		{Name: "NRQL", KeepExisting: true}, // the Go spelling matches the schema's Nrql
		{Name: "Color", KeepExisting: true},
		{Name: "Item", KeepExisting: true},
		{Name: "NotExpanded", KeepExisting: true},
	}}

	assert.Equal(t,
		[]string{
			"Alpha",
			"NRQL",
			"Color", "ColorTypes", // the variable that lists an enum's values
			"Item", "ItemInterface", "UnmarshalItemInterface", // the Go interface and its unmarshal function
		},
		KeptNames(pkg, expanded),
		"Go names of the expanded types that are kept, with what the template declares for each kind")

	assert.Nil(t, KeptNames(&config.PackageConfig{Types: []config.TypeConfig{{Name: "Alpha"}}}, expanded))
	assert.Nil(t, KeptNames(pkg, nil))
}
