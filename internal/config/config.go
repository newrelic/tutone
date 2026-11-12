package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Config is the information keeper for generating go structs from type names.
type Config struct {
	// LogLevel sets the logging level
	LogLevel string `yaml:"log_level,omitempty"`
	// Endpoint is the URL for the GraphQL API
	Endpoint string `yaml:"endpoint"`
	// Auth contains details about how to authenticate to the API in the case that it's required.
	Auth AuthConfig `yaml:"auth"`
	// Cache contains information on how and where to store the schema.
	Cache CacheConfig `yaml:"cache"`
	// Packages contain the information on how to break up the schema into code packages.
	Packages []PackageConfig `yaml:"packages,omitempty"`
	// Generators configure the work engine of this project.
	Generators []GeneratorConfig `yaml:"generators,omitempty"`
}

// AuthConfig is the information necessary to authenticate to the NerdGraph API.
type AuthConfig struct {
	// Header is the name of the API request header that is used to authenticate.
	Header string `yaml:"header,omitempty"`
	// EnvVar is the name of the environment variable to attach to the above header.
	EnvVar string `yaml:"api_key_env_var,omitempty"`
}

// CacheConfig is the information necessary to store the NerdGraph schema in JSON.
type CacheConfig struct {
	// Enable or disable the schema caching.
	Enable bool `yaml:",omitempty"`
	// SchemaFile is the location where the schema should be cached.
	SchemaFile string `yaml:"schema_file,omitempty"`
}

// PackageConfig is the information about a single package, which types to include from the schema, and which generators to use for this package.
type PackageConfig struct {
	// Name is the string that is used to refer to the name of the package.
	Name string `yaml:"name,omitempty"`
	// Path is the relative path within the project.
	Path string `yaml:"path,omitempty"`
	// ImportPath is the full path used for importing this package into a Go project
	ImportPath string `yaml:"import_path,omitempty"`
	// Types is a list of Type configurations to include in the package.
	Types []TypeConfig `yaml:"types,omitempty"`
	// Mutations is a list of Method configurations to include in the package.
	Mutations []MutationConfig `yaml:"mutations,omitempty"`
	// Generators is a list of names that reference a generator in the Config struct.
	Generators []string `yaml:"generators,omitempty"`
	// Imports is a list of strings to represent what pacakges to import for a given package.
	Imports []string `yaml:"imports,omitempty"`

	Commands []Command `yaml:"commands,omitempty"`

	Queries []Query `yaml:"queries,omitempty"`

	// FileName is the stem available to generator file name templates as
	// {{.FileName}}. It defaults to Name. Set it when the file on disk is spelled
	// differently from the package, e.g. Name "agentApplications" with FileName
	// "agentapplications", so Linux and macOS agree on the file.
	FileName string `yaml:"file_name,omitempty"`
	// GoPackageName is the identifier on the generated `package` line. It defaults
	// to Name lowercased. Set it when the Go package differs from the config name,
	// e.g. Name "scorecards" written to a folder holding package "servicearchintelligence".
	GoPackageName string `yaml:"go_package_name,omitempty"`
	// ClientTypeName is the receiver type of the generated client methods. It
	// defaults to Name with its first letter upper-cased. Set it when the client
	// type is not the package name, e.g. "OrganizationManagement" for "organization".
	ClientTypeName string `yaml:"client_type_name,omitempty"`
	// PreserveHandWritten controls whether regenerating a file keeps the top-level
	// declarations the generator does not produce, and those marked DO NOT DELETE
	// or DO NOT REGENERATE. Defaults to true.
	PreserveHandWritten *bool `yaml:"preserve_hand_written,omitempty"`
	// KeepRemovedFields keeps, when regenerating the types file, the struct fields
	// and enum values the existing declaration has and the schema no longer lists,
	// each marked with a note that the schema does not list it, so code that uses
	// them keeps compiling. It adds nothing the existing file lacks. It needs
	// PreserveHandWritten. Defaults to false.
	KeepRemovedFields bool `yaml:"keep_removed_fields,omitempty"`

	// Transient property which is set by using the --include-integration-test flag.
	IncludeIntegrationTest bool
}

