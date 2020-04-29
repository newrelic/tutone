package config

import (
	log "github.com/sirupsen/logrus"
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
}

type AuthConfig struct {
	Header string `yaml:",omitempty"`
	EnvVar string `yaml:"env_var,omitempty"`
}

type CacheConfig struct {
	Enable     bool   `yaml:",omitempty"`
	SchemaFile string `yaml:"schema_file,omitempty"`
}

//type TypeConfig struct {
//	Name     string `yaml:"name"`
//	CreateAs string `yaml:"createAs,omitempty"` // CreateAs is the Golang type to override whatever the default detected type would be
//}

const (
	DefaultCacheEnable     = false
	DefaultCacheSchemaFile = "schema.json"
	DefaultLogLevel        = "info"
	DefaultAuthHeader      = "Api-Key"
	DefaultAuthEnvVar      = "TUTONE_API_KEY"
)

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
