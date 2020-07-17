package schema

import (
	"fmt"
	"strings"

	"github.com/newrelic/tutone/internal/config"
	log "github.com/sirupsen/logrus"
)

type TypeRef struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	OfType *TypeRef `json:"ofType,omitempty"`
}

func (r *TypeRef) IsList() bool {
	kinds := r.GetKinds()

	if len(kinds) > 0 && kinds[0] == KindList {
		return true
	}

	return false
}

// GetKind returns an array or the type kind
func (r *TypeRef) GetKinds() []Kind {
	tree := []Kind{}

	if r.Kind != "" && r.Kind != KindNonNull {
		tree = append(tree, r.Kind)
	}

	// Recursion FTW
	if r.OfType != nil {
		tree = append(tree, r.OfType.GetKinds()...)
	}

	return tree
}

// GetName returns a recusive lookup of the type name
func (r *TypeRef) GetName() string {
	var fieldName string

	switch strings.ToLower(r.Name) {
	case "ids":
		// special case to avoid the struct field Ids, and prefer IDs instead
		fieldName = "IDs"
	case "id":
		fieldName = "ID"
	case "accountid":
		fieldName = "AccountID"
	default:
		fieldName = strings.Title(r.Name)
	}

	return fieldName
}

func (r *TypeRef) GetTags() string {
	if r == nil {
		return ""
	}

	jsonTag := "`json:\"" + r.Name

	// Overrides
	if strings.EqualFold(r.Name, "id") {
		jsonTag += ",string"
	}

	return jsonTag + "\"`"
}

// GetTypeNameWithOverride returns the typeName, taking into consideration any TypeOverride specified in the PackageConfig.
func (r *TypeRef) GetTypeNameWithOverride(pkgConfig *config.PackageConfig) (string, error) {
	var typeName string
	var overrideType string
	var err error

	// Discover any TypeOverride override for the current field.
	for _, p := range pkgConfig.Types {
		if p.Name == r.GetName() {
			if p.FieldTypeOverride != "" {
				overrideType = p.FieldTypeOverride
			}
		}
	}

	// Set the typeName to the override or use what is specified in the schema.
	if overrideType != "" {
		typeName = overrideType
	} else {
		typeName, _, err = r.GetType()
		if err != nil {
			return "", err
		}
	}

	if r.IsList() {
		return fmt.Sprintf("[]%s", typeName), nil
	}

	return typeName, nil
}

// GetTypeName returns a recusive lookup of the type name
func (r *TypeRef) GetTypeName() string {
	if r != nil {
		if r.Name != "" {
			return r.Name
		}

		// Recursion FTW
		if r.OfType != nil {
			return r.OfType.GetTypeName()
		}
	}

	log.Errorf("failed to get name for %#v", *r)
	return "UNKNOWN"
}

// GetType resolves the given SchemaInputField into a field name to use on a go struct.
//  type, recurse, error
func (r *TypeRef) GetType() (string, bool, error) {
	if r == nil {
		return "", false, fmt.Errorf("can not get type of nil TypeRef")
	}

	switch n := r.GetTypeName(); n {
	case "String":
		return "string", false, nil
	case "Int":
		return "int", false, nil
	case "Boolean":
		return "bool", false, nil
	case "Float":
		return "float64", false, nil
	case "ID":
		// ID is a nested object, but behaves like an integer.  This may be true of other SCALAR types as well, so logic here could potentially be moved.
		return "int", false, nil
	case "":
		return "", true, fmt.Errorf("empty field name: %+v", r)
	default:
		return n, true, nil
	}
}

// GetDescription looks for anything in the description before \n\n---\n
// and filters off anything after that (internal messaging that is not useful here)
func (r *TypeRef) GetDescription() string {
	if strings.TrimSpace(r.Description) == "" {
		return ""
	}

	return "\t /* " + r.GetName() + " - " + filterDescription(r.Description) + " */\n"
}
