package schema

import (
	"strings"
)

type EnumValue struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	IsDeprecated      bool   `json:"isDeprecated"`
	DeprecationReason string `json:"deprecationReason"`
}

// GetDescription looks for anything in the description before \n\n---\n
// and filters off anything after that (internal messaging that is not useful here)
func (e *EnumValue) GetDescription() string {
	if strings.TrimSpace(e.Description) == "" {
		return ""
	}

	return "\t /* " + e.GetName() + " - " + filterDescription(e.Description) + " */\n"
}

// GetName returns a recusive lookup of the type name
func (e *EnumValue) GetName() string {
	var fieldName string

	switch strings.ToLower(e.Name) {
	case "ids":
		// special case to avoid the struct field Ids, and prefer IDs instead
		fieldName = "IDs"
	case "id":
		fieldName = "ID"
	case "accountid":
		fieldName = "AccountID"
	default:
		fieldName = strings.Title(e.Name)
	}

	return fieldName
}
