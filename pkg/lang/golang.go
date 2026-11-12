package lang

import (
	"fmt"
	"go/token"
	"regexp"
	"sort"
	"strings"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/internal/util"

	log "github.com/sirupsen/logrus"
)

// GolangGenerator is enough information to generate Go code for a single package.
type GolangGenerator struct {
	Types       []GoStruct
	PackageName string
	// FileName, GoPackageName and ClientTypeName are the file stem, the `package`
	// identifier and the client receiver type. They are separate from PackageName
	// because one config name cannot be right for all three.
	FileName       string
	GoPackageName  string
	ClientTypeName string
	Enums          []GoEnum
	Imports        []string
	Scalars        []GoScalar
	Interfaces     []GoInterface
	Mutations      []GoMethod
	Queries        []GoMethod
	// KeepExisting names what an existing file keeps as it is for the types set to
	// keep_existing (see KeptNames).
	KeepExisting []string
	// SkippedFields maps the Go name of a type to the Go names of the fields the
	// package config skips for it (see SkippedFields).
	SkippedFields map[string][]string
	// NarrowedInterfaces are the Go names of the interfaces the package narrows to
	// some implementations (see NarrowedInterfaces).
	NarrowedInterfaces []string
}

// SetPackageNaming fills the name fields from the package config.
func (g *GolangGenerator) SetPackageNaming(pkgConfig *config.PackageConfig) {
	g.PackageName = pkgConfig.Name
	g.FileName = pkgConfig.GetFileName()
	g.GoPackageName = pkgConfig.GetGoPackageName()
	g.ClientTypeName = pkgConfig.GetClientTypeName()
}

type GoStruct struct {
	Name             string
	Description      string
	Fields           []GoStructField
	StructTags       []string
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
	// FieldName is the Go name of the GraphQL field the method reads from the
	// response. It is Name unless MethodName was overridden.
	FieldName string
	// MethodName is the complete Go name of a query method, e.g. "GetList".
	MethodName string
	// ContextMethodName is the Go name of the variant that takes a context.Context,
	// e.g. "GetListWithContext", for templates that generate one.
	ContextMethodName string
	// QueryConstName is the name of the constant holding the query string.
	QueryConstName string
	QueryVars      []QueryVar
	Signature      GoMethodSignature
	QueryString    string
	// ResponseObjectType is the name of the type for the API response.  Note that this is not the method return, but the API call response.
	ResponseObjectType string
	// ReturnAlias is the identifier that stands in for the return type in Signature.Return
	// when the type is not itself an exported identifier, e.g. `[]string`, `bool` or
	// `nrtime.EpochSeconds`. Templates print that entry as a field name as well as a type,
	// which only an identifier survives. It is replaced by ReturnType once the method is
	// rendered, and is empty when the return type is an identifier.
	ReturnAlias string
	// ReturnType is the Go type ReturnAlias stands for.
	ReturnType string
	// UnsetParams are the parameters of a method generated with
	// include_optional_arguments whose variable is left out of the request when the
	// caller passes nil, so the API applies its default.
	UnsetParams []UnsetParam
}

// UnsetParam is a parameter that may be nil and the key of the request variable it fills.
type UnsetParam struct {
	// Param is the Go name of the parameter as the template prints it.
	Param string
	// Key is the name of the GraphQL variable.
	Key string
}

// MutationResultField is the name of the field that holds the result in the response
// struct of a mutation whose return type is a ReturnAlias.
const MutationResultField = "Result"

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
			// Kept as it is, so it also takes no part in the duplicate check below.
			if endpoint.KeepExisting {
				continue
			}

			for _, field := range t.Fields {
				if field.Name == endpoint.Name {
					if err := checkEndpointArguments(endpoint, field); err != nil {
						return nil, err
					}

					method := goMethodForField(field, pkgConfig, inputFields, methodArgOptions{
						Exclude:         endpoint.ExcludeArguments,
						GoTypes:         endpoint.ArgumentGoTypes,
						IncludeOptional: endpoint.IncludeOptionalArguments,
					})

					if err := checkUniqueParameters(method); err != nil {
						return nil, err
					}

					method.QueryString = s.GetQueryStringForEndpoint(typePath, pkgQuery.Path, endpoint)
					method.ResponseObjectType = endpointResponseName(endpoint)
					method.Signature.ReturnPath = returnPath

					method.FieldName = method.Name
					method.MethodName = "Get" + UpperFirst(method.Name)
					method.QueryConstName = "get" + method.Name + "Query"

					if endpoint.MethodName != "" {
						method.MethodName = endpoint.MethodName
						method.QueryConstName = lowerFirst(endpoint.MethodName) + "Query"
					}

					method.ContextMethodName = method.MethodName + "WithContext"
					if endpoint.ContextMethodName != "" {
						method.ContextMethodName = endpoint.ContextMethodName
					}

					methods = append(methods, method)
				}
			}
		}
	}

	if err := checkDuplicateMethodNames(pkgConfig.Name, methods, func(m GoMethod) string { return m.MethodName }); err != nil {
		return nil, err
	}

	sort.SliceStable(methods, func(i, j int) bool {
		return methods[i].MethodName < methods[j].MethodName
	})

	return &methods, nil
}

