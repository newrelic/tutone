package schema

import (
	"fmt"
	"sort"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Expander is mean to hold the state while the schema is being expanded.
type Expander struct {
	sync.Mutex
	schema        *Schema
	expandedTypes []*Type
	skipTypes     []string
}

// NewExpander is to return a sane Expander.
func NewExpander(schema *Schema, skipTypes []string) *Expander {
	return &Expander{
		schema:    schema,
		skipTypes: skipTypes,
	}
}

// ExpandedTypes is the final report of all the expanded types.
func (x *Expander) ExpandedTypes() *[]*Type {
	x.Lock()
	expandedTypes := x.expandedTypes
	x.Unlock()

	for _, expandedType := range expandedTypes {
		log.WithFields(log.Fields{
			"name": expandedType.Name,
			"kind": expandedType.Kind,
		}).Debug("type included")
	}

	sort.SliceStable(expandedTypes, func(i, j int) bool {
		return expandedTypes[i].Name < expandedTypes[j].Name
	})

	return &expandedTypes
}

// ExpandType is used to populate the expander, one Type at a time.
func (x *Expander) ExpandType(t *Type) (err error) {
	if t == nil {
		return fmt.Errorf("unable to expand nil Type")
	}

	if stringInStrings(t.Name, x.skipTypes) {
		log.WithFields(log.Fields{
			"name":             t.Name,
			"skip_type_create": true,
		}).Debug("Not expanding skipped type")

		return nil
	}

	if x.includeType(t) {
		err := x.expandType(t)
		if err != nil {
			log.WithFields(log.Fields{
				"name": t.Name,
			}).Errorf("failed to expand type: %s", err)
		}
	}

	return nil
}

// ExpandTypeFromName will expand a named type if found or error.
func (x *Expander) ExpandTypeFromName(name string) error {
	t, err := x.schema.LookupTypeByName(name)
	if err != nil {
		return fmt.Errorf("failed lookup method argument: %s", err)
	}

	return x.ExpandType(t)
}

// includeType is used make sure a Type has been expanded.  A boolean ok is
// returned if the type was included.  A false value means that the type was
// already included.
func (x *Expander) includeType(t *Type) bool {
	var ok bool

	x.Lock()
	if !hasType(t, x.expandedTypes) {
		log.WithFields(log.Fields{
			"name": t.Name,
		}).Trace("including type")

		x.expandedTypes = append(x.expandedTypes, t)
		ok = true
	}
	x.Unlock()

	return ok
}

// expandType receives a Type which is used to determine the Type for all
// nested fields.
func (x *Expander) expandType(t *Type) error {
	if t == nil {
		return fmt.Errorf("unable to expand nil type")
	}

	// InputFields and Fields are handled the same way, so combine them to loop over.
	var fields []Field
	fields = append(fields, t.Fields...)
	fields = append(fields, t.InputFields...)

	log.WithFields(log.Fields{
		"name":              t.GetName(),
		"interfaces":        t.Interfaces,
		"possibleTypes":     t.PossibleTypes,
		"kind":              t.Kind,
		"fields_count":      len(t.Fields),
		"inputFields_count": len(t.InputFields),
	}).Debug("expanding type")

	// Collect the nested types from InputFields and Fields.
	for _, i := range fields {
		log.WithFields(log.Fields{
			"args": len(i.Args),
			"name": i.GetName(),
			"type": i.Type.Kind,
		}).Debug("expanding field")

		var err error

		if i.Type.OfType != nil {
			err = x.ExpandTypeFromName(i.Type.OfType.GetTypeName())
			if err != nil {
				log.WithFields(log.Fields{
					"ofType": i.Type.OfType.GetTypeName(),
					"type":   i.Type.Name,
				}).Errorf("failed to expand OfType for Type: %s", err)
				// continue
			}
		}

		err = x.ExpandTypeFromName(i.Type.GetTypeName())
		if err != nil {
			log.WithFields(log.Fields{
				"type": i.Type.Name,
			}).Errorf("failed to expand Type.Name: %s", err)
		}

		for _, arg := range i.Args {
			err := x.ExpandTypeFromName(arg.Type.GetTypeName())
			if err != nil {
				log.WithFields(log.Fields{
					"name": arg.Type.GetTypeName(),
				}).Errorf("failed to expand type from name: %s", err)
			}
		}

		for _, possibleType := range t.PossibleTypes {
			err := x.ExpandTypeFromName(possibleType.Name)
			if err != nil {
				log.WithFields(log.Fields{
					"name": possibleType.Name,
				}).Errorf("failed to expand type from name: %s", err)
			}
		}

		for _, typeInterface := range t.Interfaces {
			err := x.ExpandTypeFromName(typeInterface.Name)
			if err != nil {
				log.WithFields(log.Fields{
					"name": typeInterface.Name,
				}).Errorf("failed to expand type from name: %s", err)
			}
		}
	}

	return nil
}
