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

var packageName string

var Command = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from GraphQL Schema",
	Long: `Generate code from GraphQL Schema

TODO: Write something intelligent here.
`,
	Example: "tutone generate --config .tutone.yml",
	Run: func(cmd *cobra.Command, args []string) {
		util.LogIfError(log.ErrorLevel, Generate())
	},
}

func init() {
	Command.Flags().StringVarP(&packageName, "package", "p", "", "Go package name for the generated files")

	Command.Flags().StringP("schema", "s", fetch.DefaultSchemaCacheFile, "Schema file to read from")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("schema_file", Command.Flags().Lookup("schema")))

	Command.Flags().String("types", DefaultGenerateOutputFile, "Output file for generated types")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("generate.type_file", Command.Flags().Lookup("types")))

}
