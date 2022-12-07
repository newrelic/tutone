package lang

import (
	"fmt"

	"github.com/iancoleman/strcase"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

type TerraformResourceGenerator struct {
	PackageName string
	Imports     []string
	Resources   []Resource
}

type Resource struct {
	Name       string
	FileName   string
	Attributes []TerraformSchemaAttribute
}

type TerraformSchemaAttribute struct {
	Key  string
	Type string
}

func GenerateSchemaAttributes(s *schema.Schema, resourceConfig *config.ResourceConfig) (*[]TerraformSchemaAttribute, error) {
	var attributes []TerraformSchemaAttribute

	fields := s.LookupMutationsByPattern("alertsPolicyCreate")

	// The terraform attributes will likely be generated based on the GraphQL args

	fmt.Print("\n****************************\n")

	var args []schema.Field
	for _, f := range fields {
		fmt.Printf("\n GenerateSchemaAttributes:  %+v \n", f.Args)

		args = append(args, f.Args...)
	}

	for _, arg := range args {
		if arg.Type.OfType.Kind == schema.KindScalar {
			attr := TerraformSchemaAttribute{
				Key:  strcase.ToSnake(arg.Name),
				Type: "schema.TypeInt",
			}

			attributes = append(attributes, attr)
		}
	}

	fmt.Printf("\n GenerateSchemaAttributes - attrs:  %+v \n", attributes)

	fmt.Print("\n****************************\n")

	// baseResourceType // Find the type from the schema.json file

	return &attributes, nil
}
