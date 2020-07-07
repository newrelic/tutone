package generate

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/newrelic/tutone/internal/provider"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/providers/clientgo"
)

var providers = []provider.Provider{
	// &terraform.Generator{},
	&clientgo.Generator{},
}

// The big show
func Generate() error {
	defFile := viper.GetString("definition")
	schemaFile := viper.GetString("schema_file")
	typesFile := viper.GetString("generate.types_file")
	packageName := viper.GetString("package")

	log.WithFields(log.Fields{
		"definition_file": defFile,
		"schema_file":     schemaFile,
		"types_file":      typesFile,
		"package_name":    packageName,
	}).Info("loading generation config")

	// load the config
	cfg, err := LoadConfig(defFile)
	if err != nil {
		return err
	}

	// CLI Overrides
	if packageName != "" {
		cfg.Package = packageName
	}

	// package is required
	if cfg.Package == "" {
		return errors.New("package name required")
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
		"count_mutation":     len(cfg.Mutations),
		"count_query":        len(cfg.Queries),
		"count_subscription": len(cfg.Subscriptions),
		"count_type":         len(cfg.Types),
		"package":            cfg.Package,
	}).Info("starting code generation")

	for _, p := range providers {
		// p.LoadConfig()

		log.Debugf("generating for %T", p)
		err = p.Generate(s)
		if err != nil {
			return fmt.Errorf("unable to generate for provider %T: %s", p, err)
		}
	}

	return nil
}
