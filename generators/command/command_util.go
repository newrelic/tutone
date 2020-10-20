package command

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/huandu/xstrings"
	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
	log "github.com/sirupsen/logrus"
)

var goTypesToCobraFlagMethodMap = map[string]string{
	"int":    "IntVar",
	"string": "StringVar",
}

func hydrateCommand(s *schema.Schema, command config.Command, pkgConfig *config.PackageConfig) lang.Command {
	isBaseCommand := true
	cmdVarName := "Command"

	// Handle case where this command is a subcommand of a parent entry point
	// Note: this doesn't do anything yet,
	if !isBaseCommand {
		cmdVarName = fmt.Sprintf("cmd%s", xstrings.ToCamelCase(command.Name))
	}

	cmd := lang.Command{
		Name:             command.Name,
		CmdVariableName:  cmdVarName,
		ShortDescription: command.ShortDescription,
		LongDescription:  command.LongDescription,
		Example:          command.Example,
		GraphQLPath:      command.GraphQLPath,
	}

	if len(command.Subcommands) == 0 {
		return lang.Command{}
	}

	cmd.Subcommands = make([]lang.Command, len(command.Subcommands))

	for i, subCmdConfig := range command.Subcommands {
		var err error
		// var mutationCmdData *schema.Field
		// var queryCmdData *schema.Field
		var queryPathTypes []*schema.Type
		var subcommandMetadata *schema.Field

		// Check to see if the commands CRUD action is a mutation.
		// If it's not a mutation, then it's a query (read) request.
		subcommandMetadata, err = s.LookupMutationByName(subCmdConfig.Name)

		// If the command is not a mutation, move forward with
		// generating a command to perform a query request.
		if err != nil {
			// TODO: This works for now, but schema.LookupQueryTypesByFieldPath is optimized
			// for generating types in the client. In the case of the CLI we only need the last
			// part of the path (the endpoint). A DFS lookup could be quicker, although the
			// performance hit might be negligible.
			queryPathTypes, err = s.LookupQueryTypesByFieldPath(subCmdConfig.GraphQLPath)
			if err != nil {
				log.Fatalf("query endpoint not found: %s", err)
			}

			graphQLEndpoint := subCmdConfig.GraphQLPath[len(subCmdConfig.GraphQLPath)-1]

			for _, queryStep := range queryPathTypes {
				for _, field := range queryStep.Fields {
					if field.Name == graphQLEndpoint {
						subcommandMetadata = &field
					}
				}
			}
		}

		subcommand := hydrateSubcommand(s, subcommandMetadata, subCmdConfig)

		exampleData := lang.CommandExampleData{
			CLIName:     "newrelic",
			PackageName: pkgConfig.Name,
			Command:     cmd.Name,
			Subcommand:  subcommand.Name,
			Flags:       subcommand.Flags,
		}

		subcommand.Example = subCmdConfig.Example
		if subCmdConfig.Example == "" {
			sCmdExample, err := generateCommandExample(subcommandMetadata, exampleData)
			if err != nil {
				log.Fatal(err)
			}

			subcommand.Example = sCmdExample
		}

		cmd.Subcommands[i] = *subcommand
	}

	return cmd
}

// TODO: Consolidate shared parts of
// hydrateCommand, hydrateSubcommand, hydrateQuerySubcommand
func hydrateSubcommand(s *schema.Schema, sCmd *schema.Field, cmdConfig config.Command) *lang.Command {
	flags := hydrateFlagsFromSchema(sCmd.Args, cmdConfig)

	var clientMethodArgs []string
	for _, f := range flags {
		varName := f.VariableName
		// If the client method argument is an `INPUT_OBJECT`,
		// we need the regular name to unmarshal.
		if f.IsInputType {
			varName = f.Name
		}
		clientMethodArgs = append(clientMethodArgs, varName)
	}

	shortDescription := sCmd.Description
	// Allow configuration to override the description that comes from NerdGraph
	if cmdConfig.ShortDescription != "" {
		shortDescription = cmdConfig.ShortDescription
	}

	cmdResult := lang.Command{
		Name:             sCmd.Name,
		CmdVariableName:  fmt.Sprintf("cmd%s", xstrings.FirstRuneToUpper(sCmd.Name)),
		ShortDescription: shortDescription,
		LongDescription:  cmdConfig.LongDescription,
		ClientMethod:     cmdConfig.ClientMethod,
		ClientMethodArgs: clientMethodArgs,
		Example:          cmdConfig.Example,
		Flags:            flags,
	}

	return &cmdResult
}

func hydrateQuerySubcommand(s *schema.Schema, sCmd *schema.Field, cmdConfig config.Command) lang.Command {
	flags := hydrateFlagsFromSchema(sCmd.Args, cmdConfig)

	var clientMethodArgs []string
	for _, f := range flags {
		varName := f.VariableName
		// If the client method argument is an `INPUT_OBJECT`,
		// we need the regular name to unmarshal.
		if f.IsInputType {
			varName = f.Name
		}
		clientMethodArgs = append(clientMethodArgs, varName)
	}

	shortDescription := sCmd.Description
	// Allow configuration to override the description that comes from NerdGraph
	if cmdConfig.ShortDescription != "" {
		shortDescription = cmdConfig.ShortDescription
	}

	cmdResult := lang.Command{
		Name:             sCmd.Name,
		ShortDescription: shortDescription,
		LongDescription:  cmdConfig.LongDescription,
		ClientMethod:     cmdConfig.ClientMethod,
		ClientMethodArgs: clientMethodArgs,
		Example:          cmdConfig.Example,
		Flags:            flags,
	}

	return cmdResult
}

func generateCommandExample(sCmd *schema.Field, data lang.CommandExampleData) (string, error) {
	t := `{{ .CLIName }} {{ .Command }} {{ .Subcommand }}{{- range .Flags }} --{{ .Name }}{{ end }}`

	return codegen.RenderTemplate("commandExample", t, data)
}

func hydrateFlagsFromSchema(args []schema.Field, cmdConfig config.Command) []lang.CommandFlag {
	var flags []lang.CommandFlag

	for _, arg := range args {
		var variableName string

		isInputObject := arg.Type.IsInputObject()
		if isInputObject {
			// Add 'Input' suffix to the input variable name
			variableName = fmt.Sprintf("%sInput", cmdConfig.Name)
		} else {
			variableName = fmt.Sprintf("%s%s", cmdConfig.Name, arg.Name)
		}

		typ, _, _ := arg.Type.GetType()

		if arg.IsScalarID() {
			typ = "string"
		}

		variableType := "string"
		if arg.IsGoType() {
			variableType = typ
		}

		isRequired := arg.Type.Kind == schema.KindNonNull
		clientType := fmt.Sprintf("%s.%s", cmdConfig.ClientPackageName, typ)

		flag := lang.CommandFlag{
			Name:           arg.Name,
			Type:           typ,
			FlagMethodName: getCobraFlagMethodName(typ),
			DefaultValue:   "",
			Description:    arg.Description,
			VariableName:   variableName,
			VariableType:   variableType,
			Required:       isRequired,
			IsInputType:    isInputObject,
			ClientType:     clientType,
		}

		flags = append(flags, flag)
	}

	return flags
}

func fetchRemoteTemplate(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var respString string
	if resp.StatusCode == http.StatusOK {
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		respString = string(respBytes)
	}

	return respString, nil
}

func getCobraFlagMethodName(typeString string) string {
	if v, ok := goTypesToCobraFlagMethodMap[typeString]; ok {
		return v
	}

	// Almost all CRUD inputs will be a JSON string
	return "StringVar"
}
