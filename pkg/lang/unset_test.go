package lang

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func unsetParams(m GoMethod) map[string]string {
	out := map[string]string{}
	for _, p := range m.UnsetParams {
		out[p.Param] = p.Unset
	}

	return out
}

func TestUnsetCondition(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"string":                   `%s == ""`,
		"int":                      "%s == 0",
		"int64":                    "%s == 0",
		"float64":                  "%s == 0",
		"bool":                     "!%s",
		"*string":                  "%s == nil",
		"[]string":                 "%s == nil",
		"map[string]string":        "%s == nil",
		"interface{}":              "%s == nil",
		"SortDirection":            "reflect.ValueOf(&%s).Elem().IsZero()",
		"FilterInput":              "reflect.ValueOf(&%s).Elem().IsZero()",
		"nrtime.EpochMilliseconds": "reflect.ValueOf(&%s).Elem().IsZero()",
	}

	for goType, want := range cases {
		assert.Equal(t, want, unsetCondition(goType), goType)
	}
}

func TestGoMethodForField_OmitZero(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{Types: []config.TypeConfig{{Name: "ID", FieldTypeOverride: "string"}}}

	t.Run("an optional argument is unset at the zero value of its type", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(optionalField(), pkg, nil, methodArgOptions{OmitZero: true})

		assert.Equal(t, map[string]string{
			"first":   "%s == 0",
			"cursor":  `%s == ""`,
			"iD":      `%s == ""`,
			"typeArg": `%s == ""`,
		}, unsetParams(m), "a required argument, such as name and ids, is always sent, and so is a list of non-null items, which the query has always declared")

		assert.Equal(t, "typeArg", m.UnsetParams[len(m.UnsetParams)-1].Param)
		assert.Equal(t, "type", m.UnsetParams[len(m.UnsetParams)-1].Key, "the variable keeps the name the schema gives it")

		assert.Equal(t, map[string]string{
			"first": "int", "cursor": "string", "filter": "[]string", "iDs": "[]string",
			"iD": "string", "name": "string", "typeArg": "string",
		}, inputTypes(m), "the parameters are as they were")
	})

	t.Run("an argument in include_arguments is always sent, as it was", func(t *testing.T) {
		t.Parallel()

		m := goMethodForField(optionalField(), pkg, nil, methodArgOptions{OmitZero: true, AlwaysSend: []string{"cursor", "first"}})

		assert.NotContains(t, unsetParams(m), "cursor")
		assert.NotContains(t, unsetParams(m), "first")
		assert.Contains(t, unsetParams(m), "iD")
	})

	t.Run("without the option nothing is unset", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, goMethodForField(optionalField(), pkg, nil, methodArgOptions{}).UnsetParams)
	})

	t.Run("an enum, an input object and an override that is not a basic type are compared through reflect", func(t *testing.T) {
		t.Parallel()

		field := schema.Field{Name: "items", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}, Args: []schema.Field{
			{Name: "sort", Type: schema.TypeRef{Kind: schema.KindENUM, Name: "SortDirection"}},
			{Name: "filter", Type: schema.TypeRef{Kind: schema.KindInputObject, Name: "FilterInput"}},
			{Name: "since", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "Seconds"}},
			{Name: "page", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "Int"}},
		}}

		m := goMethodForField(field, &config.PackageConfig{}, nil, methodArgOptions{
			OmitZero: true,
			GoTypes:  map[string]string{"page": "*int", "since": "nrtime.EpochSeconds"},
		})

		assert.Equal(t, map[string]string{
			"sort":   "reflect.ValueOf(&%s).Elem().IsZero()",
			"filter": "reflect.ValueOf(&%s).Elem().IsZero()",
			"since":  "reflect.ValueOf(&%s).Elem().IsZero()",
			"page":   "%s == nil",
		}, unsetParams(m))
	})

	t.Run("a list of non-null items is declared and sent as it was", func(t *testing.T) {
		t.Parallel()

		nonNull := schema.TypeRef{Kind: schema.KindNonNull, OfType: &schema.TypeRef{Kind: schema.KindScalar, Name: "String"}}
		field := schema.Field{Name: "items", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}, Args: []schema.Field{
			{Name: "tags", Type: schema.TypeRef{Kind: schema.KindList, OfType: &nonNull}},
		}}

		m := goMethodForField(field, &config.PackageConfig{}, nil, methodArgOptions{OmitZero: true})

		assert.Empty(t, m.UnsetParams, "the argument the query always declared keeps being sent")
	})
}

// pathSchema has actor { entitySearch(query, options) { counts(facet) } } and
// actor { account(id) { things(first) } }.
func pathSchema() *schema.Schema {
	scalar := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: n} }
	obj := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindObject, Name: n} }
	nonNull := func(r schema.TypeRef) schema.TypeRef { return schema.TypeRef{Kind: schema.KindNonNull, OfType: &r} }

	return &schema.Schema{Types: []*schema.Type{
		{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{{Name: "actor", Type: obj("Actor")}}},
		{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "entitySearch", Type: obj("Search"), Args: []schema.Field{
				{Name: "query", Type: scalar("String")},
				{Name: "options", Type: schema.TypeRef{Kind: schema.KindInputObject, Name: "SearchOptions"}},
				{Name: "scope", Type: nonNull(scalar("String"))},
			}},
		}},
		{Name: "Search", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "counts", Type: scalar("String"), Args: []schema.Field{{Name: "facet", Type: scalar("String")}}},
		}},
		{Name: "SearchOptions", Kind: schema.KindInputObject},
		{Name: "String", Kind: schema.KindScalar},
	}}
}

