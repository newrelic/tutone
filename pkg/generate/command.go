package generate

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/internal/util"
	"github.com/newrelic/tutone/pkg/fetch"
)

const (
	DefaultGenerateOutputFile     = "types.go"
	DefaultGenerateDefinitionFile = ".generate.yml"
)

var packageName string

var Command = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from GraphQL Schema",
	Long: `Generate code from GraphQL Schema

Using an existing schema file, load / parse / generate code to implement it.

To use with go generate, add the following to a package file:
//go:generate tutone generate -p $GOPACKAGE
`,
	Example: "tutone generate --package $GOPACKAGE",
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

	Command.Flags().StringP("definition", "d", DefaultGenerateDefinitionFile, "Package definition of what to generate")
	viper.SetDefault("definition", DefaultGenerateDefinitionFile)
}
