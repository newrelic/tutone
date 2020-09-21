package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
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
func (s *Schema) LookupTypeByName(typeName string) (*Type, error) {
	for _, t := range s.Types {
		if t.Name == typeName {
			return t, nil
		}
	}

	return nil, fmt.Errorf("type by name %s not found", typeName)
}

// QueryFieldsForTypeName will lookup a type by the received name, and return the query fields for that type, or log an error and return and empty string.
func (s *Schema) QueryFieldsForTypeName(name string) string {
	t, err := s.LookupTypeByName(name)
	if err != nil {
		log.Errorf("failed to to retrieve type by name: %s", err)
		return ""
	}

	return s.QueryFields(t)
}

// QueryFields returns a string that contains all of the fields possible during a query, including nested objects.
func (s *Schema) QueryFields(t *Type) string {
	var lines []string

	sort.SliceStable(t.Fields, func(i, j int) bool {
		return t.Fields[i].Name < t.Fields[j].Name
	})

	for _, field := range t.Fields {
		kinds := field.Type.GetKinds()

		if len(kinds) > 0 && kinds[len(kinds)-1] == "OBJECT" {
			line := fmt.Sprintf("%s {", field.Name)
			lines = append(lines, line)

			typeName := field.Type.GetTypeName()

			subT, err := s.LookupTypeByName(typeName)
			if err != nil {
				log.Error(err)
			}

			subTContent := s.QueryFields(subT)
			subTLines := strings.Split(subTContent, "\n")
			for _, b := range subTLines {
				lines = append(lines, fmt.Sprintf("\t%s", b))
			}

			lines = append(lines, "}")
		} else {
			lines = append(lines, field.Name)
		}
	}

	for _, possibleType := range t.PossibleTypes {

		possibleT, err := s.LookupTypeByName(possibleType.Name)
		if err != nil {
			log.Error(err)
		}

		lines = append(lines, fmt.Sprintf("... on %s {", possibleType.Name))

		possibleTContent := s.QueryFields(possibleT)

		possibleTLines := strings.Split(possibleTContent, "\n")
		for _, b := range possibleTLines {
			lines = append(lines, fmt.Sprintf("\t%s", b))
		}
		lines = append(lines, "}")
	}

	return strings.Join(lines, "\n")
}

// QueryArgs is meant to fill in the data necessary for a query(<args_go_here>) string.  For example, the
// query($guids: [String]!) { actor ...
func (s *Schema) QueryArgs(t *Type, fields []string) []QueryArg {
	args := []QueryArg{}

	for _, f := range t.Fields {
		if stringInStrings(f.Name, fields) {

			for _, a := range f.Args {
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

				args = append(args, queryArg)

			}
		}
	}

	return args
}

// GetQueryStringForEndpoint packs a nerdgraph query header and footer around the set of query fields for a given type name and endpoint.
func (s *Schema) GetQueryStringForEndpoint(name string, endpoint string) string {
	t, err := s.LookupTypeByName(name)
	if err != nil {
		log.Error(err)
		return ""
	}

	args := s.QueryArgs(t, []string{endpoint})

	var queryFields string
	for _, f := range t.Fields {
		if f.Name == endpoint {
			fieldType, lookupErr := s.LookupTypeByName(f.Type.Name)
			if lookupErr != nil {
				log.Error(lookupErr)
				return ""
			}

			queryFields = s.QueryFields(fieldType)
			break
		}
	}

	data := struct {
		TypeName string
		Endpoint string
		Args     []QueryArg
		Fields   string
	}{
		t.Name,
		endpoint,
		args,
		PrefixLineTab(queryFields),
	}

	tmpl, err := template.New(name).Funcs(sprig.TxtFuncMap()).Parse(queryHeader)
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

	final := result.String() + queryFooter
	log.Printf("final: %+v", final)

	return final
}

var queryHeader = `query(
	{{- range .Args}}
	${{.Key}}: {{.Value}},
	{{- end}}
) { {{.TypeName|lower}} { {{.Endpoint}}(
	{{- range .Args}}
	{{.Key}}: ${{.Key}},
	{{- end}}
) {
{{.Fields}}
`

var queryFooter = `} } }`
