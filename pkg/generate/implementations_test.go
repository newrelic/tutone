package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func ref(kind schema.Kind, name string) schema.TypeRef { return schema.TypeRef{Kind: kind, Name: name} }

// ngepSchema models the shape that matters: a root field that returns an
// interface with several implementations.
func ngepSchema() *schema.Schema {
	return &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType", Kind: schema.KindObject},
		Types: []*schema.Type{
			{Name: "RootQueryType", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "actor", Type: ref(schema.KindObject, "Actor")},
			}},
			{Name: "Actor", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "entityManagement", Type: ref(schema.KindObject, "EntityManagement")},
			}},
			{Name: "EntityManagement", Kind: schema.KindObject, Fields: []schema.Field{
				{Name: "entity", Type: ref(schema.KindInterface, "EntityManagementEntityInterface")},
			}},
			{Name: "EntityManagementEntityInterface", Kind: schema.KindInterface, PossibleTypes: []schema.TypeRef{
				ref(schema.KindObject, "PipelineRule"),
				ref(schema.KindObject, "Scorecard"),
				ref(schema.KindObject, "Notebook"),
			}},
		},
	}
}

func possibleNames(t *testing.T, s *schema.Schema) []string {
	t.Helper()

	it, err := s.LookupTypeByName("EntityManagementEntityInterface")
	require.NoError(t, err)

	var names []string
	for _, p := range it.PossibleTypes {
		names = append(names, p.Name)
	}

	return names
}

func TestApplyImplementationFilters_OnType(t *testing.T) {
	t.Parallel()

	s := ngepSchema()
	pkg := &config.PackageConfig{Name: "pipelinecontrol", Types: []config.TypeConfig{
		{Name: "EntityManagementEntityInterface", IncludeImplementations: []string{"PipelineRule"}},
	}}

	restore, err := applyImplementationFilters(s, pkg)
	require.NoError(t, err)

	assert.Equal(t, []string{"PipelineRule"}, possibleNames(t, s))

	restore()
	assert.Equal(t, []string{"PipelineRule", "Scorecard", "Notebook"}, possibleNames(t, s))
}

func TestApplyImplementationFilters_OnEndpoint(t *testing.T) {
	t.Parallel()

	s := ngepSchema()
	pkg := &config.PackageConfig{Name: "scorecards", Queries: []config.Query{{
		Path: []string{"actor", "entityManagement"},
		Endpoints: []config.EndpointConfig{
			{Name: "entity", IncludeImplementations: []string{"Scorecard", "Notebook"}},
		},
	}}}

	restore, err := applyImplementationFilters(s, pkg)
	require.NoError(t, err)

	assert.Equal(t, []string{"Scorecard", "Notebook"}, possibleNames(t, s), "schema order is kept")

	restore()
	assert.Len(t, possibleNames(t, s), 3)
}

func TestApplyImplementationFilters_TypeAndEndpointSettingsAreCombined(t *testing.T) {
	t.Parallel()

	s := ngepSchema()
	pkg := &config.PackageConfig{
		Name:  "mixed",
		Types: []config.TypeConfig{{Name: "EntityManagementEntityInterface", IncludeImplementations: []string{"PipelineRule"}}},
		Queries: []config.Query{{
			Path:      []string{"actor", "entityManagement"},
			Endpoints: []config.EndpointConfig{{Name: "entity", IncludeImplementations: []string{"Notebook"}}},
		}},
	}

	restore, err := applyImplementationFilters(s, pkg)
	require.NoError(t, err)
	defer restore()

	assert.Equal(t, []string{"PipelineRule", "Notebook"}, possibleNames(t, s))
}

