package typegen

import (
	"os"
	"text/template"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/schema"
)

type Generator struct {
	Types       []goStruct
	PackageName string
	Enums       []goEnum
}

type goStruct struct {
	Name   string
	Doc    string
	Fields []goStructField
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

func (g *Generator) Generate(s *schema.Schema, tt *[]*schema.Type) error {
	var structsForGen []goStruct
	var enumsForGen []goEnum

	for _, t := range *tt {
		switch t.Kind {
		case schema.KindInputObject, schema.KindObject:

			xxx := goStruct{
				Name: t.Name,
				Doc:  t.Description,
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
				Description: t.Description,
			}

			for _, v := range t.EnumValues {
				value := goEnumValue{
					Name:        v.Name,
					Description: v.Description,
				}

				xxx.Values = append(xxx.Values, value)
			}

			enumsForGen = append(enumsForGen, xxx)
		default:
			log.Infof("default reached")
		}
	}

	g.Types = structsForGen
	g.Enums = enumsForGen

	// log.Infof("structsForGen: %+v", structsForGen)

	g.PackageName = "tmp"

	return fff(g, "templates/clientgo/types.go.tmpl", "tmp/types.go")
}

func fff(g *Generator, templatePath string, destination string) error {
	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return err
	}

	err = tmpl.Execute(f, g)
	if err != nil {
		return err
	}

	return nil
}
