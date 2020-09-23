package lang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"

	log "github.com/sirupsen/logrus"
)

type CommandGenerator struct {
	PackageName string
	Imports     []string
	Commands    []Command
}

type Command struct {
	Name             string
	ShortDescription string
	LongDescription  string
	Example          string
	InputType        string
	ClientMethod     string
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
	Required       bool
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
	Queries     []GoMethod
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
	// ReturnSlice indicates if the response is a slice of objects or not.  Used to flag KindList.
	ReturnSlice bool
	// Return path is the fields on the response object that nest the restuls the given method will return.
	ReturnPath []string
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

// GenerateGoMethodQueriesForPackage uses the provided configuration to generate the GoMethod structs that contain the information about performing GraphQL queries.
func GenerateGoMethodQueriesForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) (*[]GoMethod, error) {
	var methods []GoMethod

	for _, pkgQuery := range pkgConfig.Queries {

		typePath, err := s.LookupQueryTypesByFieldPath(pkgQuery.Path)
		if err != nil {
			log.Error(err)
			continue
		}

		// TODO this will eventually break when a field name of the struct is not a
		// simple capitalization.  We'd need to loop over the fields for the type
		// and grab the name as is done in constrainedResponseStructs().
		returnPath := []string{}
		for _, t := range pkgQuery.Path {
			returnPath = append(returnPath, strings.Title(t))
		}

		for _, endpointName := range pkgQuery.Endpoints {

			t := typePath[len(typePath)-1]

			for _, field := range t.Fields {
				if field.Name == endpointName {
					method := GoMethod{
						Name:        field.GetName(),
						Description: field.GetDescription(),
					}

					log.Infof("Kinds: %+v", field.Type.GetKinds())

					var prefix string
					kinds := field.Type.GetKinds()
					if kinds[0] == schema.KindList {
						prefix = "[]"
						method.Signature.ReturnSlice = true
					}

					pointerReturn := fmt.Sprintf("%s%s", prefix, field.Type.GetTypeName())
					method.Signature.Return = []string{pointerReturn, "error"}
					method.QueryString = s.GetQueryStringForEndpoint(t.Name, endpointName)
					method.Signature.ReturnPath = returnPath

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

						queryVar := QueryVar{
							Key:   methodArg.Name,
							Value: inputType.Name,
							Type:  methodArg.Type.GetTypeName(),
						}

						method.QueryVars = append(method.QueryVars, queryVar)

						method.Signature.Input = append(method.Signature.Input, inputType)
					}

					methods = append(methods, method)

				}

			}

		}

	}

	return &methods, nil
}

// GenerateGoMethodMutationsForPackage uses the provided configuration to generate the GoMethod structs that contain the information about performing GraphQL mutations.
func GenerateGoMethodMutationsForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) (*[]GoMethod, error) {
	var methods []GoMethod

	if len(pkgConfig.Mutations) == 0 {
		return nil, nil
	}

	for _, field := range s.MutationType.Fields {
		for _, pkgMutation := range pkgConfig.Mutations {

			if field.Name == pkgMutation.Name {

				method := GoMethod{
					Name:        field.GetName(),
					Description: field.GetDescription(),
				}

				pointerReturn := field.Type.GetTypeName()
				method.Signature.Return = []string{pointerReturn, "error"}
				method.QueryString = schema.PrefixLineTab(schema.PrefixLineTab(s.QueryFieldsForTypeName(field.Type.Name)))

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
							Type:  methodArg.Type.GetTypeName(),
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
				xxx.Fields = append(xxx.Fields, getStructField(f, pkgConfig))
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

	structsForGen = append(structsForGen, constrainedResponseStructs(s, pkgConfig, expandedTypes)...)

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

func getStructField(f schema.Field, pkgConfig *config.PackageConfig) GoStructField {
	var typeName string
	var typeNamePrefix string
	var typeNameSuffix string
	var err error

	typeName, err = f.GetTypeNameWithOverride(pkgConfig)
	if err != nil {
		log.Error(err)
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

	return GoStructField{
		Description: f.GetDescription(),
		Name:        f.GetName(),
		Tags:        f.GetTags(),
		Type:        fmt.Sprintf("%s%s%s", typeNamePrefix, typeName, typeNameSuffix),
	}
}

// constrainedResponseStructs is used to create response objects that contain
// fields that already exist in the expandedTypes.  This avoids creating full
// structs, and limits response objects to those types that are already
// referenced in the expandedTypes.
func constrainedResponseStructs(s *schema.Schema, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) []GoStruct {
	var goStructs []GoStruct

	isExpanded := func(expandedTypes *[]*schema.Type, typeName string) bool {
		for _, t := range *expandedTypes {
			if t.GetName() == typeName {
				return true
			}
		}

		return false
	}

	isInPath := func(types []*schema.Type, typeName string) bool {
		for _, t := range types {
			if t.GetName() == typeName {
				return true
			}
		}

		return false
	}

	for _, query := range pkgConfig.Queries {
		pathTypes, err := s.LookupQueryTypesByFieldPath(query.Path)
		if err != nil {
			log.Error(err)
			continue
		}

		// Ensure that all of the types that we will depend on in our response struct below are present.
		for _, t := range pathTypes {
			// Skip doing anything with this type if it has already been expanded.
			if isExpanded(expandedTypes, t.GetName()) {
				continue
			}

			xxx := GoStruct{
				Name:        t.GetName(),
				Description: t.GetDescription(),
			}

			for _, f := range t.Fields {
				if isExpanded(expandedTypes, f.Type.GetTypeName()) || isInPath(pathTypes, f.Type.GetName()) {
					xxx.Fields = append(xxx.Fields, getStructField(f, pkgConfig))
				}
			}

			goStructs = append(goStructs, xxx)
		}

		// Ensure we have a response struct for each of the endpoints in our config.
		for _, endpoint := range query.Endpoints {
			xxx := GoStruct{
				Name: fmt.Sprintf("%sResponse", endpoint),
			}

			firstType := pathTypes[0]

			field := GoStructField{
				Name: firstType.Name,
				Type: firstType.Name,
			}

			xxx.Fields = append(xxx.Fields, field)
		}

	}

	return goStructs
}
