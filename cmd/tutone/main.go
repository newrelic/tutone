package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/internal/util"
	"github.com/newrelic/tutone/pkg/fetch"
	"github.com/newrelic/tutone/pkg/generate"
)

var version = "dev"

var (
	appName = "tutone"
	cfgFile string
)

// Command represents the base command when called without any subcommands
var Command = &cobra.Command{
	Use:               appName,
	Short:             "Golang code generation from GraphQL",
	Long:              `Generate Go code based on the introspection of a GraphQL server`,
	Version:           version,
	DisableAutoGenTag: true, // Do not print generation date on documentation
}

func main() {
	err := Command.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	// Setup basic log stuff
	logFormatter := &log.TextFormatter{
		FullTimestamp: true,
		PadLevelText:  true,
	}
	log.SetFormatter(logFormatter)

	// Get Cobra going on flags
	cobra.OnInitialize(initConfig)

	// Config File
	Command.PersistentFlags().StringVarP(&cfgFile, "config", "c", ".tutone.yml", "Path to a configuration file")

	// Log level flag
	Command.PersistentFlags().StringP("loglevel", "l", "info", "Log level")
	viper.SetDefault("log_level", "info")
	util.LogIfError(log.ErrorLevel, viper.BindPFlag("log_level", Command.PersistentFlags().Lookup("loglevel")))

	// Add sub commands
	Command.AddCommand(fetch.Command)
	Command.AddCommand(generate.Command)
}

func initConfig() {
	viper.SetEnvPrefix("TUTONE")
	viper.AutomaticEnv()

	// Read config using Viper
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("tutone")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath(".tutone")
	}

	err := viper.ReadInConfig()
	if err != nil {
		switch e := err.(type) {
		case viper.ConfigFileNotFoundError:
			log.Debug("no config file found, using defaults")
		case viper.ConfigParseError:
			log.Errorf("error parsing config file: %v", e)
		default:
			log.Errorf("unknown error: %v", e)
		}
	}

	logLevel, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(logLevel)
}