// checkEndpointArguments fails when exclude_arguments names something the
// endpoint does not take, or leaves out an argument the API requires.
func checkEndpointArguments(endpoint config.EndpointConfig, field schema.Field) error {
	for _, name := range endpoint.ExcludeArguments {
		found := false

		for _, a := range field.Args {
			if a.Name != name {
				continue
			}

			found = true

			if a.IsRequired() {
				return fmt.Errorf("endpoint %q: exclude_arguments names %q, which the API requires", endpoint.Name, name)
			}
		}

		if !found {
			return fmt.Errorf("endpoint %q: exclude_arguments names %q, which is not an argument of the endpoint", endpoint.Name, name)
		}
	}

	for name := range endpoint.ArgumentGoTypes {
		found := false

		for _, a := range field.Args {
			if a.Name == name {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("endpoint %q: argument_go_types names %q, which is not an argument of the endpoint", endpoint.Name, name)
		}
	}

	return nil
}

// endpointResponseName is the name of the struct that wraps an endpoint's response.
// An endpoint with its own method name gets its own response struct too, so two
// endpoints that share a GraphQL name under different paths do not share one.
func endpointResponseName(e config.EndpointConfig) string {
	if e.MethodName != "" {
		return lowerFirst(e.MethodName) + "Response"
	}

	return fmt.Sprintf("%sResponse", e.Name)
}

// checkDuplicateMethodNames fails when two methods would get the same Go name,
// which would not compile. The usual cause is one endpoint name under two query
// paths, and the fix is method_name on one of them.
func checkDuplicateMethodNames(pkg string, methods []GoMethod, name func(GoMethod) string) error {
	seen := map[string]bool{}

	for _, m := range methods {
		n := name(m)
		if seen[n] {
			return fmt.Errorf("package %q: method %q is generated more than once; give the endpoints distinct names with method_name", pkg, n)
		}

		seen[n] = true
	}

	return nil
}

// UpperFirst capitalizes the first letter, as the templates' title function does.
func UpperFirst(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}

	return strings.ToLower(s[:1]) + s[1:]
}

// GenerateGoMethodMutationsForPackage uses the provided configuration to generate the GoMethod structs that contain the information about performing GraphQL mutations.
func GenerateGoMethodMutationsForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) (*[]GoMethod, error) {
	if !hasGeneratedMutation(pkgConfig.Mutations) {
		return nil, nil
	}

	mutations, err := generatedMutations(s, pkgConfig)
	if err != nil {
		return nil, err
	}

	if len(mutations) > 0 {
		methods := make([]GoMethod, 0, len(mutations))
		for _, m := range mutations {
			methods = append(methods, m.method)
		}

		sort.SliceStable(methods, func(i, j int) bool {
			return methods[i].Name < methods[j].Name
		})

		if err := checkDuplicateMethodNames(pkgConfig.Name, methods, func(m GoMethod) string { return m.Name }); err != nil {
			return nil, err
		}

		return &methods, nil
	}

	return nil, fmt.Errorf("no methods for package")
}

// generatedMutation is a mutation the package config generates: the schema field it
// calls and the method that calls it.
type generatedMutation struct {
	field  schema.Field
	method GoMethod
}

// generatedMutations makes a method for every mutation the package config generates, in
// config order.
func generatedMutations(s *schema.Schema, pkgConfig *config.PackageConfig) ([]generatedMutation, error) {
	var mutations []generatedMutation

	for _, pkgMutation := range pkgConfig.Mutations {
		// Kept as it is, so it also takes no part in the duplicate check by the caller.
		if pkgMutation.KeepExisting {
			continue
		}

		fields := s.LookupMutationsByPattern(pkgMutation.Name)

		if err := checkArgumentConfig(pkgMutation, fields); err != nil {
			return nil, err
		}

		for _, field := range fields {
			method := goMethodForField(field, pkgConfig, nil, methodArgOptions{
				Exclude: pkgMutation.ExcludeArguments,
				Rename:  pkgMutation.RenameArguments,
				GoTypes: pkgMutation.ArgumentGoTypes,

				IncludeOptional: pkgMutation.IncludeOptionalArguments,
			})
			method.QueryString = s.GetQueryStringForMutation(&field, pkgMutation)
			setReturnAlias(&method)

			if err := checkUniqueParameters(method); err != nil {
				return nil, err
			}

			mutations = append(mutations, generatedMutation{field: field, method: method})
		}
	}

	return mutations, nil
}

