package generate

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/generators/command"
	"github.com/newrelic/tutone/generators/nerdgraphclient"
	"github.com/newrelic/tutone/generators/terraform"
	"github.com/newrelic/tutone/generators/typegen"
	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/fetch"
)

type GeneratorOptions struct {
	PackageName string
	Refetch     bool
}

// Generate reads the configuration file and executes generators relevant to a particular package.
func Generate(options GeneratorOptions) error {
	schemaFile := viper.GetString("cache.schema_file")

	_, err := os.Stat(schemaFile)

	// Fetch a new schema if it doesn't exist or if --refetch flag has been provided.
	if os.IsNotExist(err) || options.Refetch {
		fetch.Fetch(
			viper.GetString("endpoint"),
			viper.GetBool("auth.disable"),
			viper.GetString("auth.header"),
			viper.GetString("auth.api_key_env_var"),
			schemaFile,
			options.Refetch,
		)
	}

	log.WithFields(log.Fields{
		"schema_file": schemaFile,
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

	// Generate for a specific package
	if options.PackageName != "" {
		return generateForPackage(options.PackageName, cfg, s)
	}

	// Generate for all configured packages
	for _, pkgConfig := range cfg.Packages {
		if err := generateForPackage(pkgConfig.Name, cfg, s); err != nil {
			return err
		}
	}

	return nil
}

func findPackageConfigByName(name string, packages []config.PackageConfig) *config.PackageConfig {
	for _, p := range packages {
		if p.Name == name {
			return &p
		}
	}

	return nil
}

func generatePkgTypes(pkgConfig *config.PackageConfig, cfg *config.Config, s *schema.Schema) error {
	allGenerators := map[string]codegen.Generator{
		"terraform":       &terraform.Generator{},
		"typegen":         &typegen.Generator{},
		"nerdgraphclient": &nerdgraphclient.Generator{},
		"command":         &command.Generator{},
	}

	log.WithFields(log.Fields{
		"name":            pkgConfig.Name,
		"generators":      pkgConfig.Generators,
		"count_type":      len(pkgConfig.Types),
		"count_imports":   len(pkgConfig.Imports),
		"count_resources": len(pkgConfig.Resources),
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

			log.WithFields(log.Fields{
				"generator": generatorName,
			}).Info("starting generator")

			err = g.Generate(s, genConfig, pkgConfig)
			if err != nil {
				return fmt.Errorf("failed to call Generate() for provider %T: %s", generatorName, err)
			}

			err = g.Execute(genConfig, pkgConfig)
			if err != nil {
				return fmt.Errorf("failed to call Execute() for provider %T: %s", generatorName, err)
			}
		}
	}

	return nil
}

func generateForPackage(packageName string, cfg *config.Config, schema *schema.Schema) error {
	pkg := findPackageConfigByName(packageName, cfg.Packages)

	if pkg == nil {
		return fmt.Errorf("[Error] package %v not found", packageName)
	}

	return generatePkgTypes(pkg, cfg, schema)
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
func getGeneratorByName(name string, matchSet map[string]codegen.Generator) (*codegen.Generator, error) {
	for n, g := range matchSet {
		if n == name {
			return &g, nil
		}
	}

	return nil, fmt.Errorf("no generator named %s found", name)
}
