package config

import (
	"errors"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Config is the information keeper for generating go structs from type names.
type Config struct {
	LogLevel string `yaml:"log_level,omitempty"` // LogLevel sets the logging level

	Endpoint string      `yaml:"endpoint"`
	Auth     AuthConfig  `yaml:"auth"`
	Cache    CacheConfig `yaml:"cache"`

	Packages   []PackageConfig   `yaml:"packages,omitempty"`
	Generators []GeneratorConfig `yaml:"generators,omitempty"`
}

// AuthConfig is the information necessary to authenticate to the NerdGraph API.
type AuthConfig struct {
	Header string `yaml:"header,omitempty"`
	EnvVar string `yaml:"api_key_env_var,omitempty"`
}

// CacheConfig is the information necessary to store the NerdGraph schema in JSON.
type CacheConfig struct {
	Enable     bool   `yaml:",omitempty"`
	SchemaFile string `yaml:"schema_file,omitempty"`
}

// PackageConfig is the information about a single package, which types to include from the schema, and which generators to use for this package.
type PackageConfig struct {
	Name    string         `yaml:"name,omitempty"`
	Path    string         `yaml:"path,omitempty"`
	Types   []TypeConfig   `yaml:"types,omitempty"`
	Methods []MethodConfig `yaml:"methods,omitempty"`
	// Generators is a list of names that reference a generator in the Config struct.
	Generators []string `yaml:"generators,omitempty"`
	Imports    []string `yaml:"imports,omitempty"`
}

// GeneratorConfig is the information necessary to execute a generator.
type GeneratorConfig struct {
	Name            string `yaml:"name,omitempty"`
	DestinationFile string `yaml:"destination_file,omitempty"`
	TemplateDir     string `yaml:"template_dir,omitempty"`
	FileName        string `yaml:"fileName,omitempty"`
	TemplateName    string `yaml:"templateName,omitempty"`
}

type MethodConfig struct {
	Name string `yaml:"name"`
}

// TypeConfig is the information about which types to render and any data specific to handling of the type.
type TypeConfig struct {
	Name string `yaml:"name"`
	// FieldTypeOverride is the Golang type to override whatever the default detected type would be for a given field.
	FieldTypeOverride string `yaml:"field_type_override,omitempty"`
	// CreateAs is used when creating a new scalar type to determine which Go type to use.
	CreateAs string `yaml:"create_as,omitempty"`
	// SkipTypeCreate allows the user to skip creating a Scalar type.
	SkipTypeCreate bool `yaml:"skip_type_create,omitempty"`
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

	yamlFile, err := ioutil.ReadFile(file)
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