// setReturnAlias gives a method whose return type is not an exported identifier an
// identifier to be rendered with, which the rendered method gets the type back for.
// The templates use the return type as the name of the response's field too, so a
// type such as `[]string` would otherwise render `resp.[]string`.
func setReturnAlias(m *GoMethod) {
	returnType := m.Signature.Return[0]
	if token.IsIdentifier(returnType) && token.IsExported(returnType) {
		return
	}

	m.ReturnType = returnType
	m.ReturnAlias = "tutoneResult" + m.Name
	m.Signature.Return[0] = m.ReturnAlias
}

// hasGeneratedMutation reports whether any mutation is left for the generator, as
// opposed to none configured or all kept as they are.
func hasGeneratedMutation(mutations []config.MutationConfig) bool {
	for _, m := range mutations {
		if !m.KeepExisting {
			return true
		}
	}

	return false
}

// checkArgumentConfig fails when exclude_arguments or rename_arguments names an
// argument that none of the matched mutations has, so a typo cannot silently do
// nothing.
func checkArgumentConfig(cfg config.MutationConfig, mutations []schema.Field) error {
	has := func(name string) bool {
		for _, m := range mutations {
			for _, a := range m.Args {
				if a.Name == name {
					return true
				}
			}
		}

		return false
	}

	for _, name := range cfg.ExcludeArguments {
		if !has(name) {
			return fmt.Errorf("mutation %q: exclude_arguments names %q, which is not an argument of any matching mutation", cfg.Name, name)
		}
	}

	for name := range cfg.RenameArguments {
		if !has(name) {
			return fmt.Errorf("mutation %q: rename_arguments names %q, which is not an argument of any matching mutation", cfg.Name, name)
		}
	}

	for name := range cfg.ArgumentGoTypes {
		if !has(name) {
			return fmt.Errorf("mutation %q: argument_go_types names %q, which is not an argument of any matching mutation", cfg.Name, name)
		}
	}

	return nil
}

// KeptNames returns the names an existing file keeps as they are for the expanded
// types that the package config sets to keep_existing: the Go name of each, which
// the merge also takes for its methods, and the other declarations the typegen
// template makes for it. That is the <Name>Types variable of an enum, and the
// <Name>Interface type and Unmarshal<Name>Interface function of an interface or a
// union. A type is matched to its config the way its other settings are, ignoring case.
func KeptNames(pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) []string {
	kept := map[string]bool{}

	for _, t := range pkgConfig.Types {
		if t.KeepExisting {
			kept[strings.ToLower(t.Name)] = true
		}
	}

	if len(kept) == 0 || expandedTypes == nil {
		return nil
	}

	var names []string

	for _, t := range *expandedTypes {
		name := t.GetName()
		if !kept[strings.ToLower(name)] {
			continue
		}

		names = append(names, name)

		switch t.Kind {
		case schema.KindENUM:
			names = append(names, name+"Types")
		case schema.KindInterface, schema.KindUnion:
			names = append(names, name+"Interface", "Unmarshal"+name+"Interface")
		}
	}

	return names
}

// NarrowedInterfaces returns the Go names of the expanded interfaces the package config
// narrows with include_implementations on a type.
func NarrowedInterfaces(pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) []string {
	if expandedTypes == nil {
		return nil
	}

	var names []string

	for _, t := range *expandedTypes {
		if t.Kind != schema.KindInterface {
			continue
		}

		if c := getTypeConfig(t.Name, pkgConfig.Types); c != nil && len(c.IncludeImplementations) > 0 {
			names = append(names, t.GetName())
		}
	}

	sort.Strings(names)

	return names
}

// SkippedFields returns, for each expanded type the package config gives
// skip_fields, the Go type name and the Go names of the fields it skips. Regenerating
// does not keep these fields: they are left out on purpose, not removed from the schema.
func SkippedFields(pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) map[string][]string {
	if expandedTypes == nil {
		return nil
	}

	skipped := map[string][]string{}

	for _, t := range *expandedTypes {
		if c := getTypeConfig(t.Name, pkgConfig.Types); c != nil && len(c.SkipFields) > 0 {
			skipped[t.GetName()] = c.SkipFields
		}
	}

	return skipped
}

