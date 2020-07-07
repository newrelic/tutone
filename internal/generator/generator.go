package generator

import "github.com/newrelic/tutone/internal/schema"

// Generator aspires to implement the interface between a NerdGraph schema and
// generated code for another project.
type Generator interface {
	// Generate is expected to
	Generate(*schema.Schema, *[]*schema.Type) error
}
