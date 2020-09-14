package schema

import (
	"fmt"
	"regexp"
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
	if t == nil {
		log.Warn("hasType(nil)")
	}

	for _, tt := range types {
		if t.Name == tt.Name {
			return true
		}
	}

	return false
}

// ExpandTypes receives a set of config.TypeConfig, which is then expanded to include
// all the nested types from the fields.
func ExpandTypes(s *Schema, types []config.TypeConfig, methods []config.MethodConfig) (*[]*Type, error) {
	if s == nil {
		return nil, fmt.Errorf("unable to expand types from nil schema")
	}

	var err error
	expander := NewExpander(s)

	for _, schemaType := range s.Types {
		if schemaType != nil {
			// Constrain our handling to include only the type names which are mentioned in the configuration.
			if typeNameInTypes(schemaType.Name, types) {
				log.WithFields(log.Fields{
					"name": schemaType.GetName(),
				}).Debugf("config type: %s", schemaType.Name)

				err = expander.ExpandType(schemaType)
				if err != nil {
					log.Error(err)
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
			err = expander.ExpandTypeFromName(field.Type.Name)
			if err != nil {
				log.WithFields(log.Fields{
					"name":  field.Type.Name,
					"field": field.Name,
				}).Errorf("unable to expand method field type: %s", err)
			}

			for _, methodArg := range field.Args {
				if methodArg.Type.OfType != nil {
					err := expander.ExpandTypeFromName(methodArg.Type.OfType.GetTypeName())
					if err != nil {
						log.WithFields(log.Fields{
							"name": methodArg.Name,
							"type": methodArg.Type,
						}).Errorf("failed to expand method argument: %s", err)
					}
				}
			}
		}
	}

	return expander.ExpandedTypes(), nil
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
		"Guid", "GUID",
		"Nrql", "NRQL",
		"Nrdb", "NRDB",
		"Url", "URL",
		"ApplicationId", "ApplicationID",
	)

	fieldName = r.Replace(fieldName)

	return fieldName
}