func GenerateGoTypesForPackage(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) (*[]GoStruct, *[]GoEnum, *[]GoScalar, *[]GoInterface, error) {
	// TODO: Putting the types in the specified path should be optional
	//       Should we use a flag or allow the user to omit that field in the config? ¿Por que no lost dos?

	var structsForGen []GoStruct
	var enumsForGen []GoEnum
	var scalarsForGen []GoScalar
	var interfacesForGen []GoInterface
	var unionsForGen []GoInterface

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

			c := getTypeConfig(t.Name, pkgConfig.Types)

			if err := checkFieldConfigs(t.Name, c, fields); err != nil {
				return nil, nil, nil, nil, err
			}

			fieldErrs := []error{}
			for _, f := range fields {
				// Skip field if specified in the config
				if c != nil && util.StringInStrings(f.GetName(), c.SkipFields) {
					log.WithFields(log.Fields{
						"name": f.GetName(),
					}).Debug("skipping field")
					continue
				}

				xxx.Fields = append(xxx.Fields, getStructField(f, pkgConfig, t))
			}

			xxx.SpecialUnmarshal = hasInterfaceField(xxx.Fields)

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

				interfacesForGen = append(interfacesForGen, goInterfaceForType(t, interfaceMethods))
			}

			sort.SliceStable(xxx.Fields, func(i, j int) bool {
				return xxx.Fields[i].Name < xxx.Fields[j].Name
			})

			structsForGen = append(structsForGen, xxx)
		case schema.KindUnion:
			if !unionHasGoType(getTypeConfig(t.Name, pkgConfig.Types)) {
				unionsForGen = append(unionsForGen, goInterfaceForType(t, interfaceMethods))
			}
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

	interfacesForGen = append(interfacesForGen, implementUnions(structsForGen, unionsForGen)...)

	constrained, err := constrainedResponseStructs(s, pkgConfig, expandedTypes)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	responses, err := argumentlessMutationResponseStructs(s, pkgConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	setSpecialUnmarshal(constrained, interfacesForGen)
	setSpecialUnmarshal(responses, interfacesForGen)

	structsForGen = append(structsForGen, constrained...)
	structsForGen = append(structsForGen, responses...)

	// The same struct can come from more than one query path (e.g. Actor from
	// actor.cloud and actor.account.cloud). Declaring it twice does not compile.
	structsForGen, err = mergeStructsByName(structsForGen)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	handWritten, err := codegen.ScanHandWritten(pkgConfig.GetDestinationPath())
	if err != nil {
		return nil, nil, nil, nil, err
	}

	omitDeclaredUnmarshalers(structsForGen, handWritten.Methods)

	sort.SliceStable(structsForGen, func(i, j int) bool {
		return structsForGen[i].Name < structsForGen[j].Name
	})

	breakRecursiveTypes(structsForGen)

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

// goInterfaceForType describes the Go interface generated for a GraphQL interface
// or union: the template needs both names of each possible type, the one GraphQL
// uses and the one Go uses, so it can pick the type by __typename.
func goInterfaceForType(t *schema.Type, interfaceMethods []string) GoInterface {
	goInterface := GoInterface{
		Description: t.GetDescription(),
		Name:        t.GetName(),
	}

	for _, x := range t.PossibleTypes {
		goInterface.PossibleTypes = append(goInterface.PossibleTypes, GoInterfacePossibleType{
			GraphQLName: x.Name,
			GoName:      x.GetName(),
		})
	}

	// Require at least one auto-generated method, allow others
	goInterface.Methods = make([]string, 0, len(interfaceMethods)+1)
	goInterface.Methods = append(goInterface.Methods, "Implements"+t.GetName()+"()")
	goInterface.Methods = append(goInterface.Methods, interfaceMethods...)

	return goInterface
}

// isGoInterface reports whether the Go type of a field is the <Name>Interface
// generated for its GraphQL type, so the field is unmarshalled by __typename. That
// is every interface, and every union the package generates. A union the config
// names a Go type for, with field_type_override or skip_type_create, is that type.
func isGoInterface(f schema.Field, pkgConfig *config.PackageConfig) bool {
	if f.Type.IsInterface() {
		return true
	}

	if !f.Type.IsUnion() {
		return false
	}

	return !unionHasGoType(getTypeConfig(f.Type.GetTypeName(), pkgConfig.Types))
}

// unionHasGoType reports whether the config names the Go type of a union, so none
// is generated: field_type_override gives it and skip_type_create says it exists.
func unionHasGoType(c *config.TypeConfig) bool {
	return c != nil && (c.FieldTypeOverride != "" || c.SkipTypeCreate)
}

// hasInterfaceField reports whether a struct needs the UnmarshalJSON that picks
// the Go type of its interface fields.
func hasInterfaceField(fields []GoStructField) bool {
	for _, f := range fields {
		if f.IsInterface {
			return true
		}
	}

	return false
}

// unmarshalsInterfaceFields reports whether a struct gets an UnmarshalJSON for its
// interface fields. It does when it has some and the package generates the
// Unmarshal<Name>Interface function for each of them. A package may instead supply
// the Go type of an interface by hand, and no generated function can decode into it.
func unmarshalsInterfaceFields(fields []GoStructField, generated []GoInterface) bool {
	if !hasInterfaceField(fields) {
		return false
	}

	names := make(map[string]bool, len(generated))
	for _, g := range generated {
		names[g.Name] = true
	}

	for _, f := range fields {
		if f.IsInterface && !names[f.TypeName] {
			return false
		}
	}

	return true
}

// setSpecialUnmarshal gives the structs that unmarshalsInterfaceFields accepts an
// UnmarshalJSON for their interface fields.
func setSpecialUnmarshal(structs []GoStruct, generated []GoInterface) {
	for i := range structs {
		structs[i].SpecialUnmarshal = unmarshalsInterfaceFields(structs[i].Fields, generated)
	}
}

// omitDeclaredUnmarshalers clears SpecialUnmarshal on the structs whose UnmarshalJSON
// a hand-written file of the package already declares, so it is not declared twice.
func omitDeclaredUnmarshalers(structs []GoStruct, handWritten map[string]bool) {
	for i := range structs {
		if structs[i].SpecialUnmarshal && handWritten[structs[i].Name+".UnmarshalJSON"] {
			log.WithFields(log.Fields{
				"struct": structs[i].Name,
			}).Info("UnmarshalJSON is declared by hand in the package, so it is not generated")

			structs[i].SpecialUnmarshal = false
		}
	}
}

// implementUnions makes the generated members of each union implement its Go
// interface, and returns the unions with those members only. A member that is not
// generated, because the config skips it, is dropped: the interface's Unmarshal
// function could not name it.
func implementUnions(structs []GoStruct, unions []GoInterface) []GoInterface {
	index := make(map[string]int, len(structs))
	for i, st := range structs {
		index[st.Name] = i
	}

	for u := range unions {
		var members []GoInterfacePossibleType

		for _, m := range unions[u].PossibleTypes {
			i, ok := index[m.GoName]
			if !ok {
				log.WithFields(log.Fields{
					"union":  unions[u].Name,
					"member": m.GoName,
				}).Warn("union member is not generated, so it is left out of the union")

				continue
			}

			if !util.StringInStrings(unions[u].Name, structs[i].Implements) {
				structs[i].Implements = append(structs[i].Implements, unions[u].Name)
			}

			members = append(members, m)
		}

		unions[u].PossibleTypes = members
	}

	return unions
}

// argumentlessMutationResponseStructs makes the struct that holds the response of each
// generated mutation that sends no variables. The client template declares that struct
// only for a mutation with variables, but every method decodes into it. These structs
// decode a result that is an interface or a union by __typename; the one the template
// declares has no UnmarshalJSON, so it cannot.
func argumentlessMutationResponseStructs(s *schema.Schema, pkgConfig *config.PackageConfig) ([]GoStruct, error) {
	if !pkgConfig.GeneratesClient() || !hasGeneratedMutation(pkgConfig.Mutations) {
		return nil, nil
	}

	mutations, err := generatedMutations(s, pkgConfig)
	if err != nil {
		return nil, err
	}

	var structs []GoStruct

	for _, m := range mutations {
		if len(m.method.QueryVars) > 0 {
			continue
		}

		structs = append(structs, GoStruct{Name: m.method.Name + "QueryResponse", Fields: []GoStructField{mutationResultField(m, pkgConfig)}})
	}

	return structs, nil
}

// mutationResultField is the field of a mutation's response struct that holds its
// result. The method reads the field named after its return type, or the result field
// when the return type was given an alias. TagKey is the key of the result in the
// response, and the other settings are those of any struct field of the result's type,
// so UnmarshalJSON can decode an interface or a union by __typename.
func mutationResultField(m generatedMutation, pkgConfig *config.PackageConfig) GoStructField {
	typeName, err := m.field.GetTypeNameWithOverride(pkgConfig)
	if err != nil {
		log.Error(err)
	}

	field := GoStructField{
		Name:        m.method.Signature.Return[0],
		Type:        m.method.Signature.Return[0],
		Tags:        fmt.Sprintf("`json:\"%s\"`", m.method.Name),
		TagKey:      m.field.Name,
		TypeName:    typeName,
		IsInterface: isGoInterface(m.field, pkgConfig),
		IsList:      m.field.Type.IsList(),
	}

	if m.method.ReturnAlias != "" {
		field.Name, field.Type = MutationResultField, m.method.ReturnType
	}

	return field
}

func getStructField(f schema.Field, pkgConfig *config.PackageConfig, parentType *schema.Type) GoStructField {
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
	if isGoInterface(f, pkgConfig) {
		typeNameSuffix = "Interface"
		isInterface = true
	}

	field := GoStructField{
		Description: f.GetDescription(),
		Name:        f.GetName(),
		TagKey:      f.Name,
		Tags:        f.GetTagsWithOverrides(*parentType, pkgConfig),
		IsInterface: isInterface,
		IsList:      isList,
		Type:        fmt.Sprintf("%s%s%s", typeNamePrefix, typeName, typeNameSuffix),
		TypeName:    typeName,
	}

	applyFieldConfig(&field, getTypeConfig(parentType.Name, pkgConfig.Types).GetFieldConfig(f.Name))

	return field
}

var tagValueRe = regexp.MustCompile(`(\w+:"[^",]*)(,[^"]*)?"`)

// applyFieldConfig applies a per-field override from the package config.
func applyFieldConfig(field *GoStructField, fc *config.FieldConfig) {
	if fc == nil {
		return
	}

	if fc.GoName != "" {
		field.Name = fc.GoName
	}

	if fc.GoType != "" {
		field.Type = fc.GoType
	}

	if fc.Pointer != nil && *fc.Pointer {
		switch {
		case field.IsInterface:
			log.WithFields(log.Fields{"field": field.TagKey}).Warn("pointer ignored for interface-typed field")
		case !strings.HasPrefix(field.Type, "*"):
			field.Type = "*" + field.Type
		}
	}

	if fc.OmitEmpty != nil {
		// Drop any existing omitempty first, so it is never doubled.
		field.Tags = strings.ReplaceAll(field.Tags, ",omitempty", "")

		if *fc.OmitEmpty {
			field.Tags = tagValueRe.ReplaceAllString(field.Tags, `$1,omitempty"`)
		}
	}
}

// constrainedResponseStructs is used to create response objects that contain
// fields that already exist in the expandedTypes.  This avoids creating full
// structs, and limits response objects to those types that are already
// referenced in the expandedTypes.
func constrainedResponseStructs(s *schema.Schema, pkgConfig *config.PackageConfig, expandedTypes *[]*schema.Type) ([]GoStruct, error) {
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
		for i, t := range pathTypes {
			// Skip doing anything with this type if it has already been expanded.
			if isExpanded(expandedTypes, t.GetName()) {
				continue
			}

			xxx := GoStruct{
				Name:        t.GetName(),
				Description: t.GetDescription(),
			}

			// The generated methods read the endpoints from the last type of the path, so
			// it has them whatever their type is: a type the package supplies is not
			// expanded, and would leave the field out.
			last := i == len(pathTypes)-1

			for _, f := range t.Fields {
				if isExpanded(expandedTypes, f.Type.GetGoTypeName()) || isInPath(pathTypes, f.Type.GetName()) || (last && hasEndpoint(query, f.Name)) {
					xxx.Fields = append(xxx.Fields, getStructField(f, pkgConfig, t))
				}
			}

			goStructs = append(goStructs, xxx)
		}

		// The generated method reads resp.<Go name of the root field>, so the response
		// struct's field must carry that name. It is not always the type's name: the
		// root field `docs` returns `DocumentationFields`.
		rootFieldNames, err := s.LookupQueryFieldsByFieldPath(query.Path)
		if err != nil {
			return nil, err
		}

		// Ensure we have a response struct for each of the endpoints in our config.
		for _, endpoint := range query.Endpoints {
			xxx := GoStruct{
				Name: endpointResponseName(endpoint),
			}

			// For the top level response object, we only use the first field path
			// that is received from the user.
			firstType := pathTypes[0]

			field := GoStructField{
				Name: rootFieldNames[0],
				Type: firstType.GetName(),
				Tags: fmt.Sprintf("`json:\"%s\"`", query.Path[0]),
			}

			xxx.Fields = append(xxx.Fields, field)
			goStructs = append(goStructs, xxx)
		}
	}

	return goStructs, nil
}

