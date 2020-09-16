package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

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

func (s *Schema) QueryFieldsForTypeName(name string) string {
	t, err := s.LookupTypeByName(name)
	if err != nil {
		log.Errorf("failed to to retrieve type by name")
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
