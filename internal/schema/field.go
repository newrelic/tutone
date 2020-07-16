package schema

import (
	"strings"

	"github.com/newrelic/tutone/internal/config"
)

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

// GetTypeNameWithOverride returns the typeName, taking into consideration any TypeOverride specified in the PackageConfig.
func (f *Field) GetTypeNameWithOverride(pkgConfig *config.PackageConfig) (string, error) {
	var typeName string
	var overrideType string
	var err error

	// Discover any TypeOverride override for the current field.
	for _, p := range pkgConfig.Types {
		if p.Name == f.GetName() {
			if p.TypeOverride != "" {
				overrideType = p.TypeOverride
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
	var fieldName string

	switch strings.ToLower(f.Name) {
	case "ids":
		// special case to avoid the struct field Ids, and prefer IDs instead
		fieldName = "IDs"
	case "id":
		fieldName = "ID"
	case "accountid":
		fieldName = "AccountID"
	default:
		fieldName = strings.Title(f.Name)
	}

	return fieldName
}

func (f *Field) GetTags() string {
	if f == nil {
		return ""
	}

	jsonTag := "`json:\"" + f.Name

	// Overrides
	if strings.EqualFold(f.Name, "id") {
		jsonTag += ",string"
	}

	return jsonTag + "\"`"
}
