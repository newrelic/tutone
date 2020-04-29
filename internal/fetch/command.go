package fetch

import (
	"encoding/json"
	"io/ioutil"
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
		e := NewEndpoint()
		e.URL = viper.GetString("endpoint")
		e.Auth.Header = viper.GetString("auth.header")
		e.Auth.APIKey = os.Getenv(viper.GetString("auth.api-key-env"))

		schema, err := e.Fetch()
		util.LogIfError(log.FatalLevel, err)

		if viper.GetBool("caching.enabled") {
			file := viper.GetString("caching.schema_file")
			log.WithFields(log.Fields{
				"schema_file": file,
			}).Debug("caching enabled")
			if file != "" {
				// Write out the schema we got
				schemaFile, _ := json.MarshalIndent(schema, "", " ")
				_ = ioutil.WriteFile(file, schemaFile, 0644)
			}
		}

		log.Info("success")
	},
}

func init() {
	Command.Flags().StringP("endpoint", "e", "", "GraphQL Endpoint")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("endpoint", Command.Flags().Lookup("endpoint")))

	Command.Flags().String("header", DefaultAuthHeader, "Header name set for Authentication")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("auth.header", Command.Flags().Lookup("header")))

	Command.Flags().String("api-key-env", DefaultAPIKeyEnv, "Environment variable to read API key from")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("auth.api-key-env", Command.Flags().Lookup("api-key-env")))

	Command.Flags().Bool("cache", false, "Enable caching of the schema")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("caching.enabled", Command.Flags().Lookup("cache")))

	Command.Flags().String("output", DefaultSchemaCacheFile, "Output file for the schema")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("caching.schema_file", Command.Flags().Lookup("output")))
}
