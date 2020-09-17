package command

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/Masterminds/sprig"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/internal/util"
	"github.com/newrelic/tutone/pkg/lang"
)

type Generator struct {
	lang.CommandGenerator
}

var goTypesToCobraFlagMethodMap = map[string]string{
	"int":    "IntVar",
	"string": "StringVar",
}

func hydrateCommand(s *schema.Schema, command config.Command) lang.Command {
	cmd := lang.Command{
		Name:             command.Name,
		ShortDescription: command.ShortDescription,
		LongDescription:  command.LongDescription,
		Example:          command.Example,
	}

	if len(command.Subcommands) > 0 {
		cmd.Subcommands = make([]lang.Command, len(command.Subcommands))

		for i, subCmdConfig := range command.Subcommands {
			cmdType, err := s.LookupRootMutationTypeFieldByName(subCmdConfig.Name)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Print("\n\n **************************** \n")
			// fmt.Printf("\n cmdType:  %+v \n", cmdType)

			// TODO: set a "mutation" or "query" type on the subcommand tutone config
			// or maybe just a boolean "isMutation" or something like that
			_ = hydrateMutationSubcommand(s, cmdType, subCmdConfig)

			fmt.Print("\n **************************** \n\n")

			// Old news, bye bye
			cmd.Subcommands[i] = lang.Command{
				Name:             subCmdConfig.Name,
				ShortDescription: subCmdConfig.ShortDescription,
				LongDescription:  subCmdConfig.LongDescription,
				Example:          subCmdConfig.Example,
				InputType:        subCmdConfig.InputType,
				ClientMethod:     subCmdConfig.ClientMethod,
				Flags:            hydrateFlags(subCmdConfig.Flags),
			}
		}
	}

	return cmd
}

type Arg struct {
	Name string
	Type string
}

func hydrateMutationSubcommand(s *schema.Schema, sCmd *schema.Field, cmdConfig config.Command) *lang.Command {
	fmt.Printf("\n hydrateSubcommand - schema:     %+v \n", sCmd)

	tmpl, err := template.New("test").Funcs(sprig.TxtFuncMap()).Parse(`
		{{- .Method -}}({{- range .Args }}{{ .Name }} {{ end -}})
	`)

	if err != nil {
		log.Fatal(err)
	}

	data := struct {
		Method string
		Args   []Arg
	}{
		Method: cmdConfig.ClientMethod,
		Args: []Arg{
			{
				Name: "accountId",
				Type: "int",
			},
			{
				Name: "policy",
				Type: "AlertsPolicyInput",
			},
		},
	}

	var resultBuf bytes.Buffer
	err = tmpl.Execute(&resultBuf, data)

	fmt.Printf("\n\n Method Signature: %v \n\n", resultBuf.String())

	if err != nil {
		log.Fatal(err)
	}

	// inputTypes := map[string]interface{}{}
	for _, inputType := range sCmd.Args {

		fmt.Printf("\n hydrateSubcommand - inputType:  %+v \n", inputType)

		// inputTypes = append(inputTypes, map[string]interface{} {
		// 	ClientMethod: fmt.Sprintf("%v(%v %v)", cmdConfig.ClientMethod,
		// })
	}

	cmdResult := lang.Command{
		Name:             sCmd.Name,
		ShortDescription: sCmd.Description, // TODO: allow user to override this in their tutone.yml
		LongDescription:  cmdConfig.LongDescription,
	}

	return &cmdResult
}

func hydrateFlags(flags []config.CommandFlag) []lang.CommandFlag {
	cmdFlags := make([]lang.CommandFlag, len(flags))

	for i, f := range flags {
		cmdFlags[i] = lang.CommandFlag{
			Name:           f.Name,
			Type:           f.Type,
			FlagMethodName: goTypesToCobraFlagMethodMap[f.Type],
			DefaultValue:   f.DefaultValue,
			Description:    f.Description,
			VariableName:   f.VariableName,
			Required:       f.Required,
		}
	}

	return cmdFlags
}

func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	log.Debugf("Generate...")

	g.PackageName = pkgConfig.Name
	g.Imports = pkgConfig.Imports

	cmds := make([]lang.Command, len(pkgConfig.Commands))
	for i, c := range pkgConfig.Commands {
		cmds[i] = hydrateCommand(s, c)
	}

	g.Commands = cmds

	return nil
}

func (g *Generator) Execute(genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	log.Debugf("Execute...")

	var err error

	destinationPath := GetDestinationPath(pkgConfig)
	if err = MakeDir(destinationPath); err != nil {
		return err
	}

	for _, command := range pkgConfig.Commands {
		fileName := fmt.Sprintf("command_%s.go", util.ToSnakeCase(command.Name))

		// Default template name is '{{ packageName }}.go.tmpl'
		templateName := "command.go.tmpl"
		if genConfig.TemplateName != "" {
			templateName = genConfig.TemplateName
		}

		fPath := fmt.Sprintf("%s/%s", destinationPath, fileName)
		destinationFile, err := codegen.RenderStringFromGenerator(fPath, g)
		if err != nil {
			return err
		}

		templateDir := "templates/command"
		if genConfig.TemplateDir != "" {
			templateDir, err = codegen.RenderStringFromGenerator(genConfig.TemplateDir, g)
			if err != nil {
				return err
			}
		}

		c := codegen.CodeGen{
			TemplateDir:     templateDir,
			TemplateName:    templateName,
			DestinationFile: destinationFile,
			DestinationDir:  destinationPath,
		}

		if err := c.WriteFile(g); err != nil {
			return err
		}

		printSuccessMessage(pkgConfig, destinationFile)
	}

	return nil
}

/////
// HELPER FUNCTIONS - TODO: Move to proper home
/////
func GetDestinationPath(pkgConfig *config.PackageConfig) string {
	if pkgConfig.Path != "" {
		return pkgConfig.Path
	}

	return "./"
}

// MakeDir creates a directory if it does't exist yet and sets
// folder permissions to 0755.
func MakeDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.Mkdir(path, 0755); err != nil {
			return err
		}
	}

	return nil
}

// printSuccessMessage prints a message to the terminal letting the user know
// that code generation was a success and outputs the package and file path for reference.
//
// Emoji unicode reference: http://www.unicode.org/emoji/charts/emoji-list.html
func printSuccessMessage(pkgConfig *config.PackageConfig, filePath string) {
	fmt.Print("\n\u2705 Code generation complete: \n\n")
	fmt.Printf("   Package:   %v \n", pkgConfig.Path)
	fmt.Printf("   File:      %v \n", filePath)
	fmt.Println("")
}