// GetFileName returns the stem for generated file names.
func (c *PackageConfig) GetFileName() string {
	if c.FileName != "" {
		return c.FileName
	}

	return c.Name
}

// GetGoPackageName returns the identifier for the generated `package` line.
func (c *PackageConfig) GetGoPackageName() string {
	if c.GoPackageName != "" {
		return c.GoPackageName
	}

	return strings.ToLower(c.Name)
}

// GetClientTypeName returns the receiver type for generated client methods.
func (c *PackageConfig) GetClientTypeName() string {
	if c.ClientTypeName != "" {
		return c.ClientTypeName
	}

	return c.DefaultClientTypeName()
}

// DefaultClientTypeName is the receiver type a template that predates client_type_name
// writes: its `title` of the package name.
func (c *PackageConfig) DefaultClientTypeName() string {
	//nolint:staticcheck // matches the template's sprig `title`, which keeps existing output unchanged
	return strings.Title(c.Name)
}

// GeneratesClient reports whether the package has the nerdgraphclient generator, which
// makes the methods that call the API.
func (c *PackageConfig) GeneratesClient() bool {
	for _, g := range c.Generators {
		if g == "nerdgraphclient" {
			return true
		}
	}

	return false
}

// GetPreserveHandWritten reports whether hand-written declarations are kept, defaulting to true.
func (c *PackageConfig) GetPreserveHandWritten() bool {
	return c.PreserveHandWritten == nil || *c.PreserveHandWritten
}

// Query is the information necessary to build a query method.  The Paths
// reference the the place in the hierarchy, while the names reference the
// objects within those paths to query.
type Query struct {
	// Path is the path of TypeNames in GraphQL that precede the objects being queried.
	Path []string `yaml:"path,omitempty"`
	// Names is a list of TypeName entries that will be found at the above Path.
	Endpoints []EndpointConfig `yaml:"endpoints,omitempty"`
}

type Command struct {
	Name              string        `yaml:"name,omitempty"`
	FileName          string        `yaml:"fileName,omitempty"`
	ShortDescription  string        `yaml:"shortDescription,omitempty"`
	LongDescription   string        `yaml:"longDescription,omitempty"`
	Example           string        `yaml:"example,omitempty"`
	InputType         string        `yaml:"inputType,omitempty"`
	ClientPackageName string        `yaml:"clientPackageName,omitempty"`
	ClientMethod      string        `yaml:"clientMethod,omitempty"`
	Flags             []CommandFlag `yaml:"flags,omitempty"`
	Subcommands       []Command     `yaml:"subcommands,omitempty"`
	GraphQLPath       []string      `yaml:"path,omitempty"`
}

type CommandFlag struct {
	Name         string `yaml:"name,omitempty"`
	Type         string `yaml:"type,omitempty"`
	DefaultValue string `yaml:"defaultValue"`
	Description  string `yaml:"description"`
	VariableName string `yaml:"variableName"`
	Required     bool   `yaml:"required"`
}

// GeneratorConfig is the information necessary to execute a generator.
type GeneratorConfig struct {
	// Name is the string that is used to reference a generator.
	Name string `yaml:"name,omitempty"`
	// TemplateDir is the path to the directory that contains all of the templates.
	TemplateDir string `yaml:"templateDir,omitempty"`
	// FileName is the target file that is to be generated.
	FileName string `yaml:"fileName,omitempty"`
	// TemplateName is the name of the template within the TemplateDir.
	TemplateName string `yaml:"templateName,omitempty"`
	// TemplateURL is a URL to a downloadable file to use as a Go template
	TemplateURL string `yaml:"templateURL,omitempty"`
}

