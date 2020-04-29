package schema

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// TODO: Remove this
var types = make(map[string]string)

// TODO: Need to refactor this
type TypeConfig struct {
	Name     string `yaml:"name"`
	CreateAs string `yaml:"createAs,omitempty"` // CreateAs is the Golang type to override whatever the default detected type would be
}

// Schema contains data about the GraphQL schema as returned by the server
type Schema struct {
	Types []*Type `json:"types"`
}

type Metadata struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`
}

// TypeGen is the mother type generator.
func (s *Schema) TypeGen(typeInfo TypeConfig) error {
	log.Infof("starting on: %+v", typeInfo)

	// Only add the new types
	if _, ok := types[typeInfo.Name]; !ok {
		output, err := s.Definition(typeInfo)
		if err != nil {
			return err
		}

		types[typeInfo.Name] = output
	}

	return nil
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

		err := s.TypeGen(TypeConfig{Name: subTName})
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
func (s *Schema) Definition(typeInfo TypeConfig) (string, error) {
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
		if typeInfo.CreateAs != "" {
			createAs = typeInfo.CreateAs
		} else {
			log.Warnf("creating scalar %s as string", t.Name)
		}

		output += "type " + t.Name + " " + createAs + "\n"
	case KindInterface:
		createAs := "interface{}"
		if typeInfo.CreateAs != "" {
			createAs = typeInfo.CreateAs
		}

		output += "type " + t.Name + " " + createAs + "\n"

	default:
		log.Warnf("unhandled object Kind: %s\n", t.Kind)
	}

	return output + "\n", nil
}

// Global type list lookup function
func (s *Schema) LookupTypeByName(typeName string) (*Type, error) {
	log.Debugf("looking for typeName: %s", typeName)

	for _, t := range s.Types {
		if t.Name == typeName {
			return t, nil
		}
	}

	return nil, fmt.Errorf("type by name %s not found", typeName)
}
