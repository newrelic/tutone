//go:build unit

package terraform

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

// loadTestSchema loads the fixture schema for unit tests.
func loadTestSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Load("testdata/schema.json")
	require.NoError(t, err)
	return s
}

func minimalGenConfig() *config.GeneratorConfig {
	return &config.GeneratorConfig{Name: "terraform"}
}

// TestGenerate_nilTerraformConfig verifies Generate returns nil when no terraform block is set.
func TestGenerate_nilTerraformConfig(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name: "alertmutingrule",
		// No Terraform field
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	assert.NoError(t, err)
	// Generator state should be zero-value — nothing was set
	assert.Empty(t, g.ResourceName)
}

// TestGenerate_populatesFields verifies all key TerraformGenerator fields are populated correctly.
func TestGenerate_populatesFields(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name:       "alertmutingrule",
		ImportPath: "github.com/newrelic/newrelic-client-go/v2/pkg/alerts",
		Mutations: []config.MutationConfig{
			{Name: "alertsMutingRuleCreate"},
			{Name: "alertsMutingRuleUpdate"},
			{Name: "alertsMutingRuleDelete"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName:      "alert_muting_rule",
			ReadMethod:        "GetMutingRuleWithContext",
			ReadType:          "direct",
			RequiresAccountID: true,
			IDType:            "int",
			ComputedFields:    []string{"account_id"},
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)

	assert.Equal(t, "alert_muting_rule", g.ResourceName)
	assert.Equal(t, "AlertMutingRule", g.ResourceTypeName)
	assert.Equal(t, "resourceNewRelicAlertMutingRule", g.FuncPrefix)
	assert.Equal(t, "Alerts", g.ClientService)
	assert.Equal(t, "AlertsMutingRuleCreateWithContext", g.CreateMethod)
	assert.Equal(t, "AlertsMutingRuleDeleteWithContext", g.DeleteMethod)
	assert.Equal(t, "GetMutingRuleWithContext", g.ReadMethod)
	assert.Equal(t, "AlertsMutingRuleCreateInput", g.CreateInputType)
	assert.True(t, g.HasUpdate)
	assert.Equal(t, "AlertsMutingRuleUpdateWithContext", g.UpdateMethod)
}

// TestGenerate_schemaFieldsSorted verifies SchemaFields are alphabetically sorted.
func TestGenerate_schemaFieldsSorted(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name:       "alertmutingrule",
		ImportPath: "github.com/newrelic/newrelic-client-go/v2/pkg/alerts",
		Mutations: []config.MutationConfig{
			{Name: "alertsMutingRuleCreate"},
			{Name: "alertsMutingRuleDelete"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName: "alert_muting_rule",
			ReadMethod:   "GetMutingRuleWithContext",
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)
	require.NotEmpty(t, g.SchemaFields)

	for i := 1; i < len(g.SchemaFields); i++ {
		assert.True(t, g.SchemaFields[i-1].Name <= g.SchemaFields[i].Name,
			"fields not sorted at index %d: %q > %q", i, g.SchemaFields[i-1].Name, g.SchemaFields[i].Name)
	}
}

// TestGenerate_readTypeDirect verifies ReadType=direct sets ReadMethod correctly.
func TestGenerate_readTypeDirect(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name: "alertmutingrule",
		Mutations: []config.MutationConfig{
			{Name: "alertsMutingRuleCreate"},
			{Name: "alertsMutingRuleDelete"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName: "alert_muting_rule",
			ReadMethod:   "GetMutingRuleWithContext",
			ReadType:     "direct",
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)
	assert.Equal(t, "direct", g.TerraformConfig.ReadType)
	assert.Equal(t, "GetMutingRuleWithContext", g.ReadMethod)
}

// TestGenerate_readTypeListFilter verifies list_filter fields are propagated.
func TestGenerate_readTypeListFilter(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name:       "ainotifications",
		ImportPath: "github.com/newrelic/newrelic-client-go/v2/pkg/ainotifications",
		Mutations: []config.MutationConfig{
			{Name: "aiNotificationsCreateDestination"},
			{Name: "aiNotificationsUpdateDestination"},
			{Name: "aiNotificationsDeleteDestination"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName:      "notification_destination",
			ClientService:     "Notifications",
			ReadMethod:        "GetDestinationsWithContextAccount",
			ReadType:          "list_filter",
			ReadFilterType:    "AiNotificationsDestinationFilter",
			ReadFilterIDField: "ID",
			ReadResultPath:    "Entities",
			RequiresAccountID: true,
			IDType:            "string",
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)
	assert.Equal(t, "list_filter", g.TerraformConfig.ReadType)
	assert.Equal(t, "Notifications", g.ClientService)
	assert.Equal(t, "AiNotificationsDestinationFilter", g.TerraformConfig.ReadFilterType)
	assert.Equal(t, "Entities", g.TerraformConfig.ReadResultPath)
	assert.Equal(t, "AiNotificationsCreateDestinationInput", g.CreateInputType)
}

// TestGenerate_readTypeEntityManagement verifies entity_management fields are propagated.
func TestGenerate_readTypeEntityManagement(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name:       "scorecards",
		ImportPath: "github.com/newrelic/newrelic-client-go/v2/pkg/scorecards",
		Mutations: []config.MutationConfig{
			{Name: "entityManagementCreateScorecard"},
			{Name: "entityManagementUpdateScorecard"},
			{Name: "entityManagementDeleteScorecard"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName:    "scorecard",
			ReadMethod:      "GetEntityWithContext",
			ReadType:        "entity_management",
			ReadEntityType:  "EntityManagementScorecardEntity",
			IDType:          "string",
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)
	assert.Equal(t, "entity_management", g.TerraformConfig.ReadType)
	assert.Equal(t, "EntityManagementScorecardEntity", g.TerraformConfig.ReadEntityType)
	assert.Equal(t, "EntityManagementCreateScorecardInput", g.CreateInputType)
}

// TestGenerate_explicitMutationNames verifies explicit mutation names override auto-detection.
func TestGenerate_explicitMutationNames(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name:       "ainotifications",
		ImportPath: "github.com/newrelic/newrelic-client-go/v2/pkg/ainotifications",
		Mutations: []config.MutationConfig{
			{Name: "aiNotificationsCreateDestination"},
			{Name: "aiNotificationsUpdateDestination"},
			{Name: "aiNotificationsDeleteDestination"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName:   "notification_destination",
			CreateMutation: "aiNotificationsCreateDestination",
			UpdateMutation: "aiNotificationsUpdateDestination",
			DeleteMutation: "aiNotificationsDeleteDestination",
			ReadMethod:     "GetDestinationsWithContextAccount",
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)
	assert.Equal(t, "AiNotificationsCreateDestinationWithContext", g.CreateMethod)
	assert.Equal(t, "AiNotificationsUpdateDestinationWithContext", g.UpdateMethod)
	assert.Equal(t, "AiNotificationsDeleteDestinationWithContext", g.DeleteMethod)
}

// TestGenerate_computedFieldOverride verifies computed_fields are applied to schema fields.
func TestGenerate_computedFieldOverride(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name: "alertmutingrule",
		Mutations: []config.MutationConfig{
			{Name: "alertsMutingRuleCreate"},
			{Name: "alertsMutingRuleDelete"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName:   "alert_muting_rule",
			ReadMethod:     "GetMutingRuleWithContext",
			ComputedFields: []string{"name"},
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)

	var nameField *lang.TerraformField
	for i := range g.SchemaFields {
		if g.SchemaFields[i].Name == "name" {
			nameField = &g.SchemaFields[i]
			break
		}
	}
	require.NotNil(t, nameField, "field 'name' not found in SchemaFields")
	assert.True(t, nameField.Computed)
}

// TestGenerate_noUpdateMutation verifies no_update_mutation causes HasUpdate=false.
func TestGenerate_noUpdateMutation(t *testing.T) {
	t.Parallel()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name: "alertmutingrule",
		Mutations: []config.MutationConfig{
			{Name: "alertsMutingRuleCreate"},
			{Name: "alertsMutingRuleUpdate"},
			{Name: "alertsMutingRuleDelete"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName:     "alert_muting_rule",
			ReadMethod:       "GetMutingRuleWithContext",
			NoUpdateMutation: true,
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)
	assert.False(t, g.HasUpdate)
}

// TestExecute_createsFiles verifies Execute writes the expected output files.
func TestExecute_createsFiles(t *testing.T) {
	t.Parallel()

	// Execute uses codegen.CodeGen which reads templates from the TemplateDir path,
	// which is relative to the CWD when tests run (package directory).
	// We override the template dir to the actual templates location.
	destDir := t.TempDir()

	s := loadTestSchema(t)
	g := &Generator{}

	pkgConfig := &config.PackageConfig{
		Name:       "alertmutingrule",
		Path:       destDir,
		ImportPath: "github.com/newrelic/newrelic-client-go/v2/pkg/alerts",
		Mutations: []config.MutationConfig{
			{Name: "alertsMutingRuleCreate"},
			{Name: "alertsMutingRuleDelete"},
		},
		Terraform: &config.TerraformConfig{
			ResourceName: "alert_muting_rule",
			ReadMethod:   "GetMutingRuleWithContext",
			ReadType:     "direct",
		},
	}

	err := g.Generate(s, minimalGenConfig(), pkgConfig)
	require.NoError(t, err)

	// Point template dir at the repo's templates/terraform directory
	// (two levels up from generators/terraform/)
	genConfig := &config.GeneratorConfig{
		Name:        "terraform",
		TemplateDir: "../../templates/terraform",
	}

	err = g.Execute(genConfig, pkgConfig)
	require.NoError(t, err)

	// Verify the expected files exist
	expectedFiles := []string{
		filepath.Join(destDir, "resource_newrelic_alert_muting_rule.go"),
		filepath.Join(destDir, "structures_newrelic_alert_muting_rule.go"),
		filepath.Join(destDir, "provider_registration.txt"),
	}
	for _, f := range expectedFiles {
		_, statErr := os.Stat(f)
		assert.NoError(t, statErr, "expected file %s to exist", f)
	}
}
