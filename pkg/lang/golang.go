package lang

import (
	"fmt"
	"sort"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"

	log "github.com/sirupsen/logrus"
)

// TODO: Move CommandGenerator and its friends to a proper home
type CommandGenerator struct {
	PackageName string
	Imports     []string
	Commands    []Command
}

type InputObject struct {
	Name   string
	GoType string
}

type Command struct {
	Name             string
	ShortDescription string
	LongDescription  string
	Example          string
	InputType        string
	ClientMethod     string
	ClientMethodArgs []string
	InputObjects     []InputObject
	Flags            []CommandFlag
	Subcommands      []Command
}

type CommandFlag struct {
	Name           string
	Type           string
	FlagMethodName string
	DefaultValue   string
	Description    string
	VariableName   string
	VariableType   string
	ClientType     string
	Required       bool
	IsInputType    bool
}

// GolangGenerator is enough information to generate Go code for a single package.
type GolangGenerator struct {
	Types       []GoStruct
	PackageName string
	Enums       []GoEnum
	Imports     []string
	Scalars     []GoScalar
	Interfaces  []GoInterface
	Mutations   []GoMethod
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

type GoMethod struct {
	Description string
	Name        string
	QueryVars   []QueryVar
	Signature   GoMethodSignature
	QueryString string
}

type GoMethodSignature struct {
	Input  []GoMethodInputType
	Return []string
}

type GoMethodInputType struct {
	Name string
	Type string
}

type QueryVar struct {
	Key   string
	Value string
	Type  string
}

func GenerateGoMethodsForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) (*[]GoMethod, error) {
	var methods []GoMethod

	for _, field := range s.MutationType.Fields {
		for _, pkgMutation := range pkgConfig.Mutations {

			if field.Name == pkgMutation.Name {

				method := GoMethod{
					Name:        field.Name,
					Description: field.GetDescription(),
				}

				// TODO It seem like we should never include the error here, and
				// instead assume an error is used in the method.
				// if field.Type.Name != "" {
				// 	pointerReturn := fmt.Sprintf("*%s", field.Type.Name)
				// 	method.Signature.Return = pointerReturn, "error"
				// }
				// Also, if we're trying to operate a field.Type without a name, what
				// is even happening?  Maybe that should be a log at the top of the
				// block with a continue.

				if field.Type.Name != "" {
					// pointerReturn := fmt.Sprintf("*%s", field.Type.Name)
					pointerReturn := field.Type.Name
					method.Signature.Return = []string{pointerReturn, "error"}
					method.QueryString = schema.PrefixLineTab(schema.PrefixLineTab(s.QueryFieldsForTypeName(field.Type.Name)))
				} else {
					method.Signature.Return = []string{"error"}
				}

				// field.Args are the arguments that are used to query the nerdgraph
				// method.  Here we build up the QueryVars object, as well as the
				// GoMethod.Signature.
				for _, methodArg := range field.Args {
					typeName, err := methodArg.GetTypeNameWithOverride(pkgConfig)
					if err != nil {
						log.Error(err)
						continue
					}

					inputType := GoMethodInputType{
						Name: methodArg.GetName(),
						Type: typeName,
					}

					// We should only need to create a query variable for method
					// arguments which are NON_NULL.
					if methodArg.Type.Kind == schema.KindNonNull {
						queryVar := QueryVar{
							Key:   methodArg.Name,
							Value: inputType.Name,
							Type:  methodArg.Type.OfType.Name,
						}

						method.QueryVars = append(method.QueryVars, queryVar)
					}

					method.Signature.Input = append(method.Signature.Input, inputType)
				}

				methods = append(methods, method)
			}
		}
	}

	if len(methods) > 0 {
		sort.SliceStable(methods, func(i, j int) bool {
			return methods[i].Name < methods[j].Name
		})
		return &methods, nil
	}

	return nil, fmt.Errorf("no methods for package")
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
				Name:        t.GetName(),
				Description: t.GetDescription(),
			}

			var fields []schema.Field
			fields = append(fields, t.Fields...)
			fields = append(fields, t.InputFields...)

			fieldErrs := []error{}
			for _, f := range fields {
				var typeName string
				var typeNamePrefix string
				var typeNameSuffix string

				typeName, err = f.GetTypeNameWithOverride(pkgConfig)
				if err != nil {
					fieldErrs = append(fieldErrs, err)
				}

				if f.Type.IsList() {
					typeNamePrefix = "[]"
				}

				// In the case a field type is of type Interface, we need to ensure we
				// append the term "Interface" to it, as is done in the "Implements"
				// below.
				if f.Type.OfType != nil {
					kinds := f.Type.OfType.GetKinds()
					if kinds[len(kinds)-1] == schema.KindInterface {
						typeNameSuffix = "Interface"
					}
				}

				field := GoStructField{
					Description: f.GetDescription(),
					Name:        f.GetName(),
					Tags:        f.GetTags(),
					Type:        fmt.Sprintf("%s%s%s", typeNamePrefix, typeName, typeNameSuffix),
				}

				xxx.Fields = append(xxx.Fields, field)
			}

			if len(fieldErrs) > 0 {
				log.Error(fieldErrs)
			}

			var implements []string
			for _, x := range t.Interfaces {
				implements = append(implements, x.GetName())
			}

			xxx.Implements = implements

			if t.Kind == schema.KindInterface {
				// Modify the struct type to avoid conflict with the interface type by the same name.
				// xxx.Name += "Type"

				// Ensure that the struct for the graphql interface implements the go interface
				xxx.Implements = append(xxx.Implements, t.GetName())

				// Handle the interface
				yyy := GoInterface{
					Description: t.GetDescription(),
					Name:        t.GetName(),
				}

				interfacesForGen = append(interfacesForGen, yyy)
			}

			structsForGen = append(structsForGen, xxx)
		case schema.KindENUM:
			xxx := GoEnum{
				Name:        t.GetName(),
				Description: t.GetDescription(),
			}

			for _, v := range t.EnumValues {
				value := GoEnumValue{
					Name:        v.GetName(),
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
