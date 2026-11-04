package lang

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

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
					if err := checkEndpointArguments(endpoint, field); err != nil {
						return nil, err
					}

					method := goMethodForField(field, pkgConfig, inputFields, methodArgOptions{Exclude: endpoint.ExcludeArguments, GoTypes: endpoint.ArgumentGoTypes})

					method.QueryString = s.GetQueryStringForEndpoint(typePath, pkgQuery.Path, endpoint)
					method.ResponseObjectType = endpointResponseName(endpoint)
					method.Signature.ReturnPath = returnPath

					method.FieldName = method.Name
					method.MethodName = "Get" + upperFirst(method.Name)
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

func upperFirst(s string) string {
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
	var methods []GoMethod

	if len(pkgConfig.Mutations) == 0 {
		return nil, nil
	}

	for _, pkgMutation := range pkgConfig.Mutations {
		fields := s.LookupMutationsByPattern(pkgMutation.Name)

		if err := checkArgumentConfig(pkgMutation, fields); err != nil {
			return nil, err
		}

		for _, field := range fields {
			method := goMethodForField(field, pkgConfig, nil, methodArgOptions{
				Exclude: pkgMutation.ExcludeArguments,
				Rename:  pkgMutation.RenameArguments,
				GoTypes: pkgMutation.ArgumentGoTypes,
			})
			method.QueryString = s.GetQueryStringForMutation(&field, pkgMutation)

			methods = append(methods, method)
		}
	}

	if len(methods) > 0 {
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

				// If any of the fields for this type are an interface type, then we
				// need to signal to the template an UnmarshalJSON() should be
				// rendered.
				if f.Type.IsInterface() {
					xxx.SpecialUnmarshal = true
				}

				xxx.Fields = append(xxx.Fields, getStructField(f, pkgConfig, t))
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

	constrained, err := constrainedResponseStructs(s, pkgConfig, expandedTypes)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	structsForGen = append(structsForGen, constrained...)

	// The same struct can come from more than one query path (e.g. Actor from
	// actor.cloud and actor.account.cloud). Declaring it twice does not compile.
	structsForGen, err = mergeStructsByName(structsForGen)
	if err != nil {
		return nil, nil, nil, nil, err
	}

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
	if f.Type.IsInterface() {
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
