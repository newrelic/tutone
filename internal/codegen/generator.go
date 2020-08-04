package codegen

import (
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

// Generator aspires to implement the interface between a NerdGraph schema and
// generated code for another project.
type Generator interface {
	Generate(*schema.Schema, *config.GeneratorConfig, *config.PackageConfig) error
	Execute(*config.GeneratorConfig, *config.PackageConfig) error
}
