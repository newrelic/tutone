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

// getReadCommandMetadata returns the associated types to generate a "read" command
func getReadCommandMetadata(s *schema.Schema, queryPath []string) (*schema.Field, error) {
	if len(queryPath) == 0 {
		return nil, fmt.Errorf("query path is empty")
	}

	rootQuery, err := s.LookupRootQueryTypeFieldByName(queryPath[0])
	if err != nil {
		return nil, fmt.Errorf("root query field not found: %s", err)
	}

	rootQueryType, err := s.LookupTypeByName(rootQuery.GetName())
	if err != nil {
		return nil, fmt.Errorf("%s", err) // TODO: Do better
	}

	// Remove the root query field from the slice since
	// we extracted its type above.
	queryPath = queryPath[1:]

	found := s.RecursiveLookupFieldByPath(queryPath, rootQueryType)
	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf("could not find matching introspection data for provided query path")
}

func hydrateCommand(s *schema.Schema, command config.Command, pkgConfig *config.PackageConfig) lang.Command {
	isBaseCommand := true
	cmdVarName := "Command"

	// Handle case where this command is a subcommand of a parent entry point
	// Note: this doesn't do anything yet
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
		var subcommandMetadata *schema.Field

		// Check to see if the commands CRUD action is a mutation.
		// If it's not a mutation, then it's a query (read) request.
		subcommandMetadata, err = s.LookupMutationByName(subCmdConfig.Name)

		if subcommandMetadata == nil {
			log.Debugf("no mutation reference found, assuming query request type")
		}

		// If the command is not a mutation, move forward with
		// generating a command to perform a query request.
		if err != nil {
			subcommandMetadata, err = getReadCommandMetadata(s, subCmdConfig.GraphQLPath)
			if err != nil {
				log.Fatal(err)
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

// TODO: Consolidate common parts of hydrateCommand, hydrateSubcommand
func hydrateSubcommand(s *schema.Schema, sCmd *schema.Field, cmdConfig config.Command) *lang.Command {
	flags := hydrateFlagsFromSchema(sCmd.Args, cmdConfig)

	var err error
	var clientMethodArgs []string
	for _, f := range flags {
		varName := f.VariableName
		// If the client method argument is an `INPUT_OBJECT`,
		// we need the regular name to unmarshal.
		if f.IsInputType {
			varName = f.Name
		}

		if f.IsEnumType {
			varName, err = wrapEnumTypeVariable(varName, f.ClientType)
			if err != nil {
				log.Fatal(err)
			}
		}

		clientMethodArgs = append(clientMethodArgs, varName)
	}

	shortDescription := sCmd.Description
	// Allow configuration to override the description that comes from NerdGraph
	if cmdConfig.ShortDescription != "" {
		shortDescription = cmdConfig.ShortDescription
	}

	cmdName := sCmd.Name
	if cmdConfig.Name != "" {
		cmdName = cmdConfig.Name
	}

	cmdResult := lang.Command{
		Name:             cmdName,
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

// Returns a string representation of a variable wrapped/typed with an enum type ref
//
// e.g apiaccess.APIAccessKeyType("KEY_TYPE")
func wrapEnumTypeVariable(varName string, clientTypeRefString string) (string, error) {
	data := struct {
		VarName string
		TypeRef string
	}{
		VarName: varName,
		TypeRef: clientTypeRefString,
	}

	// TODO: Consider passing in `pkgName` instead of a previously constructed
	// string so functionality is bit more usable/portable.
	// i.e. TypeRef in this case will look like this: `apiaccess.SomeType`
	//      But we shouldn't make the dependent function create that string
	//      and then pass it to this function.
	t := `{{ .TypeRef }}({{ .VarName }})`

	return codegen.RenderTemplate(varName, t, data)
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
			// TODO: Use helper mthod arg.GetName() to format this properly?
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

		clientType := fmt.Sprintf("%s.%s", cmdConfig.ClientPackageName, typ)

		flag := lang.CommandFlag{
			Name:           arg.Name,
			Type:           typ,
			FlagMethodName: getCobraFlagMethodName(typ),
			DefaultValue:   "",
			Description:    arg.Description,
			VariableName:   variableName,
			VariableType:   variableType,
			Required:       arg.IsRequired(),
			IsInputType:    isInputObject,
			ClientType:     clientType,
			IsEnumType:     arg.IsEnum(),
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

func generateCommandExample(sCmd *schema.Field, data lang.CommandExampleData) (string, error) {
	t := `{{ .CLIName }} {{ .Command }} {{ .Subcommand }}{{- range .Flags }} --{{ .Name }}{{ end }}`

	return codegen.RenderTemplate("commandExample", t, data)
}
