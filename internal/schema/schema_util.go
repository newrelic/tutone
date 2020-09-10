package schema

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
)

// filterDescription uses a regex to parse certain data out of the
// description of an item
func filterDescription(description string) string {
	var ret string

	re := regexp.MustCompile(`(?s)(.*)\n---\n`)
	desc := re.FindStringSubmatch(description)

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

// typeNameInTypes determines if a name is already present in a set of config.TypeConfig.
func typeNameInTypes(s string, types []config.TypeConfig) bool {
	for _, t := range types {
		if t.Name == s {
			return true
		}
	}

	return false
}

// methodNameInMethods determines if a name is already present in a set of config.MethodConfig.
func methodNameInMethods(s string, methods []config.MethodConfig) bool {
	for _, t := range methods {
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

	var expandedTypes []*Type

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
	}).Debugf("expanding type %s", t.Name)

	// Collect the nested types from InputFields and Fields.
	for _, i := range fields {

		log.WithFields(log.Fields{
			"name": i.GetName(),
			"type": i.Type,
			"args": i.Args,
		}).Debugf("expanding field %s", i.Name)

		var result *Type
		var err error

		if i.Type.OfType != nil {
			log.WithFields(log.Fields{
				"ofType":     i.Type.OfType.GetName(),
				"ofTypeName": i.Type.OfType.GetTypeName(),
				"ofTypeKind": i.Type.OfType.GetKinds(),
			}).Debug("field ofType")

			result, err = s.LookupTypeByName(i.Type.OfType.GetTypeName())
			if err != nil {
				log.Error(err)
				continue
			}
		} else if i.Type.Kind == KindObject || i.Type.Kind == KindInputObject || i.Type.Kind == KindENUM {
			result, err = s.LookupTypeByName(i.Type.Name)
			if err != nil {
				log.WithFields(log.Fields{
					// "name": i.Type.Name,
					// "kind": i.Type.Kind,
				}).Errorf("failed lookup type name: %s", err)
				continue
			}
		} else {
			log.WithFields(log.Fields{
				"name": i.GetName(),
				"type": i.Type,
			}).Debugf("not expanding %s", i.Name)
		}

		if result != nil {
			log.WithFields(log.Fields{
				"name": result.Name,
				"kind": result.Kind,
			}).Debug("type found for field")

			if !hasType(result, expandedTypes) {
				expandedTypes = append(expandedTypes, result)
			}

			// Avoid recursing forever, since an interface has dependencies that will
			// likely reference the interface.  For all other Kinds, we want to
			// continue to expand.
			if result.Kind != KindInterface {
				processed, err := expandLookupResults(s, result)
				if err != nil {
					log.WithFields(log.Fields{
						// "name": methodArg.Name,
						// "type": methodArg.Type,
					}).Errorf("failed to expand lookup result: %s", err)
				}

				if processed != nil {
					for _, f := range *processed {
						if !hasType(f, expandedTypes) {
							expandedTypes = append(expandedTypes, f)
						}
					}
				}
			} else {
				// KindInterface should also look for possibleTypes
				for _, possibleType := range result.PossibleTypes {
					r, err := s.LookupTypeByName(possibleType.Name)
					if err != nil {
						log.WithFields(log.Fields{
							// "name": i.Type.Name,
							// "kind": i.Type.Kind,
						}).Errorf("failed lookup possibleType name: %s", err)
						continue
					}

					processed, err := expandLookupResults(s, r)
					if err != nil {
						log.WithFields(log.Fields{
							// "name": methodArg.Name,
							// "type": methodArg.Type,
						}).Errorf("failed to expand lookup result: %s", err)
					}

					if processed != nil {
						for _, f := range *processed {
							if !hasType(f, expandedTypes) {
								expandedTypes = append(expandedTypes, f)
							}
						}
					}

				}
			}

		}

	}

	return &expandedTypes, nil
}

// expandTypesFromName is used to operate over the schema, and expand the results based on a type name received as a string.
func expandTypesFromName(s *Schema, name string) (*[]*Type, error) {
	var expandedTypes []*Type

	result, err := s.LookupTypeByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed lookup method argument: %s", err)
	}

	processed, err := expandLookupResults(s, result)
	if err != nil {
		log.WithFields(log.Fields{
			// "name": methodArg.Name,
			// "type": methodArg.Type,
		}).Errorf("failed to process lookup result: %s", err)
	}

	if processed != nil {
		for _, f := range *processed {
			if !hasType(f, expandedTypes) {
				expandedTypes = append(expandedTypes, f)
			}
		}
	}

	return &expandedTypes, nil
}

