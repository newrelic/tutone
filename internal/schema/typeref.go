package schema

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// TypeRef is a GraphQL reference to a Type.
type TypeRef struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	OfType *TypeRef `json:"ofType,omitempty"`
}

// IsList determines if a TypeRef is of a KIND LIST.
func (r *TypeRef) IsList() bool {
	kinds := r.GetKinds()

	if len(kinds) > 0 && kinds[0] == KindList {
		return true
	}

	return false
}

// GetKinds returns an array or the type kind
func (r *TypeRef) GetKinds() []Kind {
	tree := []Kind{}

	if r.Kind != "" {
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
	return formatGoName(r.Name)
}

// GetTypeName returns the name of the current type, or performs a recursive lookup to determine the name of the nested OfType object's name.  In the case that neither are matched, the string "UNKNOWN" is returned.  In the GraphQL schema, a non-empty name seems to appear only once in a TypeRef tree, so we want to find the first non-empty.
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
		return formatGoName(n), true, nil
	}
}

// GetDescription looks for anything in the description before \n\n---\n
// and filters off anything after that (internal messaging that is not useful here)
func (r *TypeRef) GetDescription() string {
	if strings.TrimSpace(r.Description) == "" {
		return ""
	}

	return formatDescription("", r.Description)
}

func (r *TypeRef) IsInputObject() bool {
	kinds := r.GetKinds()

	if len(kinds) > 0 && kinds[0] == KindInputObject {
		return true
	}

	// Not sure if we need this check yet...
	if r.OfType != nil && r.OfType.Kind == KindInputObject {
		return true
	}

	return false
}

func (r *TypeRef) IsScalarID() bool {
	return r.GetTypeName() != "ID" && r.OfType != nil && r.OfType.Kind == KindScalar
}
