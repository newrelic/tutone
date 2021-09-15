package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		Endpoint: "https://api222.newrelic.com/graphql",
		Auth: AuthConfig{
			Header: "Api-Key",
			EnvVar: "NEW_RELIC_API_KEY",
		},
		Cache: CacheConfig{
			Enable:     false,
			SchemaFile: "testing.schema.json",
		},
		Packages: []PackageConfig{
			{
				Name:       "alerts",
				Path:       "pkg/alerts",
				ImportPath: "github.com/newrelic/newrelic-client-go/pkg/alerts",
				Types: []TypeConfig{
					{
						Name: "AlertsMutingRuleConditionInput",
					},
					{
						Name:                  "AlertsPolicy",
						GenerateStructGetters: true,
					},
					{
						Name:              "ID",
						FieldTypeOverride: "string",
						SkipTypeCreate:    true,
					},
					{
						Name: "InterfaceImplementation",
						InterfaceMethods: []string{
							"Get() string",
						},
					},
				},
				Generators: []string{"typegen"},
				Queries: []Query{
					{
						Path: []string{
							"actor",
							"cloud",
						},
						Endpoints: []EndpointConfig{
							{
								Name:               "linkedAccounts",
								MaxQueryFieldDepth: 2,
								IncludeArguments:   []string{"provider"},
								ExcludeFields:      []string{"updatedAt"},
							},
						},
					},
				},
				Mutations: []MutationConfig{
					{
						Name:               "cloudConfigureIntegration",
						MaxQueryFieldDepth: 1,
					},
					{
						Name:               "cloudLinkAccount",
						MaxQueryFieldDepth: 1,
						ArgumentTypeOverrides: map[string]string{
							"accountId": "Int!",
							"accounts":  "CloudLinkCloudAccountsInput!",
						},
						ExcludeFields: []string{"updatedAt"},
					},
				},
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
