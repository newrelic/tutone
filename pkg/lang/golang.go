package lang

import (
	"fmt"
	"sort"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"

	log "github.com/sirupsen/logrus"
)

// GolangGenerator is enough information to generate Go code for a single package.
type GolangGenerator struct {
	Types       []GoStruct
	PackageName string
	Enums       []GoEnum
	Imports     []string
	Scalars     []GoScalar
	Interfaces  []GoInterface
}

type GoStruct struct {
	Name        string
	Description string
	Fields      []GoStructField
	Implements  []string
}

type GoStructField struct {
	Name        string
	Type        string
	Tags        string
	Description string
}

type GoEnum struct {
	Name        string
	Description string
	Values      []GoEnumValue
}

type GoEnumValue struct {
	Name        string
	Description string
}

type GoScalar struct {
	Name        string
	Description string
	Type        string
}

type GoInterface struct {
	Name        string
	Description string
	Type        string
}

func GenerateGoTypesForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) (*[]GoStruct, *[]GoEnum, *[]GoScalar, *[]GoInterface, error) {
	// TODO: Putting the types in the specified path should be optional
	//       Should we use a flag or allow the user to omit that field in the config? ¿Por que no lost dos?

	var structsForGen []GoStruct
	var enumsForGen []GoEnum
	var scalarsForGen []GoScalar
	var interfacesForGen []GoInterface

	var err error

	for _, t := range *expandedTypes {
		switch t.Kind {
		case schema.KindInputObject, schema.KindObject, schema.KindInterface:
			xxx := GoStruct{
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

				field := GoStructField{
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
				// xxx.Name += "Type"

				// Ensure that the struct for the graphql interface implements the go interface
				xxx.Implements = append(xxx.Implements, t.Name)

				// Handle the interface
				yyy := GoInterface{
					Description: t.GetDescription(),
					// Append "Interface" to Go interface names
					// to avoid name conflicts with types/structs
					Name: t.GetName() + "Interface",
				}

				interfacesForGen = append(interfacesForGen, yyy)
			}

			structsForGen = append(structsForGen, xxx)
		case schema.KindENUM:
			xxx := GoEnum{
				Name:        t.Name,
				Description: t.GetDescription(),
			}

			for _, v := range t.EnumValues {
				value := GoEnumValue{
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
				xxx := GoScalar{
					Description: t.GetDescription(),
					Name:        t.GetName(),
					Type:        createAs,
				}

				scalarsForGen = append(scalarsForGen, xxx)
			}
		// case schema.KindInterface:
		// 	xxx := GoInterface{
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

	sort.SliceStable(structsForGen, func(i, j int) bool {
		return structsForGen[i].Name < structsForGen[j].Name
	})

	sort.SliceStable(enumsForGen, func(i, j int) bool {
		return enumsForGen[i].Name < enumsForGen[j].Name
	})

	sort.SliceStable(scalarsForGen, func(i, j int) bool {
		return scalarsForGen[i].Name < scalarsForGen[j].Name
	})

	sort.SliceStable(interfacesForGen, func(i, j int) bool {
		return interfacesForGen[i].Name < interfacesForGen[j].Name
	})

	return &structsForGen, &enumsForGen, &scalarsForGen, &interfacesForGen, nil
}
