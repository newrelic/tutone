package lang

// TODO: Move CommandGenerator and its friends to a proper home
type CommandGenerator struct {
	PackageName string
	Imports     []string
	Commands    []Command
}

type InputObject struct {
	Name   string
	GoType string
}

type Command struct {
	Name             string
	CmdVariableName  string
	ShortDescription string
	LongDescription  string
	Example          string
	InputType        string
	ClientMethod     string
	ClientMethodArgs []string
	InputObjects     []InputObject
	Flags            []CommandFlag
	Subcommands      []Command

	GraphQLPath []string // Should mutations also use this? Probably
}

type CommandFlag struct {
	Name           string
	Type           string
	FlagMethodName string
	DefaultValue   string
	Description    string
	VariableName   string
	VariableType   string
	ClientType     string
	Required       bool
	IsInputType    bool
	IsEnumType     bool
}

type CommandExampleData struct {
	CLIName     string
	PackageName string
	Command     string
	Subcommand  string
	Flags       []CommandFlag
}
