//go:build unit
// +build unit

package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

func boolPtr(b bool) *bool { return &b }

func TestApplyFieldConfig(t *testing.T) {
	t.Parallel()

	base := func() GoStructField {
		return GoStructField{
			Name: "Enabled",
			Type: "bool",
			Tags: "`json:\"enabled,omitempty\"`",
		}
	}

	cases := map[string]struct {
		fc       *config.FieldConfig
		mutate   func(*GoStructField)
		wantName string
		wantType string
		wantTags string
	}{
		"nil config changes nothing": {
			fc: nil, wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"pointer": {
			fc: &config.FieldConfig{Pointer: boolPtr(true)}, wantName: "Enabled", wantType: "*bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"pointer false is a no-op": {
			fc: &config.FieldConfig{Pointer: boolPtr(false)}, wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"pointer on slice": {
			fc:       &config.FieldConfig{Pointer: boolPtr(true)},
			mutate:   func(f *GoStructField) { f.Type = "[]string" },
			wantName: "Enabled", wantType: "*[]string", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"pointer is not doubled": {
			fc:       &config.FieldConfig{Pointer: boolPtr(true), GoType: "*Foo"},
			wantName: "Enabled", wantType: "*Foo", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"omitempty off": {
			fc: &config.FieldConfig{OmitEmpty: boolPtr(false)}, wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled\"`",
		},
		"omitempty on adds it": {
			fc:       &config.FieldConfig{OmitEmpty: boolPtr(true)},
			mutate:   func(f *GoStructField) { f.Tags = "`json:\"enabled\"`" },
			wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"omitempty on is not doubled": {
			fc: &config.FieldConfig{OmitEmpty: boolPtr(true)}, wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"omitempty on applies to every tag key": {
			fc:       &config.FieldConfig{OmitEmpty: boolPtr(true)},
			mutate:   func(f *GoStructField) { f.Tags = "`json:\"enabled\" yaml:\"enabled\"`" },
			wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled,omitempty\" yaml:\"enabled,omitempty\"`",
		},
		"go type replaces the whole type": {
			fc:       &config.FieldConfig{GoType: "[]*Widget"},
			mutate:   func(f *GoStructField) { f.Type = "[]Widget" },
			wantName: "Enabled", wantType: "[]*Widget", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"go name": {
			fc: &config.FieldConfig{GoName: "EntityGuid"}, wantName: "EntityGuid", wantType: "bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
		"pointer ignored for interface field": {
			fc:       &config.FieldConfig{Pointer: boolPtr(true)},
			mutate:   func(f *GoStructField) { f.IsInterface = true },
			wantName: "Enabled", wantType: "bool", wantTags: "`json:\"enabled,omitempty\"`",
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := base()
			if tc.mutate != nil {
				tc.mutate(&f)
			}

			applyFieldConfig(&f, tc.fc)

			assert.Equal(t, tc.wantName, f.Name)
			assert.Equal(t, tc.wantType, f.Type)
			assert.Equal(t, tc.wantTags, f.Tags)
		})
	}
}

func TestCheckFieldConfigs(t *testing.T) {
	t.Parallel()

	fields := []schema.Field{{Name: "enabled"}, {Name: "nrql"}}

	assert.NoError(t, checkFieldConfigs("T", nil, fields))
	assert.NoError(t, checkFieldConfigs("T", &config.TypeConfig{Fields: []config.FieldConfig{{Name: "enabled"}}}, fields))

	err := checkFieldConfigs("T", &config.TypeConfig{Fields: []config.FieldConfig{{Name: "enabld"}}}, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"enabld"`)
}

func TestCheckExcludedArguments(t *testing.T) {
	t.Parallel()

	muts := []schema.Field{{Name: "del", Args: []schema.Field{{Name: "id"}, {Name: "version"}}}}

	assert.NoError(t, checkArgumentConfig(config.MutationConfig{Name: "del"}, muts))
	assert.NoError(t, checkArgumentConfig(config.MutationConfig{Name: "del", ExcludeArguments: []string{"version"}}, muts))

	err := checkArgumentConfig(config.MutationConfig{Name: "del", ExcludeArguments: []string{"versoin"}}, muts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"versoin"`)
}

func TestGoMethodForField_ExcludeArguments(t *testing.T) {
	t.Parallel()

	scalar := func(name string) schema.TypeRef { return schema.TypeRef{Kind: schema.KindScalar, Name: name} }

	field := schema.Field{
		Name: "entityManagementDelete",
		Type: scalar("String"),
		Args: []schema.Field{
			{Name: "id", Type: scalar("ID")},
			{Name: "version", Type: scalar("Int")},
		},
	}

	pkg := &config.PackageConfig{}

	all := goMethodForField(field, pkg, nil, methodArgOptions{})
	require.Len(t, all.Signature.Input, 2)
	require.Len(t, all.QueryVars, 2)

	left := goMethodForField(field, pkg, nil, methodArgOptions{Exclude: []string{"version"}})
	require.Len(t, left.Signature.Input, 1)
	assert.Equal(t, "int", left.Signature.Input[0].Type) // Tutone maps the ID scalar to int	require.Len(t, left.QueryVars, 1)
	assert.Equal(t, "id", left.QueryVars[0].Key)
}

func TestGetStructField_AppliesTypeFieldConfig(t *testing.T) {
	t.Parallel()

	parent := &schema.Type{Name: "LogInput", Kind: schema.KindInputObject}
	f := schema.Field{Name: "enabled", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "Boolean"}}

	pkg := &config.PackageConfig{Types: []config.TypeConfig{{
		Name: "LogInput",
		Fields: []config.FieldConfig{
			{Name: "enabled", Pointer: boolPtr(true), OmitEmpty: boolPtr(false)},
		},
	}}}

	got := getStructField(f, pkg, parent)

	assert.Equal(t, "*bool", got.Type)
	assert.Equal(t, "`json:\"enabled\"`", got.Tags)

	// A field with no override is untouched.
	plain := getStructField(f, &config.PackageConfig{}, parent)
	assert.Equal(t, "bool", plain.Type)
	assert.Equal(t, "`json:\"enabled,omitempty\"`", plain.Tags)
}