// hasEndpoint reports whether the query has an endpoint of the given name.
func hasEndpoint(query config.Query, name string) bool {
	for _, endpoint := range query.Endpoints {
		if endpoint.Name == name {
			return true
		}
	}

	return false
}

// mergeStructsByName combines structs that share a name into one, keeping the
// order in which each struct and each field was first seen. Two fields with the
// same name must agree on their type; if they do not, the config needs an
// explicit decision, so this fails rather than choosing one.
func mergeStructsByName(structs []GoStruct) ([]GoStruct, error) {
	merged := make([]GoStruct, 0, len(structs))
	index := map[string]int{}

	for _, st := range structs {
		i, seen := index[st.Name]
		if !seen {
			index[st.Name] = len(merged)
			merged = append(merged, st)

			continue
		}

		dst := &merged[i]

		if dst.Description == "" {
			dst.Description = st.Description
		}

		dst.SpecialUnmarshal = dst.SpecialUnmarshal || st.SpecialUnmarshal
		dst.GenerateGetters = dst.GenerateGetters || st.GenerateGetters

		for _, impl := range st.Implements {
			if !util.StringInStrings(impl, dst.Implements) {
				dst.Implements = append(dst.Implements, impl)
			}
		}

		for _, f := range st.Fields {
			existing := -1

			for j := range dst.Fields {
				if dst.Fields[j].Name == f.Name {
					existing = j
					break
				}
			}

			if existing == -1 {
				dst.Fields = append(dst.Fields, f)
				continue
			}

			if dst.Fields[existing].Type != f.Type {
				return nil, fmt.Errorf("struct %q: field %q has conflicting types %q and %q across query paths", st.Name, f.Name, dst.Fields[existing].Type, f.Type)
			}
		}
	}

	return merged, nil
}

