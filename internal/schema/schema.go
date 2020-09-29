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

	"github.com/Masterminds/sprig"
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

// QueryArgs is meant to fill in the data necessary for a query(<args_go_here>)
// string.  For example, query($guids: [String]!) { actor ...
func (s *Schema) QueryArgs(t *Type, fields []string) []QueryArg {
	args := []QueryArg{}

	for _, f := range t.Fields {
		if stringInStrings(f.Name, fields) {

			for _, a := range f.Args {
				queryArg := QueryArg{
					Key: a.Name,
				}

				var typeName = a.Type.GetTypeName()
				kinds := a.Type.GetKinds()
				var next Kind
				left := kinds
				remain := len(left)

				for remain > 0 {
					next, left = left[len(left)-1], left[:len(left)-1]
					switch next {
					case KindNonNull:
						typeName = fmt.Sprintf("%s!", typeName)
					case KindList:
						typeName = fmt.Sprintf("[%s]", typeName)
					}

					remain = len(left)
				}

				queryArg.Value = typeName

				args = append(args, queryArg)
			}
		}
	}

	return args
}

// GetQueryStringForEndpoint packs a nerdgraph query header and footer around the set of query fields for a given type name and endpoint.
func (s *Schema) GetQueryStringForEndpoint(typePath []*Type, fieldPath []string, endpoint string, depth int) string {

	t := typePath[len(typePath)-1]
	args := s.QueryArgs(t, []string{endpoint})

	data := struct {
		TypeName  string
		Endpoint  string
		Args      []QueryArg
		Fields    string
		FieldPath []string
	}{}

	data.TypeName = t.GetName()
	data.Endpoint = endpoint
	data.Args = args
	data.FieldPath = fieldPath

	// Match the type field to the endpoint.
	for _, f := range t.Fields {
		if f.Name == endpoint {
			fieldType, lookupErr := s.LookupTypeByName(f.Type.GetTypeName())
			if lookupErr != nil {
				log.Error(lookupErr)
				return ""
			}

			data.Fields = PrefixLineTab(fieldType.GetQueryStringFields(s, 0, depth))
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

// GetQueryStringForMutation packs a nerdgraph query header and footer around the set of query fields GraphQL mutation name.
func (s *Schema) GetQueryStringForMutation(mutation *Field, depth int) string {

	// t := typePath[len(typePath)-1]
	data := struct {
		// TypeName string
		MutationName string
		Args         []QueryArg
		Fields       string
	}{}

	data.MutationName = mutation.GetName()

	fieldType, lookupErr := s.LookupTypeByName(mutation.Type.GetTypeName())
	if lookupErr != nil {
		log.Error(lookupErr)
		return ""
	}

	for _, a := range mutation.Args {
		queryArg := QueryArg{
			Key: a.Name,
		}

		var value string
		var suffix string

		if a.Type.Kind == KindNonNull {
			suffix = "!"
		}

		typeKinds := a.Type.GetKinds()
		if typeKinds[0] == KindList {
			value = fmt.Sprintf("[%s]%s", a.Type.GetTypeName(), suffix)
		} else {
			value = fmt.Sprintf("%s%s", a.Type.GetTypeName(), suffix)
		}

		queryArg.Value = value

		data.Args = append(data.Args, queryArg)

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

var queryHeaderTemplate = `query(
	{{- range .Args}}
	${{.Key}}: {{.Value}},
	{{- end}}
) { {{ .FieldPath | join " { " }} { {{.Endpoint}}(
	{{- range .Args}}
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
