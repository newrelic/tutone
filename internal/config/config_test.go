package config

import (
	"testing"

	"github.com/tj/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	config, err := LoadConfig("doesnotexist")
	assert.Error(t, err)
	assert.Nil(t, config)

	config, err = LoadConfig("../../testdata/goodConfig_fixture.yml")
	assert.NoError(t, err)
	assert.NotNil(t, config)

	expected := &Config{
		LogLevel: "trace",
		Endpoint: "https://api-staging.newrelic.com/graphql",
		Auth: AuthConfig{
			Header: "Api-Key",
			EnvVar: "NEW_RELIC_API_KEY",
		},
		Cache: CacheConfig{
			Enable:     false,
			SchemaFile: "testingtesting.schema.json",
		},
		Packages: []PackageConfig{
			{
				Name: "alerts",
				Path: "pkg/alerts",
				Types: []TypeConfig{
					{
						Name: "AlertsMutingRuleConditionInput",
					}, {
						Name:              "ID",
						FieldTypeOverride: "string",
						SkipTypeCreate:    true,
					},
				},
				Generators: []string{"typegen"},
			},
		},
		Generators: []GeneratorConfig{
			{
				Name: "typegen",
				// DestinationFile:
				// TemplateDir:
				FileName: "types.go",
				// TemplateName:
			},
		},
	}

	assert.Equal(t, config, expected)

}