// breakRecursiveTypes turns by-value fields into pointers wherever structs
// contain each other by value, which Go rejects as an invalid recursive type.
// Slices, existing pointers and interface fields already break a cycle and are
// left alone. The structs must be sorted so the choice of field is stable.
func breakRecursiveTypes(structs []GoStruct) {
	index := make(map[string]int, len(structs))
	for i, st := range structs {
		index[st.Name] = i
	}

	const (
		unvisited = iota
		visiting
		done
	)

	state := make([]int, len(structs))

	var visit func(i int)

	visit = func(i int) {
		state[i] = visiting

		for j := range structs[i].Fields {
			f := &structs[i].Fields[j]

			if f.IsList || f.IsInterface || strings.HasPrefix(f.Type, "*") || strings.HasPrefix(f.Type, "[]") {
				continue
			}

			target, ok := index[f.Type]
			if !ok {
				continue
			}

			switch state[target] {
			case visiting:
				log.WithFields(log.Fields{
					"struct": structs[i].Name,
					"field":  f.Name,
				}).Info("making field a pointer to break a recursive type")

				f.Type = "*" + f.Type
			case unvisited:
				visit(target)
			}
		}

		state[i] = done
	}

	for i := range structs {
		if state[i] == unvisited {
			visit(i)
		}
	}
}

