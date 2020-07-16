package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
)

// Types TODO: Remove this
var Types = make(map[string]string)

type MutationInfo struct {
	Name string `yaml:"name"`
}

type SubscriptionInfo struct {
	Name string `yaml:"name"`
}

type QueryInfo struct {
	Name string `yaml:"name"`
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

// Save writes the schema out to a file
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

// TODO: Return the resolved types to allow other functions to use it
//       and we can avoid the global var `Types`
func ResolveSchemaTypes(schema Schema, typeInfo []config.TypeConfig) error {
	for _, info := range typeInfo {
		typesOutput, err := schema.TypeGen(info)
		if err != nil {
			log.Errorf("error while generating type %s: %s", info.Name, err)
		}

		fmt.Printf("\nTypes Output: %+v \n", typesOutput)
	}

	return nil
}

// TypeGen is the mother type generator.
func (s *Schema) TypeGen(typeInfo config.TypeConfig) (map[string]string, error) {
	log.Infof("starting on: %+v", typeInfo)

	// Only add the new types
	if _, ok := Types[typeInfo.Name]; !ok {
		output, err := s.Definition(typeInfo)
		if err != nil {
			return Types, err
		}

		Types[typeInfo.Name] = output
	}

	fmt.Print("\n TypeGen Done....\n\n")

	return Types, nil
}

func (s *Schema) lineForField(f Field) string {
	output := f.GetDescription()

	log.Infof("handling kind %s: %+v", f.Type.Kind, f.Type)
	fieldType, recurse, err := f.Type.GetType()
	if err != nil {
		// If we have an error, then we don't know how to handle the type to
		// determine the field name.
		log.Errorf("error resolving first non-empty name from field: %#v: %s", f.Type, err.Error())
	}

	if recurse {
		log.Debugf("recurse search for %s: %+v", fieldType, f.Type)

		// The name of the nested sub-type.  We take the first value here as the root name for the nested type.
		subTName := f.Type.GetTypeName()
		log.Tracef("subTName %s", subTName)

		_, err := s.TypeGen(config.TypeConfig{Name: subTName})
		if err != nil {
			log.Errorf("ERROR while resolving sub type %s: %s\n", subTName, err)
		}

		fieldType = subTName
	}

	fieldTypePrefix := ""

	if f.Type.IsList() {
		fieldTypePrefix = "[]"
	}

	fieldTags := f.GetTags()

	output += "\t" + f.GetName() + " " + fieldTypePrefix + fieldType + " " + fieldTags + "\n"

	return output
}

// Definition generates the Golang definition of the type
func (s *Schema) Definition(typeInfo config.TypeConfig) (string, error) {
	t, err := s.LookupTypeByName(typeInfo.Name)
	if err != nil {
		return "", err
	}

	// Start with the type description
	output := t.GetDescription()

	switch t.Kind {
	case KindInputObject, KindObject:
		output += "type " + t.Name + " struct {\n"

		// Fill in the struct fields for an input type
		for _, f := range t.InputFields {
			output += s.lineForField(f)
		}

		for _, f := range t.Fields {
			output += s.lineForField(f)
		}

		output += "}\n"
	case KindENUM:
		output += "type " + t.Name + " string\n\n"
		output += "const (\n"

		for _, v := range t.EnumValues {
			output += v.GetDescription()
			output += "\t" + v.Name + " " + t.Name + " = \"" + v.Name + "\"\n"
		}

		output += ")\n"
	case KindScalar:
		// Default to string for scalars, but warn this is might not be what they want.
		createAs := "string"
		if typeInfo.TypeOverride != "" {
			createAs = typeInfo.TypeOverride
		} else {
			log.Warnf("creating scalar %s as string", t.Name)
		}

		output += "type " + t.Name + " " + createAs + "\n"
	case KindInterface:
		createAs := "interface{}"
		if typeInfo.TypeOverride != "" {
			createAs = typeInfo.TypeOverride
		}

		output += "type " + t.Name + " " + createAs + "\n"

	default:
		log.Warnf("unhandled object Kind: %s\n", t.Kind)
	}

	return output + "\n", nil
}

// Global type list lookup function
func (s *Schema) LookupTypeByName(typeName string) (*Type, error) {
	log.Tracef("looking for typeName: %s", typeName)

	for _, t := range s.Types {
		if t.Name == typeName {
			return t, nil
		}
	}

	return nil, fmt.Errorf("type by name %s not found", typeName)
}
