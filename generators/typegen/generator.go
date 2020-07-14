package typegen

import (
	"fmt"
	"os"
	"text/template"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

type Generator struct {
	Types       []goStruct
	PackageName string
	Enums       []goEnum
}

type goStruct struct {
	Name        string
	Description string
	Fields      []goStructField
}

type goStructField struct {
	Name string
	Type string
	Tags string
	Doc  string
}

type goEnum struct {
	Name        string
	Description string
	Values      []goEnumValue
}

type goEnumValue struct {
	Name        string
	Description string
}

func (g *Generator) Generate(s *schema.Schema, config *config.Config) error {
	for _, pkg := range config.Packages {
		expandedTypes, err := schema.ExpandTypes(s, pkg.Types)
		if err != nil {
			log.Error(err)
		}

		for _, genConfig := range pkg.Generators {
			if genConfig.Name == "typegen" {
				if err := g.generateTypesForPackage(pkg, s, expandedTypes, genConfig); err != nil {
					return err
				}
			}
		}

	}

	return nil
}

// generateTypesForPackage assumes usage with the "typegen" generator.
func (g *Generator) generateTypesForPackage(pkg config.Package, schemaInput *schema.Schema, expandedTypes *[]*schema.Type, genConfig config.GeneratorConfig) error {
	// TODO: Putting the types in the specified path should be optional
	//       Should we use a flag or allow the user to omit that field in the config? ¿Por que no lost dos?

	var structsForGen []goStruct
	var enumsForGen []goEnum

	for _, t := range *expandedTypes {
		switch t.Kind {
		case schema.KindInputObject, schema.KindObject:

			xxx := goStruct{
				Name:        t.Name,
				Description: t.GetDescription(),
			}

			var fields []schema.Field
			fields = append(fields, t.Fields...)
			fields = append(fields, t.InputFields...)

			fieldErrs := []error{}
			for _, f := range fields {
				typeName, _, err := f.Type.GetType()
				if err != nil {
					log.Error(err)
					fieldErrs = append(fieldErrs, err)
				}

				field := goStructField{
					Doc:  f.GetDescription(),
					Name: f.GetName(),
					Tags: f.GetTags(),
					Type: typeName,
				}

				xxx.Fields = append(xxx.Fields, field)
			}

			if len(fieldErrs) > 0 {
				log.Error(fieldErrs)
			}

			structsForGen = append(structsForGen, xxx)
		case schema.KindENUM:
			xxx := goEnum{
				Name:        t.Name,
				Description: t.GetDescription(),
			}

			for _, v := range t.EnumValues {
				value := goEnumValue{
					Name:        v.Name,
					Description: v.GetDescription(),
				}

				xxx.Values = append(xxx.Values, value)
			}

			enumsForGen = append(enumsForGen, xxx)
		default:
			log.Debugf("default reached for: %s of kind: %+v", t.Name, t.Kind)
		}
	}

	g.Types = structsForGen
	g.Enums = enumsForGen
	g.PackageName = pkg.Name

	// Default to project root for types
	destinationPath := "./"
	if pkg.Path != "" {
		destinationPath = pkg.Path
	}

	if _, err := os.Stat(destinationPath); os.IsNotExist(err) {
		if err := os.Mkdir(destinationPath, 0755); err != nil {
			log.Error(err)
		}
	}

	// Default file name is 'types.go'
	fileName := "types.go"
	if genConfig.FileName != "" {
		fileName = genConfig.FileName
	}

	filePath := fmt.Sprintf("%s/%s", destinationPath, fileName)
	f, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
	}
	defer f.Close()

	templateName := "types.go.tmpl"
	if genConfig.TemplateName != "" {
		templateName = genConfig.TemplateName
	}

	tmpl, err := template.ParseFiles(templateName)
	if err != nil {
		return err
	}

	err = tmpl.Execute(f, g)
	if err != nil {
		return err
	}

	// TODO: Imports?? Check old implementation

	// keys := make([]string, 0, len(schema.Types))
	// for k := range schema.Types {
	// 	keys = append(keys, k)
	// }
	// sort.Strings(keys)
	//
	// for _, k := range keys {
	// 	_, err := f.WriteString(schema.Types[k])
	// 	if err != nil {
	// 		log.Error(err)
	// 	}
	// }

	return nil
}
