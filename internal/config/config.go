package config

import (
	"errors"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/newrelic/tutone/internal/schema"
)

// Config is the information keeper for generating go structs from type names.
type Config struct {
	LogLevel string `yaml:"log_level,omitempty"` // LogLevel sets the logging level

	Endpoint string
	Auth     AuthConfig
	Caching  CacheConfig

	//Package string       `yaml:"package"`
	//Types   []TypeConfig `yaml:"types"`
	//Verbose bool
	//client  *newrelic.NewRelic
	Packages   []PackageConfig   `yaml:"packages,omitempty"`
	Generators []GeneratorConfig `yaml:"generators,omitempty"`
}

// AuthConfig is the information necessary to authenticate to the NerdGraph API.
type AuthConfig struct {
	Header string `yaml:",omitempty"`
	EnvVar string `yaml:"env_var,omitempty"`
}

// CacheConfig is the information necessary to store the NerdGraph schema in JSON.
type CacheConfig struct {
	Enable     bool   `yaml:",omitempty"`
	SchemaFile string `yaml:"schema_file,omitempty"`
}

//type TypeConfig struct {
//	Name     string `yaml:"name"`
//	CreateAs string `yaml:"createAs,omitempty"` // CreateAs is the Golang type to override whatever the default detected type would be
//}

// PackageConfig is the information about a single package, which types to include from the schema, and which generators to use for this package.
type PackageConfig struct {
	Name  string            `yaml:"name,omitempty"`
	Path  string            `yaml:"path,omitempty"`
	Types []schema.TypeInfo `yaml:"types,omitempty"`
	// Generators is a list of names that reference a generator in the Config struct.
	Generators []string `yaml:"generators,omitempty"`
}

// GeneratorConfig is the information necessary to execute a generator.
type GeneratorConfig struct {
	Name            string `yaml:"name,omitempty"`
	DestinationFile string `yaml:"destination_file,omitempty"`
	TemplateDir     string `yaml:"template_dir,omitempty"`
	FileName        string `yaml:"fileName,omitempty"`
	TemplateName    string `yaml:"templateName,omitempty"`
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

// New creates a new Config.
func New() *Config {
	cfg := Config{
		Auth: AuthConfig{
			Header: DefaultAuthHeader,
			EnvVar: DefaultAuthEnvVar,
		},
		Caching: CacheConfig{
			Enable:     DefaultCacheEnable,
			SchemaFile: DefaultCacheSchemaFile,
		},
		LogLevel: DefaultLogLevel,
	}

	return &cfg
}

func (c *Config) Load() error {
	var err error

	//verbose := flag.Bool("v", false, "increase verbosity")
	//flag.StringVar(&c.Package, "p", "", "package name")

	//flag.Parse()
	logLvl, err := log.ParseLevel(c.LogLevel)
	if err != nil {
		log.SetLevel(logLvl)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	//apiKey := os.Getenv(c.Auth.EnvVar)
	//c.client, err = newrelic.New(newrelic.ConfigPersonalAPIKey(apiKey), newrelic.ConfigLogLevel(log.GetLevel().String()))
	//if err != nil {
	//	return nil
	//}

	return nil
}
