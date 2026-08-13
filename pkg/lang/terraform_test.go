//go:build unit

package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/schema"
)

func TestBuildTerraformField_scalar(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "name",
		Type: schema.TypeRef{
			Kind: schema.KindNonNull,
			OfType: &schema.TypeRef{
				Kind: schema.KindScalar,
				Name: "String",
			},
		},
	}

	tf := buildTerraformField(nil, f)

	assert.Equal(t, "name", tf.Name)
	assert.Equal(t, "TypeString", tf.TFType)
	assert.True(t, tf.Required)
	assert.False(t, tf.Optional)
	assert.False(t, tf.IsPointer)
}

func TestBuildTerraformField_optionalScalar(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "description",
		Type: schema.TypeRef{
			Kind: schema.KindScalar,
			Name: "String",
		},
	}

	tf := buildTerraformField(nil, f)

	assert.Equal(t, "TypeString", tf.TFType)
	assert.False(t, tf.Required)
	assert.True(t, tf.Optional)
	assert.True(t, tf.IsPointer)
}

func TestBuildTerraformField_intScalar(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "count",
		Type: schema.TypeRef{
			Kind: schema.KindScalar,
			Name: "Int",
		},
	}

	tf := buildTerraformField(nil, f)
	assert.Equal(t, "TypeInt", tf.TFType)
}

func TestBuildTerraformField_floatScalar(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "ratio",
		Type: schema.TypeRef{
			Kind: schema.KindScalar,
			Name: "Float",
		},
	}

	tf := buildTerraformField(nil, f)
	assert.Equal(t, "TypeFloat", tf.TFType)
}

func TestBuildTerraformField_boolScalar(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "enabled",
		Type: schema.TypeRef{
			Kind: schema.KindNonNull,
			OfType: &schema.TypeRef{
				Kind: schema.KindScalar,
				Name: "Boolean",
			},
		},
	}

	tf := buildTerraformField(nil, f)
	assert.Equal(t, "TypeBool", tf.TFType)
	assert.True(t, tf.Required)
}

func TestBuildTerraformField_unknownScalar(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "customField",
		Type: schema.TypeRef{
			Kind: schema.KindScalar,
			Name: "EpochMilliseconds",
		},
	}

	// Should not panic; should default to TypeString
	tf := buildTerraformField(nil, f)
	assert.Equal(t, "TypeString", tf.TFType)
}

func TestBuildTerraformField_enum(t *testing.T) {
	t.Parallel()

	s := &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType"},
		Types: []*schema.Type{
			{
				Name: "AlertsMutingRuleStatus",
				Kind: schema.KindENUM,
				EnumValues: []schema.EnumValue{
					{Name: "ACTIVE"},
					{Name: "INACTIVE"},
				},
			},
		},
	}

	f := schema.Field{
		Name: "status",
		Type: schema.TypeRef{
			Kind: schema.KindENUM,
			Name: "AlertsMutingRuleStatus",
		},
	}

	tf := buildTerraformField(s, f)

	assert.Equal(t, "TypeString", tf.TFType)
	assert.True(t, tf.IsEnum)
	assert.Equal(t, []string{"ACTIVE", "INACTIVE"}, tf.EnumValues)
}

func TestBuildTerraformField_nestedObject(t *testing.T) {
	t.Parallel()

	s := &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType"},
		Types: []*schema.Type{
			{
				Name: "ConditionGroupInput",
				Kind: schema.KindInputObject,
				InputFields: []schema.Field{
					{
						Name: "operator",
						Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"},
					},
				},
			},
		},
	}

	f := schema.Field{
		Name: "condition",
		Type: schema.TypeRef{
			Kind: schema.KindNonNull,
			OfType: &schema.TypeRef{
				Kind: schema.KindInputObject,
				Name: "ConditionGroupInput",
			},
		},
	}

	tf := buildTerraformField(s, f)

	assert.Equal(t, "TypeList", tf.TFType)
	assert.True(t, tf.IsNested)
	assert.Equal(t, 1, tf.MaxItems)
	assert.True(t, tf.Required)
	require.Len(t, tf.NestedFields, 1)
	assert.Equal(t, "operator", tf.NestedFields[0].Name)
}

