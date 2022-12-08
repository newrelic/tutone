package lang

import (
	"strings"

	"github.com/apex/log"
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
	Name              string
	FileName          string
	Attributes        []TerraformSchemaAttribute
	ClientPackageName string
	ClientMethod      []string
	InputObject       string
}

type TerraformSchemaAttribute struct {
	Key         string
	Type        string
	Required    bool
	Description string
}

type TerraformResourceTemplateData struct {
	Resource

	Imports     []string
	PackageName string
}

func GetMutationInputType(s *schema.Schema, resourceConfig *config.ResourceConfig, pkgConfig *config.PackageConfig) string {
	fields := s.LookupMutationsByPattern(resourceConfig.CreateMutation)

	var args []schema.Field
	for _, f := range fields {
		args = append(args, f.Args...)
	}

	for _, arg := range args {
		typeName, _ := arg.GetTypeNameWithOverride(pkgConfig)
		t, _ := s.LookupTypeByName(typeName)

		if t == nil {
			log.Debugf("no type name found for %s", arg)
			continue
		}

		switch t.Kind {
		case schema.KindInputObject:
			return t.Name
		}
	}

	return ""
}

func GenerateSchemaAttributes(s *schema.Schema, resourceConfig *config.ResourceConfig, pkgConfig *config.PackageConfig) (*[]TerraformSchemaAttribute, error) {
	var attributes []TerraformSchemaAttribute

	fields := s.LookupMutationsByPattern(resourceConfig.CreateMutation)

	var args []schema.Field
	for _, f := range fields {
		args = append(args, f.Args...)
	}

	for _, arg := range args {
		if arg.Type.OfType.Kind == schema.KindScalar {
			attr := TerraformSchemaAttribute{
				Key:         strcase.ToSnake(arg.Name),
				Type:        "schema.TypeInt",
				Description: arg.Description,
			}

			if arg.IsRequired() {
				attr.Required = true
			}

			attributes = append(attributes, attr)
		}

		typeName, _ := arg.GetTypeNameWithOverride(pkgConfig)
		t, _ := s.LookupTypeByName(typeName)

		if t == nil {
			log.Debugf("no type name found for %s", arg)
			continue
		}

		switch t.Kind {
		case schema.KindInputObject:
			for _, field := range t.InputFields {
				attr := TerraformSchemaAttribute{
					Key:         strcase.ToSnake(field.Name),
					Type:        "schema.TypeString",
					Description: strings.Trim(field.GetDescription(), "/ "),
				}

				if field.IsEnum() {
					attr.Type = "schema.TypeString"
				}

				if field.IsRequired() {
					attr.Required = true
				}

				attributes = append(attributes, attr)
			}
		}
	}

	return &attributes, nil
}
