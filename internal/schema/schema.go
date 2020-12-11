package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"
)

// QueryArg is the key value pair for an nerdgraph query argument on an endpoint.  These might be required, or not.
type QueryArg struct {
	Key   string
	Value string
}

// Schema contains data about the GraphQL schema as returned by the server
type Schema struct {
	MutationType     *Type   `json:"mutationType,omitempty"`
	QueryType        *Type   `json:"queryType,omitempty"`
	SubscriptionType *Type   `json:"subscriptionType,omitempty"`
	Types            []*Type `json:"types,omitempty"`
}

type queryStringData struct {
	Endpoint string
	// TypeName is used to identify the name of the type for use in a template if its needed..
	TypeName string
	// Arguments that are specific to an endpoint.
	EndpointArgs []QueryArg
	// The complete set of arguments for this query.
	QueryArgs []QueryArg
	Fields    string
	FieldPath []string
}

type mutationStringData struct {
	MutationName string
	Args         []QueryArg
	Fields       string
}

func ParseResponse(resp *http.Response) (*QueryResponse, error) {
	if resp == nil {
		return nil, errors.New("unable to parse nil HTTP response")
	}

	log.Debug("reading response")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Trace(string(body))

	log.Debug("unmarshal JSON")
	ret := QueryResponse{}
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

// Load takes a file and unmarshals the JSON into a Schema struct
func Load(file string) (*Schema, error) {
	if file == "" {
		return nil, errors.New("unable to load schema, no file specified")
	}
	log.WithFields(log.Fields{
		"schema_file": file,
	}).Debug("loading schema")

	jsonFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	schema := Schema{}
	err = json.Unmarshal(byteValue, &schema)
	if err != nil {
		return nil, err
	}

	// Stats for logging
	var countTypes, countMutations, countQueries, countSubscriptions int
	countTypes = len(schema.Types)
	if schema.MutationType != nil {
		countMutations = len(schema.MutationType.Fields)
	}

	log.WithFields(log.Fields{
		"count_query":        countQueries,
		"count_subscription": countSubscriptions,
		"count_type":         countTypes,
		"count_mutation":     countMutations,
	}).Info("schema loaded")

	return &schema, nil
}

// Save writes the schema out to a file for later use.
func (s *Schema) Save(file string) error {
	if file == "" {
		return errors.New("unable to save schema, no file specified")
	}

	log.WithFields(log.Fields{
		"schema_file": file,
	}).Debug("saving schema")

	schemaFile, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, schemaFile, 0644)
}