func TestBuildTerraformField_listOfPrimitive(t *testing.T) {
	t.Parallel()

	f := schema.Field{
		Name: "tags",
		Type: schema.TypeRef{
			Kind: schema.KindNonNull,
			OfType: &schema.TypeRef{
				Kind: schema.KindList,
				OfType: &schema.TypeRef{
					Kind: schema.KindNonNull,
					OfType: &schema.TypeRef{
						Kind: schema.KindScalar,
						Name: "String",
					},
				},
			},
		},
	}

	tf := buildTerraformField(nil, f)

	assert.Equal(t, "TypeList", tf.TFType)
	assert.True(t, tf.IsListOfPrimitive)
	assert.False(t, tf.IsNested)
}

func TestDeriveClientService(t *testing.T) {
	t.Parallel()

	cases := []struct {
		importPath string
		expected   string
	}{
		{"github.com/newrelic/newrelic-client-go/v2/pkg/alerts", "Alerts"},
		{"github.com/newrelic/newrelic-client-go/v2/pkg/ainotifications", "Ainotifications"},
		{"github.com/newrelic/newrelic-client-go/v2/pkg/scorecards", "Scorecards"},
		{"", ""},
	}

	for _, c := range cases {
		got := DeriveClientService(c.importPath)
		assert.Equal(t, c.expected, got, "importPath=%q", c.importPath)
	}
}

func TestDeriveMethodName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutation string
		expected string
	}{
		{"alertsMutingRuleCreate", "AlertsMutingRuleCreateWithContext"},
		{"alertsMutingRuleDelete", "AlertsMutingRuleDeleteWithContext"},
		{"aiNotificationsCreateDestination", "AiNotificationsCreateDestinationWithContext"},
		{"", ""},
	}

	for _, c := range cases {
		got := DeriveMethodName(c.mutation)
		assert.Equal(t, c.expected, got, "mutation=%q", c.mutation)
	}
}

func TestSnakeToPascal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"alert_muting_rule", "AlertMutingRule"},
		{"notification_destination", "NotificationDestination"},
		{"scorecard", "Scorecard"},
		{"", ""},
	}

	for _, c := range cases {
		got := SnakeToPascal(c.input)
		assert.Equal(t, c.expected, got, "input=%q", c.input)
	}
}

func TestResolveBaseKind(t *testing.T) {
	t.Parallel()

	// NON_NULL → INPUT_OBJECT
	tr := &schema.TypeRef{
		Kind: schema.KindNonNull,
		OfType: &schema.TypeRef{
			Kind: schema.KindInputObject,
			Name: "SomeInput",
		},
	}
	kind, name := resolveBaseKind(tr)
	assert.Equal(t, schema.KindInputObject, kind)
	assert.Equal(t, "SomeInput", name)

	// Direct SCALAR
	tr2 := &schema.TypeRef{Kind: schema.KindScalar, Name: "String"}
	kind2, name2 := resolveBaseKind(tr2)
	assert.Equal(t, schema.KindScalar, kind2)
	assert.Equal(t, "String", name2)

	// nil
	kind3, name3 := resolveBaseKind(nil)
	assert.Equal(t, schema.Kind(""), kind3)
	assert.Equal(t, "", name3)
}

func TestBuildTerraformFields_sorted(t *testing.T) {
	t.Parallel()

	s := &schema.Schema{
		MutationType: &schema.Type{Name: "RootMutationType"},
		Types: []*schema.Type{
			{
				Name: "SomeInput",
				Kind: schema.KindInputObject,
				InputFields: []schema.Field{
					{Name: "zField", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}},
					{Name: "aField", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}},
					{Name: "mField", Type: schema.TypeRef{Kind: schema.KindScalar, Name: "String"}},
				},
			},
		},
	}

	fields, err := BuildTerraformFields(s, "SomeInput")
	require.NoError(t, err)
	require.Len(t, fields, 3)

	// Must be alphabetically sorted
	assert.Equal(t, "a_field", fields[0].Name)
	assert.Equal(t, "m_field", fields[1].Name)
	assert.Equal(t, "z_field", fields[2].Name)
}
