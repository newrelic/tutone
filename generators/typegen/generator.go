package typegen

import (
	"fmt"
	"os"
	"path"
	"text/template"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

type Generator struct {
	Types       []goStruct
	PackageName string
	Enums       []goEnum
	Imports     []string
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

// Generate is the entry point for this Generator.
func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	if genConfig == nil {
		return fmt.Errorf("unable to Generate with nil genConfig")
	}

	if pkgConfig == nil {
		return fmt.Errorf("unable to Generate with nil pkgConfig")
	}

	expandedTypes, err := schema.ExpandTypes(s, pkgConfig.Types)
	if err != nil {
		log.Error(err)
	}

	// TODO: Update return pattern to be tuple? - e.g. (result, err)
	if err := g.generateTypesForPackage(s, genConfig, pkgConfig, expandedTypes); err != nil {
		return err
	}

	return nil
}

func (g *Generator) generateTypesForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) error {
	// TODO: Putting the types in the specified path should be optional
	//       Should we use a flag or allow the user to omit that field in the config? ¿Por que no lost dos?

	var structsForGen []goStruct
	var enumsForGen []goEnum
	var err error

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
				var typeName string
				typeName, err = f.GetTypeNameWithOverride(pkgConfig)
				if err != nil {
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
			log.Debugf("default reached for kind %s, ignoring", t.Name)
		}
	}

	g.Types = structsForGen
	g.Enums = enumsForGen
	g.PackageName = pkgConfig.Name
	g.Imports = pkgConfig.Imports

	// Default to project root for types
	destinationPath := "./"
	if pkgConfig.Path != "" {
		destinationPath = pkgConfig.Path
	}

	if _, err = os.Stat(destinationPath); os.IsNotExist(err) {
		if err = os.Mkdir(destinationPath, 0755); err != nil {
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

	templateDir := "templates/typegen"
	if genConfig.TemplateDir != "" {
		templateDir = genConfig.TemplateDir
	}

	templatePath := path.Join(templateDir, templateName)

	tmpl, err := template.ParseFiles(templatePath)
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
