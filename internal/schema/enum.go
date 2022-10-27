package schema

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type EnumValue struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	IsDeprecated      bool   `json:"isDeprecated"`
	DeprecationReason string `json:"deprecationReason"`
}

// GetDescription formats the description into a GoDoc comment.
func (e *EnumValue) GetDescription() string {
	if strings.TrimSpace(e.Description) == "" {
		return ""
	}

	return formatDescription("", e.Description)
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
		caser := cases.Title(language.Und, cases.NoLower)
		fieldName = caser.String(e.Name)
	}

	return fieldName
}
