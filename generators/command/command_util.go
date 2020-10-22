package command

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

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

// NOTE: Maybe move this as a more generic method on Schema struct
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

	// fmt.Printf("\n rootQueryType.Name:     %+v \n", rootQueryType.Name)

	log.Print("\n\n **************************** \n")

	queryPath = queryPath[1:]

	match := findField(s, queryPath, rootQueryType)

	fmt.Printf(" FOUND match??:   %+v \n", match)

	log.Print("\n\n **************************** \n")

	if match != nil {
		return match, nil
	}

	// fmt.Printf(" rootQueryType.Fields:   %+v \n", rootQueryType.Fields)

	// Start from index 1 instead of 0 because we directly
	// accessed the root query via index 0.
	// queryFieldNamesPath := queryPath[1:]

	// TODO: We need to handle this recursively!!
	// for _, q := range queryFieldNamesPath {
	// 	field, err := rootQueryType.GetField(q)
	// 	if err != nil {
	// 		continue
	// 		// return nil, err
	// 	}

	// 	fmt.Printf(" rootQueryType Field:   %+v \n", field.Name)
	// 	fmt.Printf(" field.Type.GetTypeName():   %+v \n", field.Type.GetTypeName())

	// 	fld, _ := s.LookupTypeByName(field.Type.GetTypeName())

	// 	fmt.Printf(" Next Field:   %+v \n", fld.Name)

	// 	if len(field.Args) > 0 {

	// 	}
	// }

	return nil, fmt.Errorf("could not find matching introspection data for provided query path")
}

func toJSON(data interface{}) string {
	c, _ := json.MarshalIndent(data, "", "  ")

	return string(c)
}

func findField(s *schema.Schema, queryFieldNames []string, obj *schema.Type) *schema.Field {

	fmt.Printf("\n\n findField queryFieldNames:     %+v \n", queryFieldNames)
	// fmt.Printf(" findField obj.Name:              %+v \n", toJSON(obj))

	// if len(queryFieldNames) ==  {
	// 	f, err := obj.GetField(queryFieldNames[0])
	// 	if err != nil {
	// 		// could not find what we're looking for
	// 		return nil
	// 	}
	// 	return f
	// }

	for _, q := range queryFieldNames {
		// fmt.Printf(" findField node name:            %+v \n", q)

		field, _ := obj.GetField(q)

		// fmt.Printf(" findField field.Name:           %+v \n\n", field)

		if len(queryFieldNames) == 1 && queryFieldNames[0] == q {
			return field
		}

		theField, _ := s.LookupTypeByName(field.Type.GetTypeName())

		// fmt.Printf(" findField theField:             %+v \n", toJSON(theField))

		remainingFields := queryFieldNames[1:]

		// fmt.Printf(" findField remainingFields:      %+v \n", remainingFields)

		found := findField(s, remainingFields, theField)

		// fmt.Printf(" findField found:                %+v \n", toJSON(found))

		// time.Sleep(10 * time.Second)

		if found != nil && len(remainingFields) == 1 {
			return found
		}

		// queryFieldNames = queryFieldNames[1:]

		// nextField, _ := s.LookupTypeByName(field.Type.GetTypeName())
		// // if err != nil {
		// // 	return field
		// // }

		// fmt.Printf(" findField nextField.name:         %+v \n", nextField.Name)
		// // fmt.Printf(" findField nextField.Args:         %+v \n", nextField)

		// result := findField(s, queryFieldNames, nextField)

		// fmt.Printf(" findField result:         %+v \n", result)

		// if result == nil {
		// 	return field
		// }
	}

	return nil
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
		// var queryPathTypes []*schema.Type
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
			// TODO: This works for now, but schema.LookupQueryTypesByFieldPath is optimized
			// for generating types in the client. In the case of the CLI we only need the last
			// part of the path (the endpoint). A DFS lookup could be quicker, although the
			// performance hit might be negligible.
			// queryPathTypes, err = s.LookupQueryTypesByFieldPath(subCmdConfig.GraphQLPath)
			// if err != nil {
			// 	log.Fatalf("query endpoint not found: %s", err)
			// }

			// graphQLParentScope := subCmdConfig.GraphQLPath[len(subCmdConfig.GraphQLPath)-2]
			// graphQLEndpoint := subCmdConfig.GraphQLPath[len(subCmdConfig.GraphQLPath)-1]

			subcommandMetadata, err = getReadCommandMetadata(s, subCmdConfig.GraphQLPath)
			if err != nil {
				log.Fatal(err)
			}

			// for _, queryStep := range queryPathTypes {

			// 	s.LookupRootQueryTypeFieldByName("actor")

			// 	fmt.Printf("\n queryStep.Name:           %+v \n", queryStep.Name)
			// 	fmt.Printf(" queryStep.Kind:           %+v \n", queryStep.Kind)
			// 	// fmt.Printf("\n queryStep.Args:           %+v \n", queryStep)
			// 	// fmt.Printf(" queryStep.InputFields:      %+v \n", queryStep.InputFields)
			// 	fmt.Printf(" queryStep.PossibleTypes:    %+v \n", queryStep.PossibleTypes)

			// 	for _, field := range queryStep.Fields {
			// 		if field.Name == "key" {
			// 			fmt.Printf(" queryStep.field.Name:       %+v \n", field.Name)
			// 			fmt.Printf(" queryStep IsRequired:       %+v \n", field.IsRequired())
			// 			// fmt.Printf("\n graphQLEndpoint:  %+v \n", graphQLEndpoint)
			// 		}

			// 		if field.Name == graphQLEndpoint {
			// 			subcommandMetadata = &field
			// 		}
			// 	}
			// }

			// fmt.Printf("\n subcommandMetadata:         %+v \n", *subcommandMetadata)

			// fmt.Print("\n **************************** \n\n")
			// time.Sleep(3 * time.Second)
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
	log.Print("\n\n **************************** \n")
	log.Printf("\n hydrateQuerySubcommand - Args:  %+v \n", sCmd.Args)
	log.Print("\n **************************** \n\n")
	time.Sleep(3 * time.Second)

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
