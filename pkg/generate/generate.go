package generate

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/generators/typegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/generator"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/fetch"
)

// Generate reads the configuration file and executes generators relevant to a particular package.
func Generate(refetch bool) error {
	fmt.Print("\n GENERATE..... \n")

	defFile := viper.GetString("definition")
	schemaFile := viper.GetString("cache.schema_file")
	typesFile := viper.GetString("generate.types_file")

	_, err := os.Stat(schemaFile)

	// Fetch a new schema if it doesn't exist or if --refetch flag has been provided.
	if os.IsNotExist(err) || refetch {
		fetch.Fetch(
			viper.GetString("endpoint"),
			viper.GetString("auth.header"),
			viper.GetString("auth.api_key_env_var"),
			schemaFile,
		)
	}

	log.WithFields(log.Fields{
		"definition_file": defFile,
		"schema_file":     schemaFile,
		"types_file":      typesFile,
	}).Info("Loading generation config")

	// load the config
	cfg, err := config.LoadConfig(viper.ConfigFileUsed())
	if err != nil {
		return err
	}

	log.Debugf("config: %+v", cfg)

	// package is required
	if len(cfg.Packages) == 0 {
		return fmt.Errorf("an array of packages is required")
	}

	// Load the schema
	s, err := schema.Load(schemaFile)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"count_packages":   len(cfg.Packages),
		"count_generators": len(cfg.Generators),
		// "count_mutation":     len(cfg.Mutations),
		// "count_query":        len(cfg.Queries),
		// "count_subscription": len(cfg.Subscriptions),
	}).Info("starting code generation")

	allGenerators := map[string]generator.Generator{
		// &terraform.Generator{},
		"typegen": &typegen.Generator{},
	}

	for _, pkgConfig := range cfg.Packages {
		log.WithFields(log.Fields{
			"name":          pkgConfig.Name,
			"generators":    pkgConfig.Generators,
			"count_type":    len(pkgConfig.Types),
			"count_imports": len(pkgConfig.Imports),
		}).Info("generating package")

		for _, generatorName := range pkgConfig.Generators {

			ggg, err := getGeneratorByName(generatorName, allGenerators)
			if err != nil {
				log.Error(err)
				continue
			}

			genConfig, err := getGeneratorConfigByName(generatorName, cfg.Generators)
			if err != nil {
				log.Error(err)
				continue
			}

			if ggg != nil && genConfig != nil {
				g := *ggg

				err = g.Generate(s, genConfig, &pkgConfig)
				if err != nil {
					return fmt.Errorf("unable to generate for provider %T: %s", generatorName, err)
				}
			}
		}
	}

	return nil
}

// getGeneratorConfigByName retrieve the *config.GeneratorConfig from the given set or errros.
func getGeneratorConfigByName(name string, matchSet []config.GeneratorConfig) (*config.GeneratorConfig, error) {
	for _, g := range matchSet {
		if g.Name == name {
			return &g, nil
		}
	}

	return nil, fmt.Errorf("no generatorConfig with name %s found", name)
}

// getGeneratorByName retrieve the *generator.Generator from the given set or errros.
func getGeneratorByName(name string, matchSet map[string]generator.Generator) (*generator.Generator, error) {
	for n, g := range matchSet {
		if n == name {
			return &g, nil
		}
	}

	return nil, fmt.Errorf("no generator named %s found", name)
}
