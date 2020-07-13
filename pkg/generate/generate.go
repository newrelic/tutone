package generate

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/generators/typegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/generator"
	"github.com/newrelic/tutone/internal/schema"
)

var generators = map[string]generator.Generator{
	// &terraform.Generator{},
	"typegen": &typegen.Generator{},
}

// The big show
func Generate() error {
	fmt.Print("\n GENERATE..... \n")

	defFile := viper.GetString("definition")
	schemaFile := viper.GetString("schema_file")
	typesFile := viper.GetString("generate.types_file")
	// packageName := viper.GetString("package")

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
		"schema": s,
	}).Trace("loaded schema")

	log.WithFields(log.Fields{
		"packages": cfg.Packages,
		// "count_mutation":     len(cfg.Mutations),
		// "count_query":        len(cfg.Queries),
		// "count_subscription": len(cfg.Subscriptions),
		// "count_type":         len(cfg.Types),
		// "package":            cfg.Package,
	}).Info("starting code generation")

	for _, pkg := range cfg.Packages {
		for _, pkgGenerator := range pkg.Generators {

			for generatorName, generator := range generators {
				if pkgGenerator.Name == generatorName {
					err = generator.Generate(s, cfg)
					if err != nil {
						return fmt.Errorf("unable to generate for provider %T: %s", generatorName, err)
					}
					//
				}

			}

		}
	}

	return nil
}
