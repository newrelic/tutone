package schema

import (
	"strings"
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

// GetDescription looks for anything in the description before \n\n---\n
// and filters off anything after that (internal messaging that is not useful here)
func (t *Type) GetDescription() string {
	if strings.TrimSpace(t.Description) == "" {
		return ""
	}

	return "\t /* " + t.GetName() + " - " + filterDescription(t.Description) + " */\n"
}

// GetName returns a recusive lookup of the type name
func (t *Type) GetName() string {
	var fieldName string

	switch strings.ToLower(t.Name) {
	case "ids":
		// special case to avoid the struct field Ids, and prefer IDs instead
		fieldName = "IDs"
	case "id":
		fieldName = "ID"
	case "accountid":
		fieldName = "AccountID"
	default:
		fieldName = strings.Title(t.Name)
	}

	return fieldName
}

func (t *Type) GetTags() string {
	if t == nil {
		return ""
	}

	jsonTag := "`json:\"" + t.Name

	// Overrides
	if strings.EqualFold(t.Name, "id") {
		jsonTag += ",string"
	}

	return jsonTag + "\"`"
}
