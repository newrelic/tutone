package schema

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
)

// Field is an attribute of a schema Type object.
type Field struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Kind        Kind   `json:"kind,omitempty"`

	Type         TypeRef     `json:"type"`
	Args         []Field     `json:"args,omitempty"`
	DefaultValue interface{} `json:"defaultValue,omitempty"`
}

// GetDescription formats the description into a GoDoc comment.
func (f *Field) GetDescription() string {
	if strings.TrimSpace(f.Description) == "" {
		return ""
	}

	return formatDescription("", f.Description)
}

// GetTypeNameWithOverride returns the typeName, taking into consideration any FieldTypeOverride specified in the PackageConfig.
func (f *Field) GetTypeNameWithOverride(pkgConfig *config.PackageConfig) (string, error) {
	var typeName string
	var overrideType string
	var err error

	// Discover any FieldTypeOverride override for the current field.
	nameToMatch := f.Type.GetTypeName()

	for _, p := range pkgConfig.Types {
		if p.Name == nameToMatch {
			if p.FieldTypeOverride != "" {
				log.WithFields(log.Fields{
					"name":                nameToMatch,
					"field_type_override": p.FieldTypeOverride,
				}).Trace("overriding typeref")
				overrideType = p.FieldTypeOverride
			}
		}
	}

	// Set the typeName to the override or use what is specified in the schema.
	if overrideType != "" {
		typeName = overrideType
	} else {
		typeName, _, err = f.Type.GetType()
		if err != nil {
			return "", err
		}
	}

	return typeName, nil
}

// GetName returns a recusive lookup of the type name
func (f *Field) GetName() string {
	return formatGoName(f.Name)
}

func (f *Field) HasPointerOverride(pkgConfig *config.PackageConfig) bool {
	var hasPointerOverride bool

	nameToMatch := f.Type.GetTypeName()
	for _, p := range pkgConfig.Types {
		if p.Name == nameToMatch {
			hasPointerOverride = p.CreateAsPointer
		}
	}

	return hasPointerOverride
}

func (f *Field) GetTagsWithOverrides(parentType Type, pkgConfig *config.PackageConfig) string {
	if f == nil {
		return ""
	}

	// Get the parent type config to apply any field struct tag overrides
	parentTypeConfig := pkgConfig.GetTypeConfigByName(parentType.Name)

	var tags string
	if parentTypeConfig != nil && len(parentTypeConfig.StructTags) > 0 {
		tags = f.buildStructTags(f.Name, parentTypeConfig.StructTags)
	}

	if tags == "" {
		return f.GetTags()
	}

	return tags
}

func (f *Field) buildStructTags(fieldName string, structTags []string) string {
	tagsString := "`"
	tagsCount := len(structTags)

	for i, tagType := range structTags {
		tagEnd := "\" "
		if i == tagsCount-1 {
			tagEnd = "\"" // no trailing space if last tag
		}

		tagsString = tagsString + tagType + ":\"" + f.Name

		if f.Type.IsInputObject() || !f.Type.IsNonNull() {
			tagsString = tagsString + ",omitempty"
		}

		tagsString = tagsString + tagEnd
	}

	// Add closing back tick
	tagsString += "`"

	return tagsString
}

// GetTags is used to return the Go struct tags for a field.
func (f *Field) GetTags() string {
	if f == nil {
		return ""
	}

	jsonTag := "`json:\"" + f.Name

	if f.Type.IsInputObject() || !f.Type.IsNonNull() {
		jsonTag += ",omitempty"
	}

	tags := jsonTag + "\"`"

	// log.Print("\n\n **************************** \n")
	// log.Printf("\n Struct Tags:  %s \n", f)
	// log.Printf("\n Struct Tags:  %s \n", jsonTag)
	// log.Print("\n **************************** \n\n")
	// time.Sleep(5 * time.Second)

	return tags
}

func (f *Field) IsPrimitiveType() bool {
	goTypes := []string{
		"int",
		"string",
		"bool",
		"boolean",
	}

	name := strings.ToLower(f.Type.GetTypeName())

	for _, x := range goTypes {
		if x == name {
			return true
		}
	}

	return false
}

// Convenience method that proxies to TypeRef method
func (f *Field) IsScalarID() bool {
	return f.Type.IsScalarID()
}

// Convenience method that proxies to TypeRef method
func (f *Field) IsRequired() bool {
	return f.Type.IsNonNull()
}

func (f *Field) IsEnum() bool {
	if f.Type.Kind == KindENUM {
		return true
	}

	return f.Type.OfType != nil && f.Type.OfType.Kind == KindENUM
}

func (f *Field) HasRequiredArg() bool {
	for _, a := range f.Args {
		if a.IsRequired() {
			return true
		}
	}

	return false
}
