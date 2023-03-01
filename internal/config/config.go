package config

import (
	"errors"
	"os"

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

	// Transient property which is set by using the --include-integration-test flag.
	IncludeIntegrationTest bool
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
}

type EndpointConfig struct {
	Name               string   `yaml:"name,omitempty"`
	MaxQueryFieldDepth int      `yaml:"max_query_field_depth,omitempty"`
	IncludeArguments   []string `yaml:"include_arguments,omitempty"`
	ExcludeFields      []string `yaml:"exclude_fields,omitempty"`
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
	// SkipTypeCreate allows the user to skip creating a Scalar type.
	SkipTypeCreate bool `yaml:"skip_type_create,omitempty"`
	// SkipFields allows the user to skip generating specific fields within a type.
	SkipFields []string `yaml:"skip_fields,omitempty"`
	// GenerateStructGetters enables the auto-generation of field getters for all fields on a struct.
	// i.e. if a struct has a field `name` then a function would be created called `GetName()`
	GenerateStructGetters bool `yaml:"generate_struct_getters,omitempty"`
	// Applies to all fields of the struct
	StructTags *StructTags `yaml:"struct_tags,omitempty"`
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

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	log.Tracef("definition: %+v", config)

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