func (s *Schema) LookupRootMutationTypeFieldByName(name string) (*Field, error) {
	for _, f := range s.MutationType.Fields {
		if f.Name == name {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("`RootMutationType.Field` by name %s not found", name)
}

func (s *Schema) LookupRootQueryTypeFieldByName(name string) (*Field, error) {
	for _, f := range s.QueryType.Fields {
		if f.Name == name {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("`RootQueryType.Field` by name %s not found", name)
}

// LookupTypeByName digs in the schema for a type that matches the given name.
// This is commonly used for retrieving the Type of a TypeRef, since the name
// is the only piece of data to go on.
func (s *Schema) LookupTypeByName(typeName string) (*Type, error) {
	for _, t := range s.Types {
		if t.Name == typeName {
			return t, nil
		}
	}

	return nil, fmt.Errorf("type by name %s not found", typeName)
}

func (s *Schema) LookupMutationByName(mutationName string) (*Field, error) {
	for _, f := range s.MutationType.Fields {
		if f.Name == mutationName {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("mutation by name %s not found", mutationName)
}

// LookupQueryTypesByFieldPath is used to retrieve the types, when all you know is
// the path of field names to an endpoint.
func (s *Schema) LookupQueryTypesByFieldPath(fieldPath []string) ([]*Type, error) {
	types := make([]*Type, len(fieldPath))
	var err error

	startingT, err := s.LookupTypeByName("RootQueryType")
	if err != nil {
		return nil, err
	}

	fieldType := func(t *Type, fieldName string) (*Type, error) {
		for _, f := range t.Fields {
			if f.Name == fieldName {
				return s.LookupTypeByName(f.Type.GetTypeName())
			}
		}

		return nil, fmt.Errorf("no field name %s on type %s", fieldName, t.Name)
	}

	found := 0
	t := startingT

	for _, fieldName := range fieldPath {
		t, err = fieldType(t, fieldName)
		if err != nil {
			return nil, err
		}

		types[found] = t
		found++
	}

	return types, nil
}

// GetInputFieldsForQueryPath is intended to return the fields that are
// available as input arguments when performing a query using the received
// query path.  For example, a []string{"actor", "account"} would look at
// `actor { account { ...` for the representivate types at those schema levels
// and determine collect the available arguments for return.
func (s *Schema) GetInputFieldsForQueryPath(queryPath []string) map[string][]Field {
	fields := make(map[string][]Field)

	pathTypes, err := s.LookupQueryTypesByFieldPath(queryPath)
	if err != nil {
		log.Error(err)
	}

	for i, t := range pathTypes {
		for _, f := range t.Fields {
			// before the last element
			if i+1 < len(queryPath) {
				pathName := queryPath[i+1]
				if f.Name == pathName {
					if len(f.Args) > 0 {
						fields[pathName] = append(fields[pathName], f.Args...)
					}
				}
			}
		}
	}

	return fields
}

// QueryFieldsForTypeName will lookup a type by the received name, and return
// the query fields for that type, or log an error and return and empty string.
// The returned string is used in a mutation, so that the relevant fields are
// returned once the mutation is performed, or during a query for a given type.
func (s *Schema) QueryFieldsForTypeName(name string, maxDepth int) string {
	t, err := s.LookupTypeByName(name)
	if err != nil {
		log.Errorf("failed to to retrieve type by name: %s", err)
		return ""
	}

	return t.GetQueryStringFields(s, 0, maxDepth)
}

// BuildQueryArgsForEndpoint is meant to fill in the data necessary for a query(<args_go_here>)
// string.  i.e: query($guids: [String]!) { actor ...
func (s *Schema) BuildQueryArgsForEndpoint(t *Type, fields []string, includeNullable bool) []QueryArg {
	args := []QueryArg{}

	for _, f := range t.Fields {
		if stringInStrings(f.Name, fields) {
			for _, a := range f.Args {
				// TODO implement optional arguments.
				if !a.IsRequired() && !includeNullable {
					continue
				}

				args = append(args, s.GetQueryArg(a))
			}
		}
	}

	return args
}

// GetQueryStringForEndpoint packs a nerdgraph query header and footer around the set of query fields for a given type name and endpoint.
func (s *Schema) GetQueryStringForEndpoint(typePath []*Type, fieldPath []string, endpoint string, depth int, includeNullable bool) string {

	// We use the final type in the type path so that we can locate the field on
	// this type by the same name of the receveid endpoint.  Without this
	// information, we've no idea where to look for the endpoint, since the name
	// could be located in several places in the schema.
	t := typePath[len(typePath)-1]

	data := queryStringData{}

	// Set the TypeName so that we can create a special UnmarshalJSON where we need it.
	data.TypeName = t.GetName()
	data.Endpoint = endpoint

	// Format the path arguments for the query.
	inputFields := s.GetInputFieldsForQueryPath(fieldPath)
	for _, pathName := range fieldPath {
		// Match the field paths we received with the input fields.
		if fields, ok := inputFields[pathName]; ok {
			for _, f := range fields {
				inputTypeName := fmt.Sprintf("%s%s", pathName, f.GetName())

				fieldSpec := fmt.Sprintf("%s(%s: $%s)", pathName, f.Name, inputTypeName)
				data.FieldPath = append(data.FieldPath, fieldSpec)

				// TODO implement optional arguments.
				if f.IsRequired() {
					data.QueryArgs = append(data.QueryArgs, QueryArg{
						Key:   inputTypeName,
						Value: fmt.Sprintf("%s!", f.Type.GetTypeName()),
					})
				}
			}

		} else {
			data.FieldPath = append(data.FieldPath, pathName)
		}
	}

	// Append to QueryArgs and EndpointArgs only after the parent field
	// requirements have been added so that they are last.
	args := s.BuildQueryArgsForEndpoint(t, []string{endpoint}, includeNullable)
	data.EndpointArgs = append(data.EndpointArgs, args...)
	// Append all the endpoint args to the query args
	data.QueryArgs = append(data.QueryArgs, data.EndpointArgs...)

	// Match the endpoint field
	for _, f := range t.Fields {
		if f.Name == endpoint {
			fieldType, lookupErr := s.LookupTypeByName(f.Type.GetTypeName())
			if lookupErr != nil {
				log.Error(lookupErr)
				return ""
			}

			if depth > 0 {
				data.Fields = PrefixLineTab(fieldType.GetQueryStringFields(s, 0, depth))
			}
			break
		}
	}

	tmpl, err := template.New(t.GetName()).Funcs(sprig.TxtFuncMap()).Parse(queryHeaderTemplate)
	if err != nil {
		log.Error(err)
		return ""
	}

	var result bytes.Buffer

	err = tmpl.Execute(&result, data)
	if err != nil {
		log.Error(err)
		return ""
	}

	final := result.String() + queryFooter + strings.Repeat(" }", len(data.FieldPath)-1)

	return final
}

// GetQueryArg returns the GraphQL formatted Query Argument
// including formatting for List and NonNull
func (s *Schema) GetQueryArg(field Field) QueryArg {
	queryArg := QueryArg{
		Key:   field.Name,
		Value: field.Type.GetTypeName(),
	}

	// Order matters here
	if field.Type.IsList() {
		queryArg.Value = "[" + queryArg.Value + "]"
	}
	if field.IsRequired() {
		queryArg.Value += "!"
	}

	return queryArg
}

// GetQueryStringForMutation packs a nerdgraph query header and footer around the set of query fields GraphQL mutation name.
func (s *Schema) GetQueryStringForMutation(mutation *Field, depth int) string {

	data := mutationStringData{
		MutationName: mutation.Name,
	}

	fieldType, lookupErr := s.LookupTypeByName(mutation.Type.GetTypeName())
	if lookupErr != nil {
		log.Error(lookupErr)
		return ""
	}

	for _, a := range mutation.Args {
		data.Args = append(data.Args, s.GetQueryArg(a))
	}

	data.Fields = PrefixLineTab(fieldType.GetQueryStringFields(s, 0, depth))
	tmpl, err := template.New(fieldType.GetName()).Funcs(sprig.TxtFuncMap()).Parse(mutationHeaderTemplate)
	if err != nil {
		log.Error(err)
		return ""
	}

	var result bytes.Buffer

	err = tmpl.Execute(&result, data)
	if err != nil {
		log.Error(err)
		return ""
	}

	final := result.String() + mutationFooter

	return final
}

// RecursiveLookupFieldByPath traverses the GraphQL query types
// based on the provided query fields path which represents a
// GraphQL query. This method returns the last field of the last "node"
// of the query tree "branch" - i.e. actor.apiAccess.key where `key`
// is the primary target "leaf".
//
// e.g. `query { actor { apiAccess { key }}}` would have a path of ["actor", "apiAccess", "key"]
func (s *Schema) RecursiveLookupFieldByPath(queryFieldPath []string, obj *Type) *Field {
	for _, q := range queryFieldPath {
		field, _ := obj.GetField(q)

		// If we've reached the end of the graphQL query branch
		// and the last query field name matches the one we're
		// searching for, we can return it here.
		if len(queryFieldPath) == 1 && queryFieldPath[0] == q {
			return field
		}

		matchingFieldType, _ := s.LookupTypeByName(field.Type.GetTypeName())

		// Reduce the slice of fields as we traverse and find matching data
		// so we can eventually stop the recursion when we reach the end.
		remainingFields := queryFieldPath[1:]

		found := s.RecursiveLookupFieldByPath(remainingFields, matchingFieldType)
		if found != nil && len(remainingFields) == 1 {
			return found
		}
	}

	return nil
}

var queryHeaderTemplate = `query(
	{{- range .QueryArgs}}
	${{.Key}}: {{.Value}},
	{{- end}}
) { {{ .FieldPath | join " { " }} { {{.Endpoint}}(
	{{- range .EndpointArgs}}
	{{.Key}}: ${{.Key}},
	{{- end}}
) {
{{.Fields}}
`

var mutationHeaderTemplate = `mutation(
	{{- range .Args}}
	${{.Key}}: {{.Value}},
	{{- end}}
) { {{.MutationName}}(
	{{- range .Args}}
	{{.Key}}: ${{.Key}},
	{{- end}}
) {
{{.Fields}}
`

var queryFooter = `} } }`
var mutationFooter = `} }`