// ExpandTypes receives a set of config.TypeConfig, which is then expanded to include
// all the nested types from the fields.
func ExpandTypes(s *Schema, types []config.TypeConfig, methods []config.MethodConfig) (*[]*Type, error) {
	if s == nil {
		return nil, fmt.Errorf("unable to expand types from nil schema")
	}

	var expandedTypes []*Type

	for _, schemaType := range s.Types {
		if schemaType != nil {

			// Constrain our handling to include only the type names which are mentioned in the configuration.
			if typeNameInTypes(schemaType.Name, types) {
				log.WithFields(log.Fields{
					"name": schemaType.GetName(),
				}).Debugf("config type: %s", schemaType.Name)
				if !hasType(schemaType, expandedTypes) {
					expandedTypes = append(expandedTypes, schemaType)
				}

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

	var methodFields []Field
	methodFields = append(methodFields, s.MutationType.Fields...)
	methodFields = append(methodFields, s.MutationType.InputFields...)

	for _, field := range methodFields {

		// Constrain our handling to include only the method names which are mentioned in the configuration.
		if methodNameInMethods(field.Name, methods) {
			log.WithFields(log.Fields{
				"name": field.GetName(),
			}).Debugf("config method: %s", field.Name)

			results, err := expandTypesFromName(s, field.Type.Name)
			if err != nil {
				log.WithFields(log.Fields{
					// "name": methodArg.Name,
					// "type": methodArg.Type,
				}).Errorf("failed lookup method argument: %s", err)
				continue
			}

			if results != nil {
				for _, f := range *results {
					if !hasType(f, expandedTypes) {
						expandedTypes = append(expandedTypes, f)
					}
				}
			}

			for _, methodArg := range field.Args {
				log.WithFields(log.Fields{
					"method": field.Name,
					"name":   methodArg.Name,
				}).Debug("argument for method")

				if methodArg.Type.OfType != nil {
					results, err := expandTypesFromName(s, methodArg.Type.OfType.GetTypeName())
					if err != nil {
						log.WithFields(log.Fields{
							"name": methodArg.Name,
							"type": methodArg.Type,
						}).Errorf("failed lookup method argument: %s", err)
						continue
					}

					if results != nil {
						for _, f := range *results {
							if !hasType(f, expandedTypes) {
								expandedTypes = append(expandedTypes, f)
							}
						}
					}
				}

				results, err := expandTypesFromName(s, methodArg.Type.Name)
				if err != nil {
					log.WithFields(log.Fields{
						"name": methodArg.Name,
						"type": methodArg.Type,
					}).Errorf("failed to process lookup result: %s", err)
				}

				if results != nil {
					for _, f := range *results {
						if !hasType(f, expandedTypes) {
							expandedTypes = append(expandedTypes, f)
						}
					}
				}

			}
		}
	}

	sort.SliceStable(expandedTypes, func(i, j int) bool {
		return expandedTypes[i].Name < expandedTypes[j].Name
	})

	for _, expandedType := range expandedTypes {
		log.WithFields(log.Fields{
			"name": expandedType.Name,
			"kind": expandedType.Kind,
		}).Debug("type included")
	}

	return &expandedTypes, nil
}

// expandLookupResults is used to operate over a schema to expand the received type.
func expandLookupResults(s *Schema, result *Type) (*[]*Type, error) {
	if result == nil {
		return nil, fmt.Errorf("unable to process nil result")
	}

	var expandedTypes []*Type

	// Append the nested type to the result set.
	if !hasType(result, expandedTypes) {
		expandedTypes = append(expandedTypes, result)
	}

	// Recursively expand any fields of the nested type
	subExpanded, err := ExpandType(s, result)
	if err != nil {
		return nil, fmt.Errorf("failed to expand type %s: %s", result.Name, err)
	}

	// Append the nested sub-types into the result set.
	if subExpanded != nil {
		for _, f := range *subExpanded {
			if !hasType(f, expandedTypes) {
				expandedTypes = append(expandedTypes, f)
			}
		}
	}
	return &expandedTypes, nil
}

// formatGoName formats a name string using a few special cases for proper capitalization.
func formatGoName(name string) string {
	var fieldName string

	switch strings.ToLower(name) {
	case "ids":
		// special case to avoid the struct field Ids, and prefer IDs instead
		fieldName = "IDs"
	case "id":
		fieldName = "ID"
	case "accountid":
		fieldName = "AccountID"
	case "accountids":
		fieldName = "AccountIDs"
	case "userid":
		fieldName = "UserID"
	case "userids":
		fieldName = "UserIDs"
	case "ingestkeyids":
		fieldName = "IngestKeyIDs"
	case "userkeyids":
		fieldName = "UserKeyIDs"
	case "keyid":
		fieldName = "KeyID"
	case "policyid":
		fieldName = "PolicyID"
	default:
		fieldName = strings.Title(name)
	}

	r := strings.NewReplacer(
		"Api", "API",
	)

	fieldName = r.Replace(fieldName)

	return fieldName
}
