package lang

import (
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/internal/util"
)

// TerraformGenerator holds all state needed to render the terraform templates for a single resource.
type TerraformGenerator struct {
	PackageName      string
	ResourceName     string // "alert_muting_rule"
	ResourceTypeName string // "AlertMutingRule"
	FuncPrefix       string // "resourceNewRelicAlertMutingRule"
	ImportPath       string // "github.com/.../pkg/alerts"
	ClientService    string // "Alerts"

	TerraformConfig *config.TerraformConfig
	SchemaFields    []TerraformField

	CreateMethod string // "AlertsMutingRuleCreateWithContext"
	ReadMethod   string // from config
	UpdateMethod string // empty if no update
	DeleteMethod string

	CreateInputType string // "AlertsMutingRuleCreateInput"
	UpdateInputType string
	ReturnType      string

	// Whether each mutation has a direct accountId arg in NerdGraph.
	// When false, accountID is NOT passed to the go-client method even when RequiresAccountID is true.
	CreateHasAccountIDArg bool
	UpdateHasAccountIDArg bool
	DeleteHasAccountIDArg bool

	HasUpdate bool
	Imports   []string
}

// TerraformField represents a single field in a Terraform schema resource.
type TerraformField struct {
	Name              string
	GoName            string
	GoTypeName        string // NerdGraph INPUT_OBJECT type name, e.g. "MaintenanceWindowConfigScopeInput"
	TFType            string
	Required          bool
	Optional          bool
	Computed          bool
	Sensitive         bool
	ForceNew          bool
	Description       string
	MaxItems          int
	IsNested          bool
	IsListOfPrimitive bool
	NestedFields      []TerraformField
	IsEnum            bool
	EnumValues        []string
	IsPointer         bool
	ConflictsWith     []string
	SkipOnRead        bool
}

// resolveBaseKind walks down through NON_NULL and LIST wrappers to find the named base type.
func resolveBaseKind(t *schema.TypeRef) (schema.Kind, string) {
	if t == nil {
		return "", ""
	}
	if t.Name != "" {
		return t.Kind, t.Name
	}
	if t.OfType != nil {
		return resolveBaseKind(t.OfType)
	}
	return "", ""
}

// BuildTerraformFields builds the TerraformField slice from a named INPUT_OBJECT type in the schema.
func BuildTerraformFields(s *schema.Schema, typeName string) ([]TerraformField, error) {
	t, err := s.LookupTypeByName(typeName)
	if err != nil {
		return nil, err
	}

	var fields []TerraformField
	for _, f := range t.InputFields {
		fields = append(fields, buildTerraformField(s, f))
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	return fields, nil
}

// buildTerraformField converts a single schema.Field to a TerraformField.
func buildTerraformField(s *schema.Schema, field schema.Field) TerraformField {
	tf := TerraformField{
		Name:        util.ToSnakeCase(field.Name),
		GoName:      field.GetName(),
		Description: field.Description,
	}

	typeRef := &field.Type

	if typeRef.IsNonNull() {
		tf.Required = true
	} else {
		tf.Optional = true
		tf.IsPointer = true
	}

	isList := typeRef.IsList()
	baseKind, baseName := resolveBaseKind(typeRef)

	switch baseKind {
	case schema.KindScalar:
		switch baseName {
		case "String", "ID", "GUID":
			tf.TFType = "TypeString"
		case "Int":
			tf.TFType = "TypeInt"
		case "Float":
			tf.TFType = "TypeFloat"
		case "Boolean":
			tf.TFType = "TypeBool"
		default:
			log.Warnf("terraform generator: unknown scalar %q on field %q, defaulting to TypeString", baseName, field.Name)
			tf.TFType = "TypeString"
		}
		if isList {
			tf.TFType = "TypeList"
			tf.IsListOfPrimitive = true
		}

	case schema.KindENUM:
		tf.TFType = "TypeString"
		tf.IsEnum = true
		enumType, err := s.LookupTypeByName(baseName)
		if err == nil {
			for _, v := range enumType.EnumValues {
				tf.EnumValues = append(tf.EnumValues, v.Name)
			}
			sort.Strings(tf.EnumValues)
		}

	case schema.KindInputObject:
		tf.TFType = "TypeList"
		tf.IsNested = true
		tf.GoTypeName = baseName // NerdGraph type name used as the Go return type in expand functions
		if !isList {
			tf.MaxItems = 1
		}
		nestedType, err := s.LookupTypeByName(baseName)
		if err == nil {
			for _, nestedField := range nestedType.InputFields {
				tf.NestedFields = append(tf.NestedFields, buildTerraformField(s, nestedField))
			}
			sort.Slice(tf.NestedFields, func(i, j int) bool {
				return tf.NestedFields[i].Name < tf.NestedFields[j].Name
			})
		}

	default:
		log.Warnf("terraform generator: unhandled kind %q for field %q, defaulting to TypeString", baseKind, field.Name)
		tf.TFType = "TypeString"
	}

	return tf
}

// FindInputObjectArgTypeName returns the type name of the first INPUT_OBJECT arg of a mutation field.
func FindInputObjectArgTypeName(f *schema.Field) string {
	for _, arg := range f.Args {
		baseKind, baseName := resolveBaseKind(&arg.Type)
		if baseKind == schema.KindInputObject {
			return baseName
		}
	}
	return ""
}

// MutationHasAccountIDArg returns true when the mutation has a direct top-level
// accountId (or accountID) argument — meaning the go-client method accepts it as
// a positional parameter. When false the caller should not pass accountID.
func MutationHasAccountIDArg(f *schema.Field) bool {
	if f == nil {
		return false
	}
	for _, arg := range f.Args {
		if strings.EqualFold(arg.Name, "accountId") {
			return true
		}
	}
	return false
}

// DeriveClientService returns the default client service name from an import path.
// e.g. "github.com/.../pkg/alerts" → "Alerts"
func DeriveClientService(importPath string) string {
	if importPath == "" {
		return ""
	}
	parts := strings.Split(importPath, "/")
	last := parts[len(parts)-1]
	caser := cases.Title(language.Und, cases.NoLower)
	return caser.String(last)
}

// DeriveMethodName converts a camelCase mutation name to a Go method name.
// e.g. "alertsMutingRuleCreate" → "AlertsMutingRuleCreateWithContext"
func DeriveMethodName(mutationName string) string {
	if mutationName == "" {
		return ""
	}
	caser := cases.Title(language.Und, cases.NoLower)
	return caser.String(mutationName) + "WithContext"
}

// SnakeToPascal converts a snake_case string to PascalCase.
// e.g. "alert_muting_rule" → "AlertMutingRule"
func SnakeToPascal(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	caser := cases.Title(language.Und, cases.NoLower)
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(caser.String(p))
	}
	return sb.String()
}