func TestGenerateGoMethods_PathArgumentsAreTakenDeclaredAndSent(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name: "search",
		Queries: []config.Query{{Path: []string{"actor", "entitySearch"}, Endpoints: []config.EndpointConfig{
			{Name: "counts"},
		}}},
	}

	methods, err := GenerateGoMethodQueriesForPackage(pathSchema(), &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)
	require.Len(t, *methods, 1)

	m := (*methods)[0]

	assert.Equal(t, []GoMethodInputType{
		{Name: "entitySearchQuery", Type: "string"},
		{Name: "entitySearchOptions", Type: "SearchOptions"},
		{Name: "entitySearchScope", Type: "string"},
		{Name: "Facet", Type: "string"},
	}, m.Signature.Input, "the path field's optional arguments are parameters too")

	// Every parameter has a variable, and the query declares it and names it on the field it belongs to.
	for _, in := range m.Signature.Input {
		assert.Contains(t, m.QueryString, "$"+lowerFirst(in.Name)+":", in.Name)
	}

	assert.Contains(t, m.QueryString, "entitySearch(query: $entitySearchQuery, options: $entitySearchOptions, scope: $entitySearchScope)")
	assert.Contains(t, m.QueryString, "$entitySearchScope: String!")
	assert.Contains(t, m.QueryString, "$entitySearchQuery: String")
	assert.Contains(t, m.QueryString, "counts(\n\tfacet: $facet,\n)")

	assert.Equal(t, map[string]string{
		"entitySearchOptions": "reflect.ValueOf(&%s).Elem().IsZero()",
		"entitySearchQuery":   `%s == ""`,
		"facet":               `%s == ""`,
	}, unsetParams(m), "only an optional argument can be left out")
}

func TestGenerateGoMethods_PathArgumentsWithIncludeOptional(t *testing.T) {
	t.Parallel()

	pkg := &config.PackageConfig{
		Name: "search",
		Queries: []config.Query{{Path: []string{"actor", "entitySearch"}, Endpoints: []config.EndpointConfig{
			{Name: "counts", IncludeOptionalArguments: true},
		}}},
	}

	methods, err := GenerateGoMethodQueriesForPackage(pathSchema(), &config.GeneratorConfig{}, pkg)
	require.NoError(t, err)

	m := (*methods)[0]

	assert.Equal(t, []GoMethodInputType{
		{Name: "entitySearchQuery", Type: "*string"},
		{Name: "entitySearchOptions", Type: "*SearchOptions"},
		{Name: "entitySearchScope", Type: "string"},
		{Name: "Facet", Type: "*string"},
	}, m.Signature.Input)

	assert.Equal(t, map[string]string{
		"entitySearchOptions": "",
		"entitySearchQuery":   "",
		"facet":               "",
	}, unsetParams(m), "a pointer is unset when nil")
}

func TestGenerateGoMethods_PathParametersFollowThePathOrder(t *testing.T) {
	t.Parallel()

	scalar := schema.TypeRef{Kind: schema.KindScalar, Name: "String"}
	obj := func(n string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindObject, Name: n} }
	nonNull := schema.TypeRef{Kind: schema.KindNonNull, OfType: &scalar}

	s := &schema.Schema{Types: []*schema.Type{
		{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "zeta", Type: obj("Zeta"), Args: []schema.Field{{Name: "zArg", Type: nonNull}}},
		}},
		{Name: "Zeta", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "alpha", Type: obj("Alpha"), Args: []schema.Field{{Name: "aArg", Type: nonNull}, {Name: "aCursor", Type: scalar}}},
		}},
		{Name: "Alpha", Kind: schema.KindObject, Fields: []schema.Field{
			{Name: "leaf", Type: scalar, Args: []schema.Field{{Name: "after", Type: scalar}}},
		}},
		{Name: "String", Kind: schema.KindScalar},
	}}

	pkg := &config.PackageConfig{
		Name: "deep",
		Queries: []config.Query{{Path: []string{"zeta", "alpha"}, Endpoints: []config.EndpointConfig{
			{Name: "leaf"},
		}}},
	}

	// The map the path arguments come from has no order, so run it more than once.
	for i := 0; i < 20; i++ {
		methods, err := GenerateGoMethodQueriesForPackage(s, &config.GeneratorConfig{}, pkg)
		require.NoError(t, err)
		require.Len(t, *methods, 1)

		var names []string
		for _, in := range (*methods)[0].Signature.Input {
			names = append(names, in.Name)
		}

		assert.Equal(t, []string{"zetaZArg", "alphaAArg", "alphaACursor", "After"}, names,
			"the outermost path field comes first, not the one that sorts first")
	}
}

func TestUnsetCondition_DoesNotPanicOnANilInterface(t *testing.T) {
	t.Parallel()

	// A named interface type is not nilable as far as the spelling shows, so it takes the
	// reflect form. That form has to hold for a nil interface value, which an
	// IsZero on reflect.ValueOf(x) itself would panic on.
	var unset interface{ Error() string }

	assert.NotPanics(t, func() {
		assert.True(t, reflect.ValueOf(&unset).Elem().IsZero())
	})

	assert.Equal(t, "reflect.ValueOf(&%s).Elem().IsZero()", unsetCondition("SomeInterface"))
}
