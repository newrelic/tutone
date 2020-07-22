package fetch

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/internal/util"
)

const (
	DefaultAPIKeyEnv       = "TUTONE_API_KEY"
	DefaultSchemaCacheFile = "schema.json"
)

var Command = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch GraphQL Schema",
	Long: `Fetch GraphQL Schema

Query the GraphQL server for schema and write it to a file.
`,
	Example: "tutone fetch --config configs/tutone.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		Fetch(
			viper.GetString("endpoint"),
			viper.GetString("auth.header"),
			viper.GetString("auth.api-key-env"),
			viper.GetString("cache.schema_file"),
		)
	},
}

func Fetch(
	endpoint string,
	authHeader string,
	authEnvVariableName string,
	schemaFile string,
) {
	e := NewEndpoint()
	e.URL = endpoint
	e.Auth.Header = authHeader
	e.Auth.APIKey = os.Getenv(authEnvVariableName)

	schema, err := e.Fetch()
	util.LogIfError(log.FatalLevel, err)

	if schemaFile != "" {
		util.LogIfError(log.ErrorLevel, schema.Save(schemaFile))
	}

	log.WithFields(log.Fields{
		"endpoint":    endpoint,
		"schema_file": schemaFile,
	}).Info("successfully fetched schema")
}

func init() {
	Command.Flags().StringP("endpoint", "e", "", "GraphQL Endpoint")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("endpoint", Command.Flags().Lookup("endpoint")))

	Command.Flags().String("header", DefaultAuthHeader, "Header name set for Authentication")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("auth.header", Command.Flags().Lookup("header")))

	Command.Flags().String("api-key-env", DefaultAPIKeyEnv, "Environment variable to read API key from")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("auth.api-key-env", Command.Flags().Lookup("api-key-env")))

	Command.Flags().StringP("schema", "s", DefaultSchemaCacheFile, "Output file for the schema")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("schema_file", Command.Flags().Lookup("schema")))
}
