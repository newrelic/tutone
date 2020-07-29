package typegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path"
	"sort"
	"text/template"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
)

type Generator struct {
	lang.GolangGenerator
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

	structsForGen, enumsForGen, scalarsForGen, interfacesForGen, err := g.generateTypesForPackage(s, genConfig, pkgConfig, expandedTypes)
	if err != nil {
		return err
	}

	// The do() below expects to have Generator g populated for use in the template files.
	g.PackageName = pkgConfig.Name
	g.Imports = pkgConfig.Imports

	if structsForGen != nil {
		g.Types = *structsForGen
	}

	if enumsForGen != nil {
		g.Enums = *enumsForGen
	}

	if scalarsForGen != nil {
		g.Scalars = *scalarsForGen
	}

	if interfacesForGen != nil {
		g.Interfaces = *interfacesForGen
	}

	err = g.do(genConfig, pkgConfig)
	if err != nil {
		return err
	}

	return nil
}

func (g *Generator) generateTypesForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) (*[]lang.GoStruct, *[]lang.GoEnum, *[]lang.GoScalar, *[]lang.GoInterface, error) {
	// TODO: Putting the types in the specified path should be optional
	//       Should we use a flag or allow the user to omit that field in the config? ¿Por que no lost dos?

	var structsForGen []lang.GoStruct
	var enumsForGen []lang.GoEnum
	var scalarsForGen []lang.GoScalar
	var interfacesForGen []lang.GoInterface

	var err error

	for _, t := range *expandedTypes {
		switch t.Kind {
		case schema.KindInputObject, schema.KindObject, schema.KindInterface:
			xxx := lang.GoStruct{
				Name:        t.Name,
				Description: t.GetDescription(),
			}

			var fields []schema.Field
			fields = append(fields, t.Fields...)
			fields = append(fields, t.InputFields...)

			fieldErrs := []error{}
			for _, f := range fields {
				var typeName string
				var typeNamePrefix string

				typeName, err = f.GetTypeNameWithOverride(pkgConfig)
				if err != nil {
					fieldErrs = append(fieldErrs, err)
				}

				if f.Type.IsList() {
					typeNamePrefix = "[]"
				}

				field := lang.GoStructField{
					Description: f.GetDescription(),
					Name:        f.GetName(),
					Tags:        f.GetTags(),
					Type:        fmt.Sprintf("%s%s", typeNamePrefix, typeName),
				}

				xxx.Fields = append(xxx.Fields, field)
			}

			if len(fieldErrs) > 0 {
				log.Error(fieldErrs)
			}

			var implements []string
			for _, x := range t.Interfaces {
				implements = append(implements, x.Name)
			}

			xxx.Implements = implements

			if t.Kind == schema.KindInterface {
				// Modify the struct type to avoid conflict with the interface type by the same name.
				xxx.Name += "Type"

				// Handle the interface
				yyy := lang.GoInterface{
					Description: t.GetDescription(),
					Name:        t.GetName(),
				}

				interfacesForGen = append(interfacesForGen, yyy)
			}

			structsForGen = append(structsForGen, xxx)
		case schema.KindENUM:
			xxx := lang.GoEnum{
				Name:        t.Name,
				Description: t.GetDescription(),
			}

			for _, v := range t.EnumValues {
				value := lang.GoEnumValue{
					Name:        v.Name,
					Description: v.GetDescription(),
				}

				xxx.Values = append(xxx.Values, value)
			}

			enumsForGen = append(enumsForGen, xxx)
		case schema.KindScalar:
			// Default scalars to string
			createAs := "string"
			skipTypeCreate := false
			nameToMatch := t.GetName()

			var seenNames []string
			for _, p := range pkgConfig.Types {
				if stringInStrings(p.Name, seenNames) {
					log.Warnf("duplicate package config name detected: %s", p.Name)
					continue
				}
				seenNames = append(seenNames, p.Name)

				if p.Name == nameToMatch {
					if p.CreateAs != "" {
						createAs = p.CreateAs
					}

					if p.SkipTypeCreate {
						skipTypeCreate = true
					}
				}
			}

			if !t.IsGoType() && !skipTypeCreate {
				xxx := lang.GoScalar{
					Description: t.GetDescription(),
					Name:        t.GetName(),
					Type:        createAs,
				}

				scalarsForGen = append(scalarsForGen, xxx)
			}
		// case schema.KindInterface:
		// 	xxx := lang.GoInterface{
		// 		Description: t.GetDescription(),
		// 		Name:        t.GetName(),
		// 	}
		//
		// 	interfacesForGen = append(interfacesForGen, xxx)
		default:
			log.WithFields(log.Fields{
				"name": t.Name,
				"kind": t.Kind,
			}).Warn("kind not implemented")
		}
	}

	return &structsForGen, &enumsForGen, &scalarsForGen, &interfacesForGen, nil
}

// do performs the template render and file writement, according to the received configurations for the current Generator instance.
func (g *Generator) do(genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	var err error

	sort.SliceStable(g.Types, func(i, j int) bool {
		return g.Types[i].Name < g.Types[j].Name
	})

	sort.SliceStable(g.Enums, func(i, j int) bool {
		return g.Enums[i].Name < g.Enums[j].Name
	})

	sort.SliceStable(g.Scalars, func(i, j int) bool {
		return g.Scalars[i].Name < g.Scalars[j].Name
	})

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
	file, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
	}
	defer file.Close()

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

	var resultBuf bytes.Buffer

	err = tmpl.Execute(&resultBuf, g)
	if err != nil {
		return err
	}

	formatted, err := format.Source(resultBuf.Bytes())
	if err != nil {
		return err
	}

	// Rewrite the file with the formatted output
	_, err = file.WriteAt(formatted, 0)
	if err != nil {
		return err
	}

	return nil
}

func stringInStrings(s string, ss []string) bool {
	for _, sss := range ss {
		if s == sss {
			return true
		}
	}

	return false
}
