package lang

import (
	"fmt"
	"sort"
	"strings"

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
	Mutations   []GoMethod
	Queries     []GoMethod
}

type GoStruct struct {
	Name             string
	Description      string
	Fields           []GoStructField
	Implements       []string
	SpecialUnmarshal bool
	GenerateGetters  bool
}

type GoStructField struct {
	Name        string
	Type        string
	TypeName    string
	Tags        string
	TagKey      string
	Description string
	IsInterface bool
	IsList      bool
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
	Name          string
	Description   string
	Type          string
	PossibleTypes []GoInterfacePossibleType
	Methods       []string
}

type GoInterfacePossibleType struct {
	GoName      string
	GraphQLName string
}

type GoMethod struct {
	Description string
	Name        string
	QueryVars   []QueryVar
	Signature   GoMethodSignature
	QueryString string
	// ResponseObjectType is the name of the type for the API response.  Note that this is not the method return, but the API call response.
	ResponseObjectType string
}

type GoMethodSignature struct {
	Input  []GoMethodInputType
	Return []string
	// ReturnSlice indicates if the response is a slice of objects or not.  Used to flag KindList.
	ReturnSlice bool
	// Return path is the fields on the response object that nest the results the given method will return.
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

		returnPath, err := s.LookupQueryFieldsByFieldPath(pkgQuery.Path)
		if err != nil {
			return nil, err
		}

		inputFields := s.GetInputFieldsForQueryPath(pkgQuery.Path)

		// The endpoint we care about will always be on the last of the path elements specified.
		t := typePath[len(typePath)-1]

		// Find the intersection of the received endpoint and the field name on the last type in the path.
		// For example, given the following path...
		// actor { cloud { } }
		// ... we want to generate the method based on the Type of the field 'cloud'.
		for _, endpoint := range pkgQuery.Endpoints {
			for _, field := range t.Fields {
				if field.Name == endpoint.Name {
					method := goMethodForField(field, pkgConfig, inputFields)

					method.QueryString = s.GetQueryStringForEndpoint(typePath, pkgQuery.Path, endpoint.Name, endpoint.MaxQueryFieldDepth, endpoint.IncludeArguments)
					method.ResponseObjectType = fmt.Sprintf("%sResponse", endpoint.Name)
					method.Signature.ReturnPath = returnPath

					methods = append(methods, method)
				}
			}
		}
	}

	if len(methods) > 0 {
		sort.SliceStable(methods, func(i, j int) bool {
			return methods[i].Name < methods[j].Name
		})
		return &methods, nil
	}

	return &methods, nil
}

