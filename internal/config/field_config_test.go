package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_FieldOptionsAndExcludedArguments(t *testing.T) {
	t.Parallel()

	yml := `
packages:
  - name: logconfigurations
    types:
      - name: LogConfigurationsUpdateDataPartitionRuleInput
        fields:
          - name: enabled
            pointer: true
            omitempty: false
            go_type: "*bool"
            go_name: Enabled
    mutations:
      - name: entityManagementDelete
        exclude_arguments: [version]
`

	path := filepath.Join(t.TempDir(), "tutone.yml")
	require.NoError(t, os.WriteFile(path, []byte(yml), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.Packages, 1)

	tc := cfg.Packages[0].GetTypeConfigByName("LogConfigurationsUpdateDataPartitionRuleInput")
	require.NotNil(t, tc)

	fc := tc.GetFieldConfig("enabled")
	require.NotNil(t, fc)
	require.NotNil(t, fc.Pointer)
	assert.True(t, *fc.Pointer)
	require.NotNil(t, fc.OmitEmpty)
	assert.False(t, *fc.OmitEmpty)
	assert.Equal(t, "*bool", fc.GoType)
	assert.Equal(t, "Enabled", fc.GoName)

	assert.Nil(t, tc.GetFieldConfig("missing"))
	assert.Equal(t, []string{"version"}, cfg.Packages[0].Mutations[0].ExcludeArguments)
}

func TestTypeConfig_GetFieldConfig_NilReceiver(t *testing.T) {
	t.Parallel()

	var tc *TypeConfig
	assert.Nil(t, tc.GetFieldConfig("x"))
}
