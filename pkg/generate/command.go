package generate

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/internal/util"
	"github.com/newrelic/tutone/pkg/fetch"
)

const (
	DefaultGenerateOutputFile = "types.go"
)

var (
	packageName            string
	refetch                bool
	includeIntegrationTest bool
)

var Command = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from GraphQL Schema",
	Long: `Generate code from GraphQL Schema

The generate command will generate code based on the
configured types in your .tutone.yml configuration file.
Use the --refetch flag when new types have been added to
your upstream GraphQL schema to ensure your generated code
is up to date with your configured GraphQL API.
`,
	Example: "tutone generate --config .tutone.yml",
	Run: func(cmd *cobra.Command, args []string) {
		util.LogIfError(log.ErrorLevel, Generate(GeneratorOptions{
			PackageName:            packageName,
			Refetch:                refetch,
			IncludeIntegrationTest: includeIntegrationTest,
		}))
	},
}

func init() {
	Command.Flags().StringVarP(&packageName, "package", "p", "", "Go package name for the generated files")

	Command.Flags().StringP("schema", "s", fetch.DefaultSchemaCacheFile, "Schema file to read from")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("schema_file", Command.Flags().Lookup("schema")))

	Command.Flags().String("types", DefaultGenerateOutputFile, "Output file for generated types")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("generate.type_file", Command.Flags().Lookup("types")))

	Command.Flags().BoolVar(&refetch, "refetch", false, "Force a refetch of your GraphQL schema to ensure the generated types are up to date.")
	Command.Flags().BoolVar(&includeIntegrationTest, "include-integration-test", false, "Generate a basic scaffolded integration test file for the associated package.")
}
