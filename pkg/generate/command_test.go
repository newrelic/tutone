package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file are not parallel: they change the generate command's flags and the
// process-wide viper settings.

// setSchemaFlag passes --schema as a command line would, and binds it as the command does before it runs.
func setSchemaFlag(t *testing.T, value string) {
	t.Helper()

	flag := Command.Flags().Lookup("schema")
	t.Cleanup(func() {
		flag.Changed = false
		require.NoError(t, flag.Value.Set(flag.DefValue))
	})

	require.NoError(t, Command.Flags().Set("schema", value))
	require.NoError(t, Command.PreRunE(Command, nil))
}

func TestCommand_SchemaFlag(t *testing.T) {
	setSchemaFlag(t, "other-schema.json")

	assert.Equal(t, "other-schema.json", viper.GetString("cache.schema_file"))
}

func TestCommand_ReturnsGenerateError(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".tutone.yml")
	require.NoError(t, os.WriteFile(configFile, []byte("packages:\n  - name: alpha\n    bogus_option: true\n"), 0o600))
	// The schema file must exist, or Generate fetches one.
	schemaFile := filepath.Join(dir, "schema.json")
	require.NoError(t, os.WriteFile(schemaFile, []byte("{}"), 0o600))

	setSchemaFlag(t, schemaFile)
	viper.SetConfigFile(configFile)

	err := Command.RunE(Command, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus_option")
}