// MutationConfig is the information about the GraphQL mutations.
type MutationConfig struct {
	// Name is the name of the GraphQL method.
	Name                  string            `yaml:"name"`
	MaxQueryFieldDepth    int               `yaml:"max_query_field_depth,omitempty"`
	ArgumentTypeOverrides map[string]string `yaml:"argument_type_overrides,omitempty"`
	ExcludeFields         []string          `yaml:"exclude_fields,omitempty"`
	// ExcludeArguments lists GraphQL argument names to leave out of the generated
	// method signature and query string, for optional arguments that must not be
	// sent when the caller has no value (e.g. entityManagementDelete's `version`).
	// A name that matches no argument of the mutation is an error.
	ExcludeArguments []string `yaml:"exclude_arguments,omitempty"`
	// RenameArguments maps a GraphQL argument name to the Go parameter name, for
	// arguments whose default name is unusable (Go keywords get a safe name
	// automatically, but a chosen name keeps existing call sites readable).
	RenameArguments map[string]string `yaml:"rename_arguments,omitempty"`
	// ArgumentGoTypes maps a GraphQL argument name to the Go type of its parameter,
	// e.g. {cursor: "*string"} so a caller can pass nil for "not set". An unknown
	// name is an error.
	ArgumentGoTypes map[string]string `yaml:"argument_go_types,omitempty"`
	// IncludeOptionalArguments makes the method take the mutation's optional
	// (nullable) arguments as pointers, or as the slice they are for a list, so a
	// caller can leave one unset. An unset argument is left out of the request's
	// variables, so the API applies its own default instead of receiving the
	// type's zero value. exclude_arguments still removes an argument and
	// argument_go_types still sets a parameter's type. It changes the parameter
	// types of an existing method, so use it on mutations a package is adding.
	IncludeOptionalArguments bool `yaml:"include_optional_arguments,omitempty"`
	// KeepExisting leaves the mutation out of generation. Its method and query
	// constant stay in the file as hand-written code, for a mutation that was
	// renamed or reshaped by hand.
	KeepExisting bool `yaml:"keep_existing,omitempty"`
}

type EndpointConfig struct {
	Name               string   `yaml:"name,omitempty"`
	MaxQueryFieldDepth int      `yaml:"max_query_field_depth,omitempty"`
	IncludeArguments   []string `yaml:"include_arguments,omitempty"`
	ExcludeFields      []string `yaml:"exclude_fields,omitempty"`
	// ExcludeArguments lists GraphQL argument names to leave out of the generated
	// method signature and query string, for optional arguments the caller never
	// sets (e.g. accountManagement's `isCanceled`). A name that is not an argument
	// of the endpoint is an error.
	ExcludeArguments []string `yaml:"exclude_arguments,omitempty"`
	// ArgumentGoTypes maps a GraphQL argument name to the Go type of its parameter,
	// e.g. {cursor: "*string"}. An unknown name is an error.
	ArgumentGoTypes map[string]string `yaml:"argument_go_types,omitempty"`
	// IncludeOptionalArguments declares every optional (nullable) argument of the
	// endpoint in its query, where otherwise only the required ones and those in
	// IncludeArguments are, and makes the method take each as a pointer, or as the
	// slice it is for a list, so a caller can leave it unset. An unset argument is
	// left out of the request's variables, so the API applies its own default (e.g.
	// `first: 10`) instead of receiving null. exclude_arguments still removes an
	// argument and argument_go_types still sets a parameter's type. It changes the
	// parameter types of an existing method, so use it on endpoints a package is
	// adding.
	IncludeOptionalArguments bool `yaml:"include_optional_arguments,omitempty"`
	// MethodName is the complete Go method name (e.g. "GetDestinationsAccount").
	// It defaults to "Get" plus the endpoint name. Set it on each of two endpoints
	// that share a name under different query paths.
	MethodName string `yaml:"method_name,omitempty"`
	// ContextMethodName is the Go name of the variant that takes a context.Context,
	// for templates that generate one. It defaults to MethodName plus "WithContext".
	// Set it when an existing exported name puts the suffix elsewhere, e.g.
	// "GetDestinationsWithContextAccount" next to "GetDestinationsAccount".
	ContextMethodName string `yaml:"context_method_name,omitempty"`
	// IncludeImplementations narrows the interface this endpoint returns to the
	// named implementations, for this package only.
	IncludeImplementations []string `yaml:"include_implementations,omitempty"`
	// KeepExisting leaves the endpoint out of generation, so its methods and query
	// constant in the file stay as hand-written code. It is for endpoints renamed by
	// hand, such as two that share a name under different paths and would otherwise
	// fail as a duplicate method name. The types and the endpoint's response struct,
	// which the hand-written methods use, are still generated.
	KeepExisting bool `yaml:"keep_existing,omitempty"`
	// ArgumentTypeOverrides is accepted for existing configs and has no effect:
	// it has always been ignored on endpoints (only mutations use it). It stays a
	// known key so those configs keep loading under strict parsing.
	//
	// Deprecated: remove it from the config.
	ArgumentTypeOverrides []map[string]string `yaml:"argument_type_overrides,omitempty"`
}