// One generate run covers every package against one schema. The filter for the
// first package must be gone by the time the second is generated.
func TestApplyImplementationFilters_DoesNotLeakIntoTheNextPackage(t *testing.T) {
	t.Parallel()

	s := ngepSchema()

	first := &config.PackageConfig{Name: "pipelinecontrol", Types: []config.TypeConfig{
		{Name: "EntityManagementEntityInterface", IncludeImplementations: []string{"PipelineRule"}},
	}}
	second := &config.PackageConfig{Name: "notebooks", Types: []config.TypeConfig{
		{Name: "EntityManagementEntityInterface", IncludeImplementations: []string{"Notebook"}},
	}}
	unfiltered := &config.PackageConfig{Name: "other"}

	restore, err := applyImplementationFilters(s, first)
	require.NoError(t, err)
	assert.Equal(t, []string{"PipelineRule"}, possibleNames(t, s))
	restore()

	restore, err = applyImplementationFilters(s, second)
	require.NoError(t, err, "the second package must see all three implementations to pick Notebook")
	assert.Equal(t, []string{"Notebook"}, possibleNames(t, s))
	restore()

	restore, err = applyImplementationFilters(s, unfiltered)
	require.NoError(t, err)
	assert.Equal(t, []string{"PipelineRule", "Scorecard", "Notebook"}, possibleNames(t, s), "a package with no filter keeps every implementation")
	restore()
}

func TestApplyImplementationFilters_UnknownImplementationIsAnErrorAndLeavesSchemaIntact(t *testing.T) {
	t.Parallel()

	s := ngepSchema()
	pkg := &config.PackageConfig{Name: "p", Types: []config.TypeConfig{
		{Name: "EntityManagementEntityInterface", IncludeImplementations: []string{"PipelineRule", "Typo"}},
	}}

	_, err := applyImplementationFilters(s, pkg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"Typo"`)
	assert.Len(t, possibleNames(t, s), 3, "a failed filter must not leave the schema narrowed")
}

func TestApplyImplementationFilters_UnknownTypeOrEndpointIsAnError(t *testing.T) {
	t.Parallel()

	_, err := applyImplementationFilters(ngepSchema(), &config.PackageConfig{Name: "p", Types: []config.TypeConfig{
		{Name: "NoSuchInterface", IncludeImplementations: []string{"X"}},
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchInterface")

	_, err = applyImplementationFilters(ngepSchema(), &config.PackageConfig{Name: "p", Queries: []config.Query{{
		Path:      []string{"actor", "entityManagement"},
		Endpoints: []config.EndpointConfig{{Name: "nope", IncludeImplementations: []string{"X"}}},
	}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestApplyImplementationFilters_NoSettingsChangesNothing(t *testing.T) {
	t.Parallel()

	s := ngepSchema()
	restore, err := applyImplementationFilters(s, &config.PackageConfig{Name: "p"})
	require.NoError(t, err)

	assert.Len(t, possibleNames(t, s), 3)
	restore()
	assert.Len(t, possibleNames(t, s), 3)
}

func TestWarnUnknownTypes(t *testing.T) {
	t.Parallel()

	s := ngepSchema()

	assert.Empty(t, warnUnknownTypes(s, &config.PackageConfig{Name: "p", Types: []config.TypeConfig{{Name: "Actor"}}}))

	got := warnUnknownTypes(s, &config.PackageConfig{Name: "p", Types: []config.TypeConfig{
		{Name: "Actor"}, {Name: "ActorNRQLInput"}, {Name: "Nope"},
	}})
	assert.Equal(t, []string{"ActorNRQLInput", "Nope"}, got)
}

func TestWarnUnknownTypes_CaseOnly(t *testing.T) {
	t.Parallel()

	s := ngepSchema()

	// create_as applies to a type whose name differs only in case.
	assert.Empty(t, warnUnknownTypes(s, &config.PackageConfig{Name: "p", Types: []config.TypeConfig{
		{Name: "ACTOR", CreateAs: "string"},
	}}))

	got := warnUnknownTypes(s, &config.PackageConfig{Name: "p", Types: []config.TypeConfig{
		{Name: "ACTOR", SkipFields: []string{"entityManagement"}},
	}})
	assert.Equal(t, []string{"ACTOR"}, got)
}

func TestExactNameSettings(t *testing.T) {
	t.Parallel()

	assert.Empty(t, exactNameSettings(config.TypeConfig{Name: "T", CreateAs: "string", SkipTypeCreate: true, GenerateStructGetters: true, InterfaceMethods: []string{"M()"}}))
	assert.Equal(t, []string{"field_type_override", "skip_fields", "struct_tags", "fields"}, exactNameSettings(config.TypeConfig{
		Name:              "T",
		FieldTypeOverride: "string",
		SkipFields:        []string{"a"},
		StructTags:        &config.StructTags{Tags: []string{"json"}},
		Fields:            []config.FieldConfig{{Name: "b"}},
	}))
}