// GenerateGoMethodMutationsForPackage uses the provided configuration to generate the GoMethod structs that contain the information about performing GraphQL mutations.
func GenerateGoMethodMutationsForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) (*[]GoMethod, error) {
	var methods []GoMethod

	if len(pkgConfig.Mutations) == 0 {
		return nil, nil
	}

	// for _, field := range s.MutationType.Fields {
	for _, pkgMutation := range pkgConfig.Mutations {
		field, err := s.LookupMutationByName(pkgMutation.Name)
		if err != nil {
			log.Error(err)
			continue
		}

		if field == nil {
			log.Errorf("unable to generate mutation from nil field, %s", pkgMutation.Name)
			continue
		}

		method := goMethodForField(*field, pkgConfig, nil)
		method.QueryString = s.GetQueryStringForMutation(field, pkgMutation.MaxQueryFieldDepth, pkgMutation.ArgumentTypeOverrides)

		methods = append(methods, method)
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

	configNames := make(map[string]config.TypeConfig, len(pkgConfig.Types))

	// pivot the data
	for _, p := range pkgConfig.Types {
		configNames[strings.ToLower(p.Name)] = p
	}

	for _, t := range *expandedTypes {
		var interfaceMethods []string
		var generateGetters bool
		// Default scalars to string
		createAs := "string"

		if p, ok := configNames[strings.ToLower(t.GetName())]; ok {
			log.WithFields(log.Fields{
				"create_as":               p.CreateAs,
				"field_type_override":     p.FieldTypeOverride,
				"generate_struct_getters": p.GenerateStructGetters,
				"kind":                    t.Kind,
				"name":                    p.Name,
				"skip_type_create":        p.SkipTypeCreate,
			}).Debug("found type config")

			if p.SkipTypeCreate {
				log.WithFields(log.Fields{
					"name": p.Name,
				}).Debug("skipping")
				continue
			}

			generateGetters = p.GenerateStructGetters

			if p.CreateAs != "" {
				createAs = p.CreateAs
			}

			if len(p.InterfaceMethods) > 0 {
				interfaceMethods = p.InterfaceMethods
			}
		}

		switch t.Kind {
		case schema.KindInputObject, schema.KindObject, schema.KindInterface:
			xxx := GoStruct{
				Name:            t.GetName(),
				Description:     t.GetDescription(),
				GenerateGetters: generateGetters,
			}

			var fields []schema.Field
			fields = append(fields, t.Fields...)
			fields = append(fields, t.InputFields...)

			fieldErrs := []error{}
			for _, f := range fields {
				// If any of the fields for this type are an interface type, then we
				// need to signal to the template an UnmarshalJSON() should be
				// rendered.
				if f.Type.IsInterface() {
					xxx.SpecialUnmarshal = true
				}

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

				// Inform the template about which possible implementations exist for
				// this interface.  We need to know about both the name that GraphQL
				// uses and the name that Go uses.  This is to allow some flexibility
				// in the template for how to reference the implementation information.
				for _, x := range t.PossibleTypes {
					ttt := GoInterfacePossibleType{
						GraphQLName: x.Name,
						GoName:      x.GetName(),
					}

					yyy.PossibleTypes = append(yyy.PossibleTypes, ttt)
				}

				// Require at least one auto-generated method, allow others
				yyy.Methods = make([]string, 0, len(interfaceMethods)+1)
				yyy.Methods = append(yyy.Methods, "Implements"+t.GetName()+"()")
				yyy.Methods = append(yyy.Methods, interfaceMethods...)

				interfacesForGen = append(interfacesForGen, yyy)
			}

			sort.SliceStable(xxx.Fields, func(i, j int) bool {
				return xxx.Fields[i].Name < xxx.Fields[j].Name
			})

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
			// Alpha sort the ENUMs
			sort.SliceStable(xxx.Values, func(i, j int) bool {
				return xxx.Values[i].Name < xxx.Values[j].Name
			})

			enumsForGen = append(enumsForGen, xxx)
		case schema.KindScalar:
			if !t.IsGoType() {
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
	var isList bool
	var err error

	typeName, err = f.GetTypeNameWithOverride(pkgConfig)
	if err != nil {
		log.Error(err)
	}

	// In the case we have a LIST type, we need to prefix the type with the slice
	// descriptor.
	if f.Type.IsList() {
		typeNamePrefix = "[]"
		isList = true
	}

	// Used to signal the template that the UnmarshalJSON should handle this field as an Interface.
	var isInterface bool

	// In the case a field type is of type Interface, we need to ensure we
	// append the term "Interface" to it, as is done in the "Implements"
	// below.
	if f.Type.IsInterface() {
		typeNameSuffix = "Interface"
		isInterface = true
	}

	return GoStructField{
		Description: f.GetDescription(),
		Name:        f.GetName(),
		TagKey:      f.Name,
		Tags:        f.GetTags(),
		IsInterface: isInterface,
		IsList:      isList,
		Type:        fmt.Sprintf("%s%s%s", typeNamePrefix, typeName, typeNameSuffix),
		TypeName:    typeName,
	}
}

// constrainedResponseStructs is used to create response objects that contain
// fields that already exist in the expandedTypes.  This avoids creating full
// structs, and limits response objects to those types that are already
// referenced in the expandedTypes.
func constrainedResponseStructs(s *schema.Schema, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) []GoStruct {
	var goStructs []GoStruct

	// Determine if the typeName received exists in the received expandedTypes list.
	isExpanded := func(expandedTypes *[]*schema.Type, typeName string) bool {
		for _, t := range *expandedTypes {
			if t.GetName() == typeName {
				return true
			}
		}

		return false
	}

	// Determine if the received typeName is in the  received schema.Type list.
	isInPath := func(types []*schema.Type, typeName string) bool {
		for _, t := range types {
			if t.GetName() == typeName {
				return true
			}
		}

		return false
	}

	// Build a response object for each one of the queries in the configuration.
	for _, query := range pkgConfig.Queries {
		// Retrieve the corresponding types for each of the field names in the query config.
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
				Name: fmt.Sprintf("%sResponse", endpoint.Name),
			}

			// For the top level response object, we only use the first field path
			// that is received from the user.
			firstType := pathTypes[0]

			field := GoStructField{
				Name: firstType.GetName(),
				Type: firstType.GetName(),
				Tags: fmt.Sprintf("`json:\"%s\"`", query.Path[0]),
			}

			xxx.Fields = append(xxx.Fields, field)
			goStructs = append(goStructs, xxx)
		}
	}

	return goStructs
}

// goMethodForField creates a new GoMethod based on a field.  Note that the
// implementation specific information like QueryString are not added to the
// method, and it is up to the caller to flavor the method accordingly.
//
// The received inputFields are to seed the initial objet.  This allows the
// caller to pass additional context about the received field's place in the
// schema that are used as a starting place for input variables, since the
// parent objects may require those inputs.
func goMethodForField(field schema.Field, pkgConfig *config.PackageConfig, inputFields map[string][]schema.Field) GoMethod {
	method := GoMethod{
		Name:        field.GetName(),
		Description: field.GetDescription(),
	}

	for pathName, fields := range inputFields {
		for _, f := range fields {
			if f.Type.IsNonNull() {
				typeName, err := f.GetTypeNameWithOverride(pkgConfig)
				if err != nil {
					log.Error(err)
					continue
				}

				var prefix string
				if f.Type.IsList() {
					prefix = "[]"
				}

				inputType := GoMethodInputType{
					// Flavor the name of the input object with the field from the path
					// in which we were found.
					Name: fmt.Sprintf("%s%s", pathName, f.GetName()),
					Type: fmt.Sprintf("%s%s", prefix, typeName),
				}

				method.Signature.Input = append(method.Signature.Input, inputType)

				queryVar := QueryVar{
					Key:   inputType.Name,
					Value: inputType.Name,
					Type:  inputType.Type,
				}

				method.QueryVars = append(method.QueryVars, queryVar)
			}
		}
	}

	var prefix, suffix string

	if field.Type.IsList() {
		prefix = "[]"
		method.Signature.ReturnSlice = true
	}

	if field.Type.IsInterface() {
		suffix = "Interface"
	}

	returnTypeName, err := field.GetTypeNameWithOverride(pkgConfig)
	if err != nil {
		log.Error(err)
		returnTypeName = field.Type.GetName()
	}
	method.Signature.Return = []string{
		prefix + returnTypeName + suffix,
		"error",
	}

	for _, methodArg := range field.Args {
		typeName, err := methodArg.GetTypeNameWithOverride(pkgConfig)
		if err != nil {
			log.Error(err)
			continue
		}

		var methodArgPrefix string
		if methodArg.Type.IsList() {
			methodArgPrefix = "[]"
		}

		inputType := GoMethodInputType{
			Name: methodArg.GetName(),
			Type: fmt.Sprintf("%s%s", methodArgPrefix, typeName),
		}

		method.Signature.Input = append(method.Signature.Input, inputType)

		queryVar := QueryVar{
			Key:   methodArg.Name,
			Value: inputType.Name,
			Type:  methodArg.Type.GetTypeName(),
		}

		method.QueryVars = append(method.QueryVars, queryVar)
	}

	return method
}