// TypeConfig is the information about which types to render and any data specific to handling of the type.
type TypeConfig struct {
	// InterfaceMethods is a list of additional methods that are added to an interface definition. The methods are not
	// defined in the code, so must be implemented by the user.
	InterfaceMethods []string `yaml:"interface_methods,omitempty"`
	// Name of the type (required)
	Name string `yaml:"name"`
	// FieldTypeOverride is the Golang type to override whatever the default detected type would be for a given field.
	FieldTypeOverride string `yaml:"field_type_override,omitempty"`
	// CreateAs is used when creating a new scalar type to determine which Go type to use.
	CreateAs string `yaml:"create_as,omitempty"`
	// SkipTypeCreate skips creating the Go type, usually along with FieldTypeOverride.
	// A union with it is not generated at all, and its fields keep the type the
	// package has for it.
	SkipTypeCreate bool `yaml:"skip_type_create,omitempty"`
	// SkipFields allows the user to skip generating specific fields within a type.
	SkipFields []string `yaml:"skip_fields,omitempty"`
	// GenerateStructGetters enables the auto-generation of field getters for all fields on a struct.
	// i.e. if a struct has a field `name` then a function would be created called `GetName()`
	GenerateStructGetters bool `yaml:"generate_struct_getters,omitempty"`
	// Applies to all fields of the struct
	StructTags *StructTags `yaml:"struct_tags,omitempty"`
	// Fields holds per-field overrides, for when a whole-type setting such as
	// StructTags is too broad.
	Fields []FieldConfig `yaml:"fields,omitempty"`
	// IncludeImplementations narrows this interface or union type to the named
	// implementations, for this package only.
	IncludeImplementations []string `yaml:"include_implementations,omitempty"`
	// KeepExisting keeps the type's declaration in the existing file exactly as it
	// is, as if it were marked DO NOT REGENERATE, and so the declarations made for
	// it: its methods, an enum's <Name>Types variable, and an interface's
	// <Name>Interface type and Unmarshal<Name>Interface function. The type is still
	// expanded, so types that refer to it are generated as before, and anything the
	// file does not declare yet is generated normally.
	KeepExisting bool `yaml:"keep_existing,omitempty"`
}

// FieldConfig overrides how a single field of a generated struct is rendered.
// Unset options leave the default behaviour untouched.
type FieldConfig struct {
	// Name is the GraphQL field name (e.g. "enabled"), not the Go name.
	Name string `yaml:"name"`
	// Pointer makes the Go type a pointer when true. It is ignored for
	// interface-typed fields, which are handled by a generated UnmarshalJSON.
	Pointer *bool `yaml:"pointer,omitempty"`
	// OmitEmpty adds (true) or removes (false) `omitempty` on this field's tags.
	OmitEmpty *bool `yaml:"omitempty,omitempty"`
	// GoType replaces the whole Go type of the field, slice and pointer markers included.
	GoType string `yaml:"go_type,omitempty"`
	// GoName replaces the Go field name.
	GoName string `yaml:"go_name,omitempty"`
}

// GetFieldConfig returns the override for the named GraphQL field, or nil.
func (t *TypeConfig) GetFieldConfig(name string) *FieldConfig {
	if t == nil {
		return nil
	}

	for i := range t.Fields {
		if t.Fields[i].Name == name {
			return &t.Fields[i]
		}
	}

	return nil
}

