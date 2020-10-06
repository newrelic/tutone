package command

import (
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
)

func hydrateSubcommand(
	s *schema.Schema,
	sCmd *schema.Field,
	cmdConfig config.Command,
) *lang.Command {
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
