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
	Methods     []GoMethod
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

func getRootQueryFields(gqlRootQueryTypeName string, types []*schema.Type) []schema.Field {
	for _, t := range types {
		if t.Name == gqlRootQueryTypeName {
			return t.Fields
		}
	}

	return []schema.Field{}
}

func getTypeMetadata(typeName string, types []*schema.Type) []schema.Field {
	for _, t := range types {
		if t.Name == typeName {
			return t.Fields
		}
	}

	return []schema.Field{}
}

func GenerateGoMethodsForPackage(
	s *schema.Schema,
	genConfig *config.GeneratorConfig,
	pkgConfig *config.PackageConfig,
) (*[]GoMethod, error) {
	var methods []GoMethod

	log.Print("\n\n **************************** \n")

	// gqlRootQueryTypeName := s.QueryType.Name
	// gqlRootQueryFields := getRootQueryFields(gqlRootQueryTypeName, s.Types)

	// gqlActorMetadata := getTypeMetadata("Actor", s.Types)
	// gqlAccountMetadata := getTypeMetadata("Account", s.Types)

	// log.Printf("\n gqlActorMetadata:  %+v \n", gqlActorMetadata)
	// log.Printf("\n gqlAccountMetadata:  %+v \n", gqlAccountMetadata)

	// for _, field := range s.Types {

	// 	// log.Printf("\n FIELD - name:  %+v \n", field.Name)
	// 	// log.Printf("  FIELD - kind:  %+v \n", field.Kind)
	// 	// log.Printf("  FIELD - fields:  %+v \n", field.Fields)

	// 	// 	// if field.Name == "actor" {
	// 	// 	// 	for _, qq := range q.
	// 	// 	// }

	// 	// 	// log.Printf("\n Root Query:  %+v \n", q.Name)
	// 	// 	// for _, pkgMethod := range pkgConfig.Methods {

	// 	// 	// }
	// }

	log.Print("\n **************************** \n\n")

	for _, field := range s.MutationType.Fields {
		for _, pkgMethod := range pkgConfig.Methods {

			if field.Name == pkgMethod.Name {

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