type StructTags struct {
	// Set the type of struct tags - e.g. ["json"] or for multiple ["json", "yaml", etc...]
	// Note this will apply to ALL fields within the struct. Use with caution.
	Tags []string `yaml:"tags"`

	// Set to `false` to exclude `omitempty` from struct tags
	// Note this will apply to ALL fields within the struct. Use with caution.
	OmitEmpty *bool `yaml:"omitempty"`
}

const (
	DefaultCacheEnable     = false
	DefaultCacheSchemaFile = "schema.json"
	DefaultLogLevel        = "info"
	DefaultAuthHeader      = "Api-Key"
	DefaultAuthEnvVar      = "TUTONE_API_KEY"
)

// LoadConfig will load a config file at the specified path or error.
func LoadConfig(file string) (*Config, error) {
	if file == "" {
		return nil, errors.New("config file name required")
	}
	log.WithFields(log.Fields{
		"file": file,
	}).Debug("loading package definition")

	yamlFile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	// Strict, so a misspelled or unsupported key fails loudly and names the key
	// instead of being dropped, which would silently change what is generated.
	var config Config
	err = yaml.UnmarshalStrict(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", file, err)
	}
	log.Tracef("definition: %+v", config)

	warnIgnoredKeys(&config)

	return &config, nil
}

func (c *PackageConfig) GetDestinationPath() string {
	if c.Path != "" {
		return c.Path
	}

	return "./"
}

func (c *PackageConfig) GetTypeConfigByName(name string) *TypeConfig {
	for _, typeConfig := range c.Types {
		if typeConfig.Name == name {
			return &typeConfig
		}
	}

	return nil
}

// warnIgnoredKeys tells the user about settings that load but do nothing.
func warnIgnoredKeys(c *Config) {
	for _, p := range c.Packages {
		for _, name := range duplicateTypeNames(p.Types) {
			log.WithFields(log.Fields{
				"package": p.Name,
				"type":    name,
			}).Warn("type is configured more than once; only the first entry is used, so settings on later ones are ignored")
		}

		for _, q := range p.Queries {
			for _, e := range q.Endpoints {
				if len(e.ArgumentTypeOverrides) > 0 {
					log.WithFields(log.Fields{
						"package":  p.Name,
						"endpoint": e.Name,
					}).Warn("argument_type_overrides on an endpoint is ignored; only mutations use it. Remove it")
				}
			}
		}

		if names := keepExistingNames(p); len(names) > 0 && !p.GetPreserveHandWritten() {
			log.WithFields(log.Fields{
				"package": p.Name,
				"entries": names,
			}).Warn("keep_existing needs preserve_hand_written, which is false; the existing code is replaced, not kept")
		}

		if p.KeepRemovedFields && !p.GetPreserveHandWritten() {
			log.WithFields(log.Fields{
				"package": p.Name,
			}).Warn("keep_removed_fields needs preserve_hand_written, which is false; the existing code is replaced, not kept")
		}
	}
}

// keepExistingNames returns the types, mutations and endpoints the package sets to keep_existing.
func keepExistingNames(p PackageConfig) []string {
	var names []string

	for _, t := range p.Types {
		if t.KeepExisting {
			names = append(names, t.Name)
		}
	}

	for _, m := range p.Mutations {
		if m.KeepExisting {
			names = append(names, m.Name)
		}
	}

	for _, q := range p.Queries {
		for _, e := range q.Endpoints {
			if e.KeepExisting {
				names = append(names, e.Name)
			}
		}
	}

	return names
}

// duplicateTypeNames returns the type names that appear in more than one entry.
// Lookups use the first entry, so settings on a later one are silently ignored.
func duplicateTypeNames(types []TypeConfig) []string {
	seen := map[string]bool{}
	reported := map[string]bool{}

	var dups []string

	for _, t := range types {
		if seen[t.Name] && !reported[t.Name] {
			dups = append(dups, t.Name)
			reported[t.Name] = true
		}

		seen[t.Name] = true
	}

	return dups
}
