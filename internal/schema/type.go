package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Type defines a specific type within the schema
type Type struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	EnumValues    []EnumValue `json:"enumValues,omitempty"`
	Fields        []Field     `json:"fields,omitempty"`
	InputFields   []Field     `json:"inputFields,omitempty"`
	Interfaces    []TypeRef   `json:"interfaces,omitempty"`
	PossibleTypes []TypeRef   `json:"possibleTypes,omitempty"`
}

// Save writes the schema out to a file
func (t *Type) Save(file string) error {
	if file == "" {
		return errors.New("unable to save schema, no file specified")
	}

	log.WithFields(log.Fields{
		"schema_file": file,
	}).Debug("saving schema")

	schemaFile, err := json.MarshalIndent(t, "", " ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, schemaFile, 0644)
}

// GetDescription formats the description into a GoDoc comment.
func (t *Type) GetDescription() string {
	if strings.TrimSpace(t.Description) == "" {
		return ""
	}

	return formatDescription(t.GetName(), t.Description)
}

// GetName returns the name of a Type, formatted for Go title casing.
func (t *Type) GetName() string {
	return formatGoName(t.Name)
}

// IsGoType is used to determine if a type in NerdGraph is already a native type of Golang.
func (t *Type) IsGoType() bool {
	goTypes := []string{
		"int",
		"string",
		"bool",
		"boolean",
	}

	name := strings.ToLower(t.GetName())

	for _, x := range goTypes {
		if x == name {
			return true
		}
	}

	return false
}

func (t *Type) GetQueryStringFields(s *Schema, depth, maxDepth int) string {
	depth++

	var lines []string

	sort.SliceStable(t.Fields, func(i, j int) bool {
		return t.Fields[i].Name < t.Fields[j].Name
	})

	parentFieldNames := []string{}

	for _, field := range t.Fields {
		kinds := field.Type.GetKinds()

		lastKind := kinds[len(kinds)-1]

		switch lastKind {
		case KindObject, KindInterface:
			if depth > maxDepth {
				continue
			}

			typeName := field.Type.GetTypeName()

			subT, err := s.LookupTypeByName(typeName)
			if err != nil {
				log.Error(err)
				continue
			}

			hasRequiredArg := func(Args []Field) bool {
				for _, a := range field.Args {
					kinds := a.Type.GetKinds()
					if len(kinds) > 0 {
						if kinds[0] == KindNonNull {
							return true
						}
					}
				}

				return false
			}

			// If any of the arguments for a given field are required, then we
			// currently skip the field in the query since we are not handling the
			// parameters necessary to fill that out.  TODO is perhaps to ensure that
			// all the query field arguments are available for each of the nested
			// fields.
			if hasRequiredArg(field.Args) {
				continue
			}

			line := fmt.Sprintf("%s {", field.Name)
			lines = append(lines, line)
			if lastKind == KindInterface {
				lines = append(lines, "\t__typename")
			}

			subTContent := subT.GetQueryStringFields(s, depth, maxDepth)
			subTLines := strings.Split(subTContent, "\n")
			for _, b := range subTLines {
				lines = append(lines, fmt.Sprintf("\t%s", b))
			}

			lines = append(lines, "}")
		default:
			lines = append(lines, field.Name)
			parentFieldNames = append(parentFieldNames, field.Name)
		}

	}

	for _, possibleType := range t.PossibleTypes {
		possibleT, err := s.LookupTypeByName(possibleType.Name)
		if err != nil {
			log.Error(err)
		}

		lines = append(lines, fmt.Sprintf("... on %s {", possibleType.Name))
		lines = append(lines, "\t__typename")

		possibleTContent := possibleT.GetQueryStringFields(s, depth, maxDepth)

		possibleTLines := strings.Split(possibleTContent, "\n")
		for _, b := range possibleTLines {
			// Here we skip the fields that are already expressed on the parent type.
			// Since we are enumerating the interface types on the type, we want to
			// reduce the query complexity, while still retaining all of the data.
			// This allows us to rely on the parent types fields and avoid increasing
			// the complexity by enumerating all fields on the PossibleTypes as well.
			if stringInStrings(b, parentFieldNames) {
				continue
			}

			lines = append(lines, fmt.Sprintf("\t%s", b))
		}
		lines = append(lines, "}")
	}

	return strings.Join(lines, "\n")
}

func (t *Type) GetField(name string) (*Field, error) {
	for _, f := range t.Fields {
		if f.Name == name {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("field '%s' not found", name)
}
