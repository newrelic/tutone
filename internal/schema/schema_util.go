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

// PrefixLineTab adds a \t character to the beginning of each line.
func PrefixLineTab(s string) string {
	var lines []string

	for _, t := range strings.Split(s, "\n") {
		lines = append(lines, fmt.Sprintf("\t%s", t))
	}

	return strings.Join(lines, "\n")
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

// mutationNameInMutations determines if a name is already present in a set of config.MutationConfig.
func mutationNameInMutations(s string, mutations []config.MutationConfig) bool {
	for _, t := range mutations {
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
func ExpandTypes(s *Schema, pkgConfig *config.PackageConfig) (*[]*Type, error) {
	if s == nil {
		return nil, fmt.Errorf("unable to expand types from nil schema")
	}

	if pkgConfig == nil {
		return nil, fmt.Errorf("unable to expand types from nil PackageConfig")
	}

	var err error
	expander := NewExpander(s)

	for _, schemaType := range s.Types {
		if schemaType != nil {
			// Constrain our handling to include only the type names which are mentioned in the configuration.
			if typeNameInTypes(schemaType.Name, pkgConfig.Types) {
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

	var mutationFields []Field
	mutationFields = append(mutationFields, s.MutationType.Fields...)
	mutationFields = append(mutationFields, s.MutationType.InputFields...)

	for _, field := range mutationFields {
		// Constrain our handling to include only the mutation names which are mentioned in the configuration.
		if mutationNameInMutations(field.Name, pkgConfig.Mutations) {
			err = expander.ExpandTypeFromName(field.Type.GetTypeName())
			if err != nil {
				log.WithFields(log.Fields{
					"name":  field.Type.Name,
					"field": field.Name,
				}).Errorf("unable to expand mutation field type: %s", err)
			}

			for _, mutationArg := range field.Args {
				if mutationArg.Type.OfType != nil {
					err := expander.ExpandTypeFromName(mutationArg.Type.OfType.GetTypeName())
					if err != nil {
						log.WithFields(log.Fields{
							"name": mutationArg.Name,
							"type": mutationArg.Type,
						}).Errorf("failed to expand mutation argument: %s", err)
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

func stringInStrings(s string, ss []string) bool {
	for _, sss := range ss {
		if s == sss {
			return true
		}
	}

	return false
}