func getTypeConfig(name string, typeConfigs []config.TypeConfig) *config.TypeConfig {
	for i := range typeConfigs {
		if typeConfigs[i].Name == name {
			return &typeConfigs[i]
		}
	}

	return nil
}

// checkFieldConfigs fails when a fields: entry names a field the type does not
// have, so a typo cannot silently do nothing.
func checkFieldConfigs(typeName string, c *config.TypeConfig, fields []schema.Field) error {
	if c == nil {
		return nil
	}

	for _, fc := range c.Fields {
		found := false

		for _, f := range fields {
			if f.Name == fc.Name {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("type %q: fields names %q, which is not a field of the type", typeName, fc.Name)
		}
	}

	return nil
}

// methodArgOptions are the per-mutation settings that change a method's arguments.
type methodArgOptions struct {
	// Exclude lists GraphQL argument names to leave out entirely.
	Exclude []string
	// Rename maps a GraphQL argument name to the Go parameter name.
	Rename map[string]string
	// GoTypes maps a GraphQL argument name to the Go type of its parameter.
	GoTypes map[string]string
	// IncludeOptional makes the parameter of an optional argument nilable, so a caller
	// can leave it unset (see config include_optional_arguments).
	IncludeOptional bool
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// safeParamName returns a name the templates can lower-case into a legal Go
// parameter. The templates lower-case the first letter, so an argument called
// `type` would become the keyword `type`.
func safeParamName(name string) string {
	if goKeywords[lowerFirst(name)] {
		return name + "Arg"
	}

	return name
}

// sharedPathInput returns the path input that has the parameter's name, as the templates
// spell it, and its type, or nil if there is none.
func sharedPathInput(pathInputs []GoMethodInputType, param GoMethodInputType) *GoMethodInputType {
	for i := range pathInputs {
		if lowerFirst(pathInputs[i].Name) == lowerFirst(param.Name) && pathInputs[i].Type == param.Type {
			return &pathInputs[i]
		}
	}

	return nil
}

// checkUniqueParameters fails when two parameters of a method would get the same name,
// which would not compile.
func checkUniqueParameters(m GoMethod) error {
	seen := map[string]bool{}

	for _, in := range m.Signature.Input {
		name := lowerFirst(in.Name)
		if seen[name] {
			return fmt.Errorf("method %q: parameter %q is declared more than once; exclude_arguments, or for a mutation rename_arguments, resolves it", m.Name, name)
		}

		seen[name] = true
	}

	return nil
}

// goMethodForField creates a new GoMethod based on a field.  Note that the
// implementation specific information like QueryString are not added to the
// method, and it is up to the caller to flavor the method accordingly.
//
// The received inputFields are to seed the initial objet.  This allows the
// caller to pass additional context about the received field's place in the
// schema that are used as a starting place for input variables, since the
// parent objects may require those inputs.
func goMethodForField(field schema.Field, pkgConfig *config.PackageConfig, inputFields map[string][]schema.Field, opts methodArgOptions) GoMethod {
	method := GoMethod{
		Name:        field.GetName(),
		FieldName:   field.GetName(),
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

	pathInputs := len(method.Signature.Input)

	var prefix, suffix string

	if field.Type.IsList() {
		prefix = "[]"
		method.Signature.ReturnSlice = true
	}

	if isGoInterface(field, pkgConfig) {
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
		if util.StringInStrings(methodArg.Name, opts.Exclude) {
			continue
		}

		typeName, err := methodArg.GetTypeNameWithOverride(pkgConfig)
		if err != nil {
			log.Error(err)
			continue
		}

		var methodArgPrefix string
		if methodArg.Type.IsList() {
			methodArgPrefix = "[]"
		}

		argName := methodArg.GetName()
		if renamed, ok := opts.Rename[methodArg.Name]; ok {
			argName = renamed
		}

		goType := fmt.Sprintf("%s%s", methodArgPrefix, typeName)
		if override, ok := opts.GoTypes[methodArg.Name]; ok {
			goType = override
		}

		inputType := GoMethodInputType{
			Name: safeParamName(argName),
			Type: goType,
		}

		// An argument that has the name and type of a parameter that the path
		// already takes, such as an `accountId` argument below `account(id:)`, is the
		// same value, so the method takes it once and sends it for both.
		if shared := sharedPathInput(method.Signature.Input[:pathInputs], inputType); shared != nil {
			method.QueryVars = append(method.QueryVars, QueryVar{
				Key:   methodArg.Name,
				Value: shared.Name,
				Type:  methodArg.Type.GetTypeName(),
			})

			continue
		}

		// The shared-input check above compares the type the argument has without this,
		// so an optional argument is matched as before.
		if opts.IncludeOptional && methodArg.Type.IsNullable() {
			if _, overridden := opts.GoTypes[methodArg.Name]; !overridden {
				inputType.Type = optionalGoType(inputType.Type, methodArg.Type.IsList())
			}

			if nilable(inputType.Type) {
				method.UnsetParams = append(method.UnsetParams, UnsetParam{Param: lowerFirst(inputType.Name), Key: methodArg.Name})
			}
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

// optionalGoType is the type of the parameter for a nullable argument whose Go type is
// goType: a pointer, so nil means unset. A list is a slice, which is nil already, and a
// type that is a pointer already, such as a field_type_override of "*Filter", stays.
func optionalGoType(goType string, isList bool) string {
	if isList || nilable(goType) {
		return goType
	}

	return "*" + goType
}

// nilable reports whether a value of the Go type can be nil, as far as the spelling
// of the type shows.
func nilable(goType string) bool {
	return strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") ||
		strings.HasPrefix(goType, "map[") || goType == "interface{}" || goType == "any"
}
