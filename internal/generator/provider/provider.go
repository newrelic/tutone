package provider

import "github.com/newrelic/tutone/internal/schema"

// Provider aspires to implement the interface between a NerdGraph schema and
// generated code for another project.
type Provider interface {
	// Generate is expected to
	Generate(*schema.Schema, *[]*schema.Type) error
}
