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

	assert.Equal(t, "trace", config.LogLevel, "trace")

	assert.Equal(t, "testingtesting.schema.json", config.Cache.SchemaFile)
	assert.False(t, config.Cache.Enable)

	assert.Equal(t, "https://api-staging.newrelic.com/graphql", config.Endpoint)

	assert.Equal(t, 1, len(config.Packages))

}
