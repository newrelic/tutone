package schema

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
)

// Field is an attribute of a schema Type object.
type Field struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	Type         TypeRef     `json:"type"`
	Args         []Field     `json:"args,omitempty"`
	DefaultValue interface{} `json:"defaultValue,omitempty"`
}

// GetDescription formats the description into a GoDoc comment.
func (f *Field) GetDescription() string {
	if strings.TrimSpace(f.Description) == "" {
		return ""
	}

	return formatDescription("", f.Description)
}

// GetTypeNameWithOverride returns the typeName, taking into consideration any FieldTypeOverride specified in the PackageConfig.
func (f *Field) GetTypeNameWithOverride(pkgConfig *config.PackageConfig) (string, error) {
	var typeName string
	var overrideType string
	var err error

	// Discover any FieldTypeOverride override for the current field.
	nameToMatch := f.GetName()
	for _, p := range pkgConfig.Types {
		if p.Name == nameToMatch {
			if p.FieldTypeOverride != "" {
				log.Debugf("overriding typeref for %s, using type %s", nameToMatch, p.FieldTypeOverride)
				overrideType = p.FieldTypeOverride
			}
		}
	}

	// Set the typeName to the override or use what is specified in the schema.
	if overrideType != "" {
		typeName = overrideType
	} else {
		typeName, _, err = f.Type.GetType()
		if err != nil {
			return "", err
		}
	}

	return typeName, nil
}

// GetName returns a recusive lookup of the type name
func (f *Field) GetName() string {
	return formatGoName(f.Name)
}

// GetTags is used to return the Go struct tags for a field.
func (f *Field) GetTags() string {
	if f == nil {
		return ""
	}

	jsonTag := "`json:\"" + f.Name

	// Overrides
	// if strings.EqualFold(f.Name, "id") {
	// BUG for values that have been overridden in the config, this should not be assumed.
	// 	jsonTag += ",string"
	// }

	return jsonTag + "\"`"
}

func (f *Field) IsGoType() bool {
	goTypes := []string{
		"int",
		"string",
		"bool",
		"boolean",
	}

	name := strings.ToLower(f.Type.GetTypeName())

	for _, x := range goTypes {
		if x == name {
			return true
		}
	}

	return false
}
