package schema

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// filterDescription uses a regex to parse certain data out of the
// description of an item
func filterDescription(description string) string {
	var ret string

	re := regexp.MustCompile(`(?s)(.*)\n---\n`)
	desc := re.FindStringSubmatch(description)

	log.Tracef("description: %#v", desc)

	if len(desc) > 1 {
		ret = desc[1]
	} else {
		ret = description
	}

	return strings.TrimSpace(ret)
}

func formatDescription(name string, description string) string {
	if strings.TrimSpace(description) == "" {
		return ""
	}

	filtered := filterDescription(description)
	lines := strings.Split(filtered, "\n")

	var resultLines []string

	for i, l := range lines {
		if i == 0 && name != "" {
			resultLines = append(resultLines, fmt.Sprintf("// %s - %s", name, l))
		} else {
			resultLines = append(resultLines, fmt.Sprintf("// %s", l))
		}
	}

	return strings.Join(resultLines, "\n")
}

// typeNameInTypes determines if a name is already present in a set of TypeInfo.
func typeNameInTypes(s string, types []TypeInfo) bool {
	for _, t := range types {
		if t.Name == s {
			return true
		}
	}

	return false
}

// hasType determines if a Type is already present in a slice of Type objects.
func hasType(t *Type, types []*Type) bool {
	for _, tt := range types {
		if t.Name == tt.Name {
			return true
		}
	}

	return false
}

// ExpandType receives a Type which is used to determine the Type for all
// nested fields.
func ExpandType(s *Schema, t *Type) (*[]*Type, error) {
	if s == nil {
		return nil, fmt.Errorf("unable to expand type from nil schema")
	}

	if t == nil {
		return nil, fmt.Errorf("unable to expand nil type")
	}

	log.Debugf("expanding type %s", t.GetName())

	var f []*Type

	// InputFields and Fields are handled the same way, so combine them to loop over.
	var fields []Field
	fields = append(fields, t.Fields...)
	fields = append(fields, t.InputFields...)

	// Collect the nested types from InputFields and Fields.
	for _, i := range fields {
		if i.Type.OfType != nil {
			log.Tracef("field %s of type %s", i.GetName(), i.Type.OfType.GetName())

			result, err := s.LookupTypeByName(i.Type.OfType.GetTypeName())
			if err != nil {
				log.Error(err)
				continue
			}

			if result != nil {
				// Append the nested type to the result set.
				f = append(f, result)

				// Recursively expand any fields of the nested type
				subExpanded, err := ExpandType(s, result)
				if err != nil {
					log.Error(err)
					continue
				}

				// Append the nested sub-types into the result set.
				if subExpanded != nil {
					f = append(f, *subExpanded...)
				}
			}
		}
	}

	return &f, nil
}

// ExpandTypes receives a set of TypeInfo, which is then expanded to include
// all the nested types from the fields.
func ExpandTypes(s *Schema, types []TypeInfo) (*[]*Type, error) {
	if s == nil {
		return nil, fmt.Errorf("unable to expand types from nil schema")
	}

	var expandedTypes []*Type

	for _, schemaType := range s.Types {
		if schemaType != nil {

			// Match the name of types we've resolve and append them to the list
			if typeNameInTypes(schemaType.GetName(), types) {
				expandedTypes = append(expandedTypes, schemaType)

				fieldTypes, err := ExpandType(s, schemaType)
				if err != nil {
					log.Error(err)
				}

				// Avoid duplicates, append the unique names to the set
				for _, f := range *fieldTypes {
					if !hasType(f, expandedTypes) {
						expandedTypes = append(expandedTypes, f)
					}
				}
			}
		}
	}

	return &expandedTypes, nil
}
