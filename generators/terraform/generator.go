package terraform

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/output"
	"github.com/newrelic/tutone/internal/schema"
)

// Generator implements codegen.Generator for Terraform provider resources.
type Generator struct {
	TerraformGenerator
}

// TerraformGenerator holds all template data for a single Terraform resource.
type TerraformGenerator struct {
	// Identity
	PackageName    string
	ResourceName   string // e.g. "alert_muting_rule"
	TFResourceName string // e.g. "newrelic_alert_muting_rule"
	ResourceFunc   string // e.g. "resourceNewRelicAlertMutingRule"
	ExpandFunc     string // e.g. "expandNewRelicAlertMutingRuleCreateInput"
	ExpandUpdateFunc string // e.g. "expandNewRelicAlertMutingRuleUpdateInput"
	FlattenFunc    string // e.g. "flattenNewRelicAlertMutingRule"

	// Client
	ClientPackages     []string
	ClientPackageAlias string // e.g. "alerts"  (last segment of first client_package)
	ClientAccessor     string // e.g. "Alerts"  (TitleCase of alias, used as client.<Accessor>.Method)

	// Type names (qualified with package alias for Go code)
	InputTypeName       string // e.g. "alerts.MutingRuleCreateInput"
	UpdateInputTypeName string // e.g. "alerts.MutingRuleUpdateInput"
	OutputTypeName      string // e.g. "alerts.MutingRule"

	// CRUD
	CreateMethod string
	ReadMethod   string
	UpdateMethod string
	DeleteMethod string
	HasUpdate    bool

	// Read behaviour
	ReadType          string
	IDFields          []string
	IDType            string // "int" | "string"
	RequiresAccountID bool
	RequiresOrgID     bool
	NoUpdateMutation  bool
	BatchCreate       bool
	BatchDelete       bool
	ReadAfterCreate   bool
	ReadRetry         bool
	RetryOnCreate     bool
	RetryTimeoutSec   int

	// Not-found signals
	ReadNotFoundString  string
	ReadNotFoundAsError bool
	ReadDeletedField    string

	// List / filter
	ReadListMethod    string
	ReadFilterType    string
	ReadFilterIDField string
	ReadResultPath    string
	ReadFilterIDPath  string

	// Specialised read types
	ReadEntityType    string
	ReadTraversalPath string

	// Parent-child (R3)
	ParentVerifyMethod string
	ParentIDField      string

	// Two-step create (C4)
	PostCreateUpdateFields []string

	// Cross-field constraints
	ConflictingFields [][]string
	IDFallback        bool

	// Schema fields
	SchemaFields []TerraformSchemaField

	// Build tags for test file
	BuildTags []string

	// HasManualFields is true when at least one field requires a TUTONE:MANUAL stub.
	HasManualFields bool

	// Gap 1: multi-arg create support
	CreateInputVars []CreateInputVar // one per create_inputs entry
	// Pre-resolved ctx-prefixed arg lists for each CRUD client call
	CreateCallArgs []string
	ReadCallArgs   []string
	UpdateCallArgs []string
	DeleteCallArgs []string

	// Website / documentation fields (used by docs.html.markdown.tmpl)
	SidebarCurrent string // e.g. "docs-newrelic-resource-alert-muting-rule"
	ResourceTitle  string // e.g. "alert muting rule"
	ImportIDFormat string // e.g. "<account_id>:<id>" or "<id>"
	ImportExample  string // e.g. "12345678:67890" or "example_id"
	WebsiteDir     string // base path for website output, defaults to "website"
}

// CreateInputVar is the runtime representation of one CreateInputConfig entry.
type CreateInputVar struct {
	ArgName    string // GraphQL arg name, e.g. "pathpoint"
	VarName    string // Go variable name, e.g. "pathpointInput"
	ExpandFunc string // e.g. "expandNewRelicPathpointFlowCreateInput"
	Type       string // qualified Go type, e.g. "pathpoint.PathPointFlowInput"
	Source     string // "nested_block" | "flat"
	// SchemaFields holds the fields for the secondary input type so the structures
	// template can emit a fully-generated expand function for this arg.
	// Only populated for indices 1+ (the primary expand is g.SchemaFields).
	SchemaFields []TerraformSchemaField
}

// TerraformSchemaField is one attribute in the generated schema.Schema map.
type TerraformSchemaField struct {
	Name            string // snake_case Terraform attribute name
	GoFieldName     string // PascalCase Go struct field name
	TFType          string // schema.TypeString | TypeInt | TypeBool | TypeFloat | TypeList | TypeSet
	GoTypeAssertion string // .(string) | .(int) | .(bool) | .(float64)
	Required        bool
	Optional        bool
	Computed        bool
	ForceNew        bool
	Sensitive       bool
	Description     string
	IsNested        bool
	NestedFields    []TerraformSchemaField
	GoNestedTypeName string
	MaxItems        int
	IsEnum          bool
	IsEnumList      bool
	EnumValues      []string
	EnumGoType      string
	ConflictsWith   []string
	// Gap 2: pointer field — use &expanded in expand, nil-guard in flatten
	IsPointer bool
	// Gap 3: custom scalar — non-empty means fully generated using these expressions
	CustomExpand  string // Go expand expression, $VALUE replaced with d.GetOk result
	CustomFlatten string // Go flatten expression, $FIELD replaced with result.FieldName
	// NeedsManual: true only when no CustomScalarMapping exists for this custom SCALAR
	NeedsManual    bool
	CustomTypeName string
}

func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	if pkgConfig.Terraform == nil {
		return fmt.Errorf("package %q has no terraform: config block", pkgConfig.Name)
	}

	tf := pkgConfig.Terraform

	g.PackageName = pkgConfig.Name
	g.ResourceName = tf.ResourceName
	g.TFResourceName = "newrelic_" + tf.ResourceName
	g.ResourceFunc = toResourceFuncName(g.TFResourceName)
	g.FlattenFunc = toFlattenFuncName(g.TFResourceName)

	g.ClientPackages = tf.ClientPackages
	g.ClientPackageAlias = derivePackageAlias(tf.ClientPackages)
	if tf.ClientAccessor != "" {
		g.ClientAccessor = tf.ClientAccessor
	} else {
		g.ClientAccessor = toClientAccessor(g.ClientPackageAlias)
	}

	g.ReadType = tf.ReadType
	if g.ReadType == "" {
		g.ReadType = "direct"
	}
	g.ReadMethod = tf.ReadMethod
	g.IDFields = tf.IDFields
	g.IDType = tf.IDType
	if g.IDType == "" {
		g.IDType = "int"
	}
	g.RequiresAccountID = tf.RequiresAccountID
	g.RequiresOrgID = tf.RequiresOrgID
	g.NoUpdateMutation = tf.NoUpdateMutation
	g.BatchCreate = tf.BatchCreate
	g.BatchDelete = tf.BatchDelete
	g.ReadAfterCreate = tf.ReadAfterCreate
	g.ReadRetry = tf.ReadRetry
	g.RetryOnCreate = tf.RetryOnCreate
	g.RetryTimeoutSec = tf.RetryTimeoutSec
	g.ReadNotFoundString = tf.ReadNotFoundString
	g.ReadNotFoundAsError = tf.ReadNotFoundAsError
	g.ReadDeletedField = tf.ReadDeletedField
	g.ReadListMethod = tf.ReadListMethod
	g.ReadFilterType = tf.ReadFilterType
	g.ReadFilterIDField = tf.ReadFilterIDField
	g.ReadResultPath = tf.ReadResultPath
	g.ReadFilterIDPath = tf.ReadFilterIDPath
	g.ReadEntityType = tf.ReadEntityType
	g.ReadTraversalPath = tf.ReadTraversalPath
	g.ParentVerifyMethod = tf.ParentVerifyMethod
	g.ParentIDField = tf.ParentIDField
	g.PostCreateUpdateFields = tf.PostCreateUpdateFields
	g.ConflictingFields = tf.ConflictingFields
	g.IDFallback = tf.IDFallback
	g.BuildTags = tf.BuildTags

	mutations := pkgConfig.Mutations

	// CRUD method names — use explicit config values if set, else derive
	if tf.CreateMethod != "" {
		g.CreateMethod = tf.CreateMethod
	} else if len(mutations) >= 1 {
		g.CreateMethod = camelToClientCall(mutations[0].Name)
	}
	if tf.UpdateMethod != "" {
		g.UpdateMethod = tf.UpdateMethod
	} else if len(mutations) >= 2 {
		g.UpdateMethod = camelToClientCall(mutations[1].Name)
	}
	if tf.DeleteMethod != "" {
		g.DeleteMethod = tf.DeleteMethod
	} else if len(mutations) >= 3 {
		g.DeleteMethod = camelToClientCall(mutations[2].Name)
	}
	g.HasUpdate = !tf.NoUpdateMutation && g.UpdateMethod != ""

	// Derive input/output type names from schema
	if len(mutations) >= 1 {
		inputType, outputType := deriveTypes(s, mutations[0].Name)
		if inputType != "" {
			g.InputTypeName = g.ClientPackageAlias + "." + inputType
			g.ExpandFunc = "expandNewRelic" + snakeToCamel(tf.ResourceName) + "CreateInput"
		}
		if outputType != "" {
			g.OutputTypeName = g.ClientPackageAlias + "." + outputType
		}
	}
	if g.HasUpdate && len(mutations) >= 2 {
		_, updateInputType := deriveUpdateInputType(s, mutations[1].Name)
		if updateInputType != "" {
			g.UpdateInputTypeName = g.ClientPackageAlias + "." + updateInputType
			g.ExpandUpdateFunc = "expandNewRelic" + snakeToCamel(tf.ResourceName) + "UpdateInput"
		}
	}

	// Gap 1: build CreateInputVars from create_inputs config
	g.CreateInputVars = buildCreateInputVars(tf.CreateInputs, g.ClientPackageAlias, snakeToCamel(tf.ResourceName))
	// If no explicit create_inputs, use single default input
	if len(g.CreateInputVars) == 0 {
		g.ExpandFunc = "expandNewRelic" + snakeToCamel(tf.ResourceName) + "CreateInput"
	}

	// Populate SchemaFields for secondary create inputs (indices 1+) so the
	// structures template can emit fully-generated expand functions for each arg.
	for i := range g.CreateInputVars {
		if i == 0 {
			continue // primary — already covered by g.SchemaFields
		}
		civ := &g.CreateInputVars[i]
		// Strip package prefix: "pathpoint.PathPointScopeInput" → "PathPointScopeInput"
		typeName := civ.Type
		if dotIdx := strings.LastIndex(typeName, "."); dotIdx >= 0 {
			typeName = typeName[dotIdx+1:]
		}
		t, err := s.LookupTypeByName(typeName)
		if err == nil && t != nil {
			rawFields := t.InputFields
			if len(rawFields) == 0 {
				rawFields = t.Fields
			}
			civ.SchemaFields = buildFieldsFromRaw(s, rawFields, g.ClientPackageAlias,
				map[string]bool{}, map[string]bool{}, map[string]bool{},
				stringSet(tf.PointerFields), tf.CustomScalarMappings)
		}
	}

	// Gap 4: resolve CRUD call arg lists
	g.CreateCallArgs = resolveCallArgs(tf.CRUDArgs, "create", g)
	g.ReadCallArgs = resolveCallArgs(tf.CRUDArgs, "read", g)
	g.UpdateCallArgs = resolveCallArgs(tf.CRUDArgs, "update", g)
	g.DeleteCallArgs = resolveCallArgs(tf.CRUDArgs, "delete", g)

	// Derive automation status after fields are built — used for file-level banner
	g.HasManualFields = false

	// Build schema fields
	if len(mutations) >= 1 {
		computedSet := stringSet(tf.ComputedFields)
		sensitiveSet := stringSet(tf.SensitiveFields)
		immutableSet := stringSet(tf.ImmutableFields)

		pointerSet := stringSet(tf.PointerFields)
		fields, err := g.buildSchemaFields(s, mutations[0].Name, g.ClientPackageAlias,
			computedSet, sensitiveSet, immutableSet, pointerSet, tf.CustomScalarMappings)
		if err != nil {
			log.Warnf("could not derive schema fields for %s: %s", tf.ResourceName, err)
		} else {
			g.SchemaFields = fields
		}
	}

	// Determine overall automation status
	for _, f := range g.SchemaFields {
		if f.NeedsManual || hasManualNested(f) {
			g.HasManualFields = true
			break
		}
	}

	// Distribute ConflictingFields pairs onto individual schema fields
	for _, pair := range tf.ConflictingFields {
		for _, fieldName := range pair {
			// strip nested path prefix (e.g. "schedule.0.end_repeat" → "end_repeat") for matching
			base := fieldName
			if idx := strings.LastIndex(fieldName, "."); idx >= 0 {
				base = fieldName[idx+1:]
			}
			applyConflictsWithDeep(g.SchemaFields, base, pair)
		}
	}

	// Always inject account_id if required and not derived from input type
	if tf.RequiresAccountID && !fieldsContain(g.SchemaFields, "account_id") {
		g.SchemaFields = append(g.SchemaFields, TerraformSchemaField{
			Name:            "account_id",
			GoFieldName:     "AccountID",
			TFType:          "schema.TypeInt",
			GoTypeAssertion: ".(int)",
			Optional:        true,
			Computed:        true,
			Description:     "The New Relic account ID to operate on. Defaults to the account set in the provider.",
		})
	}

	// Website / documentation fields
	g.SidebarCurrent = "docs-newrelic-resource-" + strings.ReplaceAll(g.ResourceName, "_", "-")
	g.ResourceTitle = strings.ReplaceAll(g.ResourceName, "_", " ")
	g.WebsiteDir = tf.WebsiteDir
	if g.WebsiteDir == "" {
		g.WebsiteDir = "website"
	}
	g.ImportIDFormat, g.ImportExample = deriveImportFormat(g.ResourceName, g.IDType, g.IDFields, g.RequiresAccountID)
	if tf.ImportIDFormat != "" {
		g.ImportIDFormat = tf.ImportIDFormat
	}
	if tf.ImportExample != "" {
		g.ImportExample = tf.ImportExample
	}

	return nil
}

func (g *Generator) Execute(genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	destDir := "./"
	if pkgConfig.Path != "" {
		destDir = pkgConfig.Path
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	templateDir := "templates/terraform"
	if genConfig.TemplateDir != "" {
		templateDir = genConfig.TemplateDir
	}

	var generated []string

	// 1. resource_newrelic_<name>.go
	resourceFile := fmt.Sprintf("%s/resource_%s.go", destDir, g.TFResourceName)
	if err := renderTemplate(templateDir, "resource.go.tmpl", resourceFile, destDir, g); err != nil {
		return fmt.Errorf("resource template: %w", err)
	}
	generated = append(generated, resourceFile)

	// 2. structures_newrelic_<name>.go
	structuresFile := fmt.Sprintf("%s/structures_%s.go", destDir, g.TFResourceName)
	if err := renderTemplate(templateDir, "structures.go.tmpl", structuresFile, destDir, g); err != nil {
		return fmt.Errorf("structures template: %w", err)
	}
	generated = append(generated, structuresFile)

	// 3. resource_newrelic_<name>_test.go — scaffold-once (never overwrite)
	testFile := fmt.Sprintf("%s/resource_%s_test.go", destDir, g.TFResourceName)
	if !fileExists(testFile) {
		if err := renderTemplate(templateDir, "test.go.tmpl", testFile, destDir, g); err != nil {
			log.Warnf("test template: %s", err)
		} else {
			generated = append(generated, testFile)
		}
	}

	// 4. provider_registration.txt — copy-paste snippet
	regFile := fmt.Sprintf("%s/provider_registration.txt", destDir)
	if err := appendProviderRegistration(regFile, g.TFResourceName, g.ResourceFunc); err != nil {
		log.Warnf("provider_registration.txt: %s", err)
	} else {
		generated = append(generated, regFile)
	}

	// 5. provider_newrelic.go — auto-patch ResourcesMap if file exists
	providerFile := filepath.Join(destDir, "provider_newrelic.go")
	if fileExists(providerFile) {
		if err := patchProviderFile(providerFile, g.TFResourceName, g.ResourceFunc); err != nil {
			log.Warnf("could not auto-patch provider_newrelic.go: %s", err)
		} else {
			generated = append(generated, providerFile)
		}
	}

	// 6. website/docs/r/<name>.html.markdown — scaffold-once (never overwrite)
	docsDir := filepath.Join(g.WebsiteDir, "docs", "r")
	docFile := filepath.Join(docsDir, g.ResourceName+".html.markdown")
	if !fileExists(docFile) {
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			log.Warnf("could not create %s: %s", docsDir, err)
		} else if err := renderRawTemplate(templateDir, "docs.html.markdown.tmpl", docFile, docsDir, g); err != nil {
			log.Warnf("docs template: %s", err)
		} else {
			generated = append(generated, docFile)
		}
	}

	// 7. website/newrelic.erb — patch @resources array if file exists
	erbFile := filepath.Join(g.WebsiteDir, "newrelic.erb")
	if fileExists(erbFile) {
		if err := patchNavERB(erbFile, g.ResourceName); err != nil {
			log.Warnf("could not patch %s: %s", erbFile, err)
		} else {
			generated = append(generated, erbFile)
		}
	}

	output.PrintSuccessMessage(destDir, generated)
	return nil
}

// ── Schema field derivation ──────────────────────────────────────────────────

// lookupMutationDeep finds a mutation by name searching both the root mutation type and one
// level of namespace objects (e.g. "pathpoint" → pathPointCreate). NerdGraph namespaces most
// mutations under a namespace object on the root mutation type.
func lookupMutationDeep(s *schema.Schema, name string) (*schema.Field, error) {
	// Fast path: top-level mutation (e.g. legacy resources)
	if m, err := s.LookupMutationByName(name); err == nil {
		return m, nil
	}

	// Slow path: search one level of namespaces
	for _, ns := range s.MutationType.Fields {
		nsType, err := s.LookupTypeByName(ns.Type.GetTypeName())
		if err != nil || nsType == nil {
			continue
		}
		for i, f := range nsType.Fields {
			if f.Name == name {
				return &nsType.Fields[i], nil
			}
		}
	}
	return nil, fmt.Errorf("mutation %q not found at root or one namespace level deep", name)
}

// buildSchemaFields derives TerraformSchemaField list from the create mutation's input type.
func (g *Generator) buildSchemaFields(
	s *schema.Schema,
	createMutationName string,
	pkgAlias string,
	computedSet, sensitiveSet, immutableSet, pointerSet map[string]bool,
	scalarMappings map[string]config.ScalarMapping,
) ([]TerraformSchemaField, error) {
	mutation, err := lookupMutationDeep(s, createMutationName)
	if err != nil {
		return nil, fmt.Errorf("mutation %q: %w", createMutationName, err)
	}

	inputTypeName := ""
	for _, arg := range mutation.Args {
		n := strings.ToLower(arg.Name)
		if n == "accountid" || n == "orgid" || n == "organizationid" {
			continue
		}
		inputTypeName = arg.Type.GetTypeName()
		break
	}
	if inputTypeName == "" {
		return nil, nil
	}

	inputType, err := s.LookupTypeByName(inputTypeName)
	if err != nil {
		return nil, fmt.Errorf("input type %q: %w", inputTypeName, err)
	}

	rawFields := inputType.InputFields
	if len(rawFields) == 0 {
		rawFields = inputType.Fields
	}

	return buildFieldsFromRaw(s, rawFields, pkgAlias, computedSet, sensitiveSet, immutableSet, pointerSet, scalarMappings), nil
}

func buildFieldsFromRaw(
	s *schema.Schema,
	rawFields []schema.Field,
	pkgAlias string,
	computedSet, sensitiveSet, immutableSet, pointerSet map[string]bool,
	scalarMappings map[string]config.ScalarMapping,
) []TerraformSchemaField {
	var fields []TerraformSchemaField

	for _, f := range rawFields {
		typeName := f.Type.GetTypeName()
		isEnum := isEnumField(f, s)
		isEnumList := f.Type.IsList() && isListOfEnum(f, s)
		nested := isNestedType(f.Type) && !isEnumList

		tfType := typeRefToTFType(f.Type, isEnum, isEnumList, nested)
		goTypeAssertion := tfTypeToGoAssertion(tfType)

		// Issue 6: snake_case attribute names (GraphQL is camelCase, Terraform is snake_case)
		snakeName := camelToSnake(f.Name)
		// GoFieldName stays as PascalCase for Go struct access
		goFieldName := upperFirst(f.Name)

		// Issue 12: strip internal content from descriptions
		desc := filterDescription(strings.TrimSpace(f.Description))

		tf := TerraformSchemaField{
			Name:            snakeName,
			GoFieldName:     goFieldName,
			TFType:          tfType,
			GoTypeAssertion: goTypeAssertion,
			Description:     desc,
			IsNested:        nested,
			IsEnum:          isEnum && !f.Type.IsList(),
			IsEnumList:      isEnumList,
		}

		// Issue 7: TypeSet for unordered enum lists; MaxItems:1 for single nested objects
		if nested && !f.Type.IsList() {
			tf.MaxItems = 1
		}

		// Enum handling (scalar enum)
		if isEnum && !f.Type.IsList() {
			tf.EnumValues = lookupEnumValues(s, typeName)
			tf.EnumGoType = pkgAlias + "." + typeName
			tf.GoTypeAssertion = ".(string)"
		}
		// Enum list
		if isEnumList {
			tf.EnumValues = lookupEnumValues(s, typeName)
			tf.EnumGoType = pkgAlias + "." + typeName
		}

		// Computed / Optional / Required — use snake name for lookup
		if computedSet[snakeName] || computedSet[f.Name] {
			tf.Computed = true
			tf.Optional = true
		} else if f.IsRequired() {
			tf.Required = true
		} else {
			tf.Optional = true
		}

		if immutableSet[snakeName] || immutableSet[f.Name] {
			tf.ForceNew = true
		}
		if sensitiveSet[snakeName] || sensitiveSet[f.Name] {
			tf.Sensitive = true
		}

		// Gap 2: pointer field detection
		if pointerSet[snakeName] || pointerSet[f.Name] {
			tf.IsPointer = true
		}

		// Nested: recurse to build child schema + set concrete Go type name
		if nested {
			tf.GoNestedTypeName = pkgAlias + "." + typeName
			nestedType, err := s.LookupTypeByName(typeName)
			if err == nil {
				nestedRaw := nestedType.InputFields
				if len(nestedRaw) == 0 {
					nestedRaw = nestedType.Fields
				}
				tf.NestedFields = buildFieldsFromRaw(s, nestedRaw, pkgAlias,
					map[string]bool{}, map[string]bool{}, map[string]bool{},
					pointerSet, scalarMappings)
				for _, nf := range tf.NestedFields {
					if nf.NeedsManual {
						tf.NeedsManual = true
						break
					}
				}
			}
		}

		// Gap 3: custom scalar — check mapping before marking NeedsManual
		if !nested && !isEnum && !isEnumList && isCustomScalar(f, s) {
			if mapping, ok := scalarMappings[typeName]; ok {
				// Fully generated using the provided mapping expressions
				tf.TFType = mapping.TFType
				tf.GoTypeAssertion = tfTypeToGoAssertion(mapping.TFType)
				tf.CustomExpand = strings.ReplaceAll(mapping.Expand, "$VALUE", "v"+tf.GoTypeAssertion)
				tf.CustomFlatten = mapping.Flatten // $FIELD substituted in template
			} else {
				// No mapping provided — still needs manual
				tf.NeedsManual = true
				tf.CustomTypeName = typeName
				tf.TFType = "schema.TypeString"
				tf.GoTypeAssertion = ".(string)"
			}
		}

		fields = append(fields, tf)
	}

	return fields
}

func hasManualNested(f TerraformSchemaField) bool {
	for _, nf := range f.NestedFields {
		if nf.NeedsManual || hasManualNested(nf) {
			return true
		}
	}
	return false
}

// applyConflictsWithDeep walks the field tree and adds the full conflict pair list to the
// field whose base name matches targetBase, excluding itself from its own ConflictsWith list.
func applyConflictsWithDeep(fields []TerraformSchemaField, targetBase string, pair []string) {
	for i := range fields {
		if fields[i].Name == targetBase {
			others := make([]string, 0, len(pair)-1)
			for _, p := range pair {
				base := p
				if idx := strings.LastIndex(p, "."); idx >= 0 {
					base = p[idx+1:]
				}
				if base != targetBase {
					others = append(others, p)
				}
			}
			fields[i].ConflictsWith = others
		}
		if len(fields[i].NestedFields) > 0 {
			applyConflictsWithDeep(fields[i].NestedFields, targetBase, pair)
		}
	}
}

// isCustomScalar returns true for SCALAR types that aren't the five standard GraphQL scalars.
// These need hand-written time.Parse / strconv / custom marshalling.
func isCustomScalar(f schema.Field, s *schema.Schema) bool {
	typeName := f.Type.GetTypeName()
	// Standard scalars — fully automatable
	switch typeName {
	case "String", "Int", "Float", "Boolean", "ID":
		return false
	}
	t, err := s.LookupTypeByName(typeName)
	if err != nil {
		return false
	}
	return t.Kind == schema.KindScalar
}

// ── Type helpers ─────────────────────────────────────────────────────────────

func typeRefToTFType(ref schema.TypeRef, isEnum, isEnumList, nested bool) string {
	if isEnumList {
		// Issue 7: unordered enum arrays → TypeSet
		return "schema.TypeSet"
	}
	if ref.IsList() {
		return "schema.TypeList"
	}
	switch ref.GetTypeName() {
	case "Int":
		return "schema.TypeInt"
	case "Float":
		return "schema.TypeFloat"
	case "Boolean":
		return "schema.TypeBool"
	case "String", "ID":
		return "schema.TypeString"
	default:
		if nested {
			return "schema.TypeList"
		}
		return "schema.TypeString"
	}
}

// isListOfEnum returns true when a LIST field's element type is an ENUM.
func isListOfEnum(f schema.Field, s *schema.Schema) bool {
	if !f.Type.IsList() {
		return false
	}
	typeName := f.Type.GetTypeName()
	t, err := s.LookupTypeByName(typeName)
	if err != nil || t == nil {
		return false
	}
	return t.Kind == schema.KindENUM
}

// camelToSnake converts "actionOnMutingRuleWindowEnded" → "action_on_muting_rule_window_ended"
func camelToSnake(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && !unicode.IsUpper(runes[i-1]) {
				result = append(result, '_')
			} else if i > 0 && unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// filterDescription strips NR-internal content from field descriptions.
// Anything after "---\n**NR Internal" or "---\n**Internal" is dropped.
// Slack URLs, issue tracker links and team IDs should not appear in public Terraform docs.
var (
	internalMarker  = regexp.MustCompile(`(?s)\s*---\s*\*{0,2}(NR\s+)?[Ii]nternal.*$`)
	slackURLPattern = regexp.MustCompile(`https?://newrelic\.slack\.com\S*`)
	issueURLPattern = regexp.MustCompile(`https?://[a-z0-9-]+\.atlassian\.net\S*`)
)

func filterDescription(desc string) string {
	desc = internalMarker.ReplaceAllString(desc, "")
	desc = slackURLPattern.ReplaceAllString(desc, "")
	desc = issueURLPattern.ReplaceAllString(desc, "")
	return strings.TrimSpace(desc)
}

func tfTypeToGoAssertion(tfType string) string {
	switch tfType {
	case "schema.TypeInt":
		return ".(int)"
	case "schema.TypeBool":
		return ".(bool)"
	case "schema.TypeFloat":
		return ".(float64)"
	default:
		return ".(string)"
	}
}

func isNestedType(ref schema.TypeRef) bool {
	if ref.IsList() {
		inner := ref.GetTypeName()
		switch inner {
		case "String", "Int", "Float", "Boolean", "ID":
			return false
		}
		for _, k := range ref.GetKinds() {
			if k == schema.KindInputObject || k == schema.KindObject || k == schema.KindInterface {
				return true
			}
		}
	}
	for _, k := range ref.GetKinds() {
		if k == schema.KindInputObject || k == schema.KindObject || k == schema.KindInterface {
			return true
		}
	}
	return false
}

func isEnumField(f schema.Field, s *schema.Schema) bool {
	if f.IsEnum() {
		return true
	}
	typeName := f.Type.GetTypeName()
	t, err := s.LookupTypeByName(typeName)
	if err != nil {
		return false
	}
	return t.Kind == schema.KindENUM
}

func lookupEnumValues(s *schema.Schema, typeName string) []string {
	t, err := s.LookupTypeByName(typeName)
	if err != nil || t == nil {
		return nil
	}
	vals := make([]string, 0, len(t.EnumValues))
	for _, ev := range t.EnumValues {
		vals = append(vals, ev.Name)
	}
	return vals
}

// deriveTypes returns (inputTypeName, outputTypeName) from a mutation.
func deriveTypes(s *schema.Schema, mutationName string) (string, string) {
	mutation, err := s.LookupMutationByName(mutationName)
	if err != nil {
		return "", ""
	}
	var inputType string
	for _, arg := range mutation.Args {
		n := strings.ToLower(arg.Name)
		if n == "accountid" || n == "orgid" || n == "organizationid" {
			continue
		}
		inputType = arg.Type.GetTypeName()
		break
	}
	outputType := mutation.Type.GetTypeName()
	return inputType, outputType
}

// deriveUpdateInputType returns the update mutation's input type.
func deriveUpdateInputType(s *schema.Schema, mutationName string) (string, string) {
	return deriveTypes(s, mutationName)
}

// ── Name helpers ─────────────────────────────────────────────────────────────

func toResourceFuncName(tfName string) string {
	return "resource" + snakeToCamel(tfName)
}

func toFlattenFuncName(tfName string) string {
	return "flattenNewRelic" + snakeToCamel(strings.TrimPrefix(tfName, "newrelic_"))
}

// snakeToCamel: "newrelic_alert_muting_rule" → "NewRelicAlertMutingRule"
// Handles compound proper nouns: "newrelic" → "NewRelic".
func snakeToCamel(s string) string {
	// Expand known compound segments before splitting
	s = strings.ReplaceAll(s, "newrelic", "new_relic")
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// camelToClientCall: "alertsMutingRuleCreate" → "AlertsMutingRuleCreateWithContext"
func camelToClientCall(mutationName string) string {
	if mutationName == "" {
		return ""
	}
	return strings.ToUpper(mutationName[:1]) + mutationName[1:] + "WithContext"
}

// derivePackageAlias returns "alerts" from "github.com/.../pkg/alerts"
func derivePackageAlias(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	parts := strings.Split(packages[0], "/")
	return parts[len(parts)-1]
}

// toClientAccessor: "alerts" → "Alerts"
func toClientAccessor(alias string) string {
	return upperFirst(alias)
}

// ── Provider file patching ───────────────────────────────────────────────────

// patchProviderFile inserts a ResourcesMap entry into provider_newrelic.go if not already present.
func patchProviderFile(providerFile, tfName, funcName string) error {
	content, err := os.ReadFile(providerFile)
	if err != nil {
		return err
	}

	line := fmt.Sprintf("\t\t\t\"%s\":%s%s(),", tfName, "\t\t\t\t\t\t\t\t", funcName)
	// Simpler alignment
	line = fmt.Sprintf("\t\t\t\"%s\": %s(),", tfName, funcName)

	if strings.Contains(string(content), `"`+tfName+`"`) {
		log.Infof("provider_newrelic.go already contains %q — skipping patch", tfName)
		return nil
	}

	anchor := "ResourcesMap: map[string]*schema.Resource{"
	if !strings.Contains(string(content), anchor) {
		return fmt.Errorf("could not find ResourcesMap anchor in %s", providerFile)
	}

	// Insert alphabetically after the opening brace
	patched := insertAlphabetically(string(content), anchor, line, tfName)
	return os.WriteFile(providerFile, []byte(patched), 0644)
}

// insertAlphabetically finds the right position inside the ResourcesMap block.
func insertAlphabetically(content, anchor, newLine, tfName string) string {
	lines := strings.Split(content, "\n")
	anchorIdx := -1
	for i, l := range lines {
		if strings.Contains(l, anchor) {
			anchorIdx = i
			break
		}
	}
	if anchorIdx < 0 {
		return content
	}

	insertAt := anchorIdx + 1
	for i := anchorIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || trimmed == "}" || trimmed == "}," {
			break
		}
		// Extract the key from lines like `"newrelic_foo": ...`
		if strings.HasPrefix(trimmed, `"`) {
			end := strings.Index(trimmed[1:], `"`)
			if end >= 0 {
				existingKey := trimmed[1 : end+1]
				if existingKey < tfName {
					insertAt = i + 1
				} else {
					break
				}
			}
		}
	}

	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertAt]...)
	result = append(result, newLine)
	result = append(result, lines[insertAt:]...)
	return strings.Join(result, "\n")
}

func appendProviderRegistration(regFile, tfName, funcName string) error {
	// Read existing to avoid duplicates
	if existing, err := os.ReadFile(regFile); err == nil {
		if strings.Contains(string(existing), `"`+tfName+`"`) {
			return nil
		}
	}
	line := fmt.Sprintf("\"%s\": %s(),\n", tfName, funcName)
	f, err := os.OpenFile(regFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// ── Utilities ────────────────────────────────────────────────────────────────

func stringSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func fieldsContain(fields []TerraformSchemaField, name string) bool {
	for _, f := range fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func renderTemplate(templateDir, templateName, destFile, destDir string, g *Generator) error {
	c := codegen.CodeGen{
		TemplateDir:     templateDir,
		TemplateName:    templateName,
		DestinationFile: destFile,
		DestinationDir:  destDir,
	}
	return c.WriteFile(g)
}

// ── Gap 1: CreateInputVar helpers ────────────────────────────────────────────

func buildCreateInputVars(inputs []config.CreateInputConfig, pkgAlias, resourceCamel string) []CreateInputVar {
	vars := make([]CreateInputVar, 0, len(inputs))
	for i, inp := range inputs {
		varName := camelToSnake(inp.Arg) // arg name → snake for var prefix
		varName = strings.ReplaceAll(varName, "_", "") + "Input"
		// First input's expand function is the primary ExpandFunc
		expandFunc := "expandNewRelic" + resourceCamel
		if i == 0 {
			expandFunc += "CreateInput"
		} else {
			expandFunc += upperFirst(inp.Arg) + "Input"
		}
		vars = append(vars, CreateInputVar{
			ArgName:    inp.Arg,
			VarName:    varName,
			ExpandFunc: expandFunc,
			Type:       pkgAlias + "." + inp.Type,
			Source:     inp.Source,
		})
	}
	return vars
}

// ── Gap 4: CRUDArgs resolution ───────────────────────────────────────────────

// resolveCallArgs builds the argument list for a CRUD go-client call.
// ctx is prepended only when the method name ends with "WithContext"; non-context
// go-client methods accept no ctx parameter (they call context.Background() internally).
func resolveCallArgs(crud *config.CRUDArgsConfig, op string, g *Generator) []string {
	var tokens []string

	if crud != nil {
		switch op {
		case "create":
			tokens = crud.Create
		case "read":
			tokens = crud.Read
		case "update":
			tokens = crud.Update
		case "delete":
			tokens = crud.Delete
		}
	}

	// Default token lists when crud_args not specified
	if len(tokens) == 0 {
		switch op {
		case "create":
			if g.RequiresAccountID {
				tokens = append(tokens, "account_id")
			}
			if g.RequiresOrgID {
				tokens = append(tokens, "org_id")
			}
			for _, civ := range g.CreateInputVars {
				tokens = append(tokens, civ.ArgName)
			}
			if len(g.CreateInputVars) == 0 {
				tokens = append(tokens, "input")
			}
		case "read":
			if g.RequiresAccountID {
				tokens = append(tokens, "account_id")
			}
			tokens = append(tokens, "id")
		case "update":
			if g.RequiresAccountID {
				tokens = append(tokens, "account_id")
			}
			tokens = append(tokens, "id", "input")
		case "delete":
			if g.RequiresAccountID {
				tokens = append(tokens, "account_id")
			}
			tokens = append(tokens, "id")
		}
	}

	// Prepend ctx only for WithContext go-client methods; the non-context variants
	// call context.Background() internally and have no ctx parameter.
	methodName := ""
	switch op {
	case "create":
		methodName = g.CreateMethod
	case "read":
		methodName = g.ReadMethod
	case "update":
		methodName = g.UpdateMethod
	case "delete":
		methodName = g.DeleteMethod
	}

	var args []string
	if strings.HasSuffix(methodName, "WithContext") {
		args = []string{"ctx"}
	}
	for _, tok := range tokens {
		args = append(args, tokenToGoVar(tok, g))
	}
	return args
}

// tokenToGoVar maps a config token to its Go variable name.
func tokenToGoVar(tok string, g *Generator) string {
	switch tok {
	case "account_id":
		return "accountID"
	case "org_id":
		return "orgID"
	case "id":
		return "id"
	case "input":
		return "input"
	default:
		// Check create input vars by arg name
		for _, civ := range g.CreateInputVars {
			if civ.ArgName == tok {
				return civ.VarName
			}
		}
		return tok
	}
}

// scanLines is a helper used in tests.
func scanLines(s string) []string {
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

var _ = scanLines // suppress unused warning; used in tests

// ── Website / documentation helpers ─────────────────────────────────────────

// deriveImportFormat returns the import ID format string and an example value.
func deriveImportFormat(resourceName, idType string, idFields []string, requiresAccountID bool) (format, example string) {
	if idType == "int" || (requiresAccountID && len(idFields) > 0) {
		resourceID := resourceName + "_id"
		if len(idFields) > 0 {
			resourceID = idFields[len(idFields)-1]
		}
		return "<account_id>:<" + resourceID + ">", "12345678:67890"
	}
	if len(idFields) > 1 {
		parts := make([]string, len(idFields))
		for i, f := range idFields {
			parts[i] = "<" + f + ">"
		}
		return strings.Join(parts, ":"), strings.Repeat("example:", len(idFields)-1) + "id"
	}
	return "<id>", "example_id"
}

// renderRawTemplate renders a template file without running goimports (for non-Go files).
func renderRawTemplate(templateDir, templateName, destFile, destDir string, g *Generator) error {
	c := codegen.CodeGen{
		TemplateDir:     templateDir,
		TemplateName:    templateName,
		DestinationFile: destFile,
		DestinationDir:  destDir,
	}
	return c.WriteRawFile(g)
}

// patchNavERB inserts resourceName into the @resources array in website/newrelic.erb
// in alphabetical order. No-ops if the entry already exists.
func patchNavERB(erbFile, resourceName string) error {
	data, err := os.ReadFile(erbFile)
	if err != nil {
		return err
	}
	content := string(data)

	if strings.Contains(content, `"`+resourceName+`"`) {
		log.Infof("%s already contains %q — skipping", erbFile, resourceName)
		return nil
	}

	startMarker := "@resources = ["
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return fmt.Errorf("could not find @resources array in %s", erbFile)
	}

	bracketStart := strings.Index(content[startIdx:], "[") + startIdx
	bracketEnd := strings.Index(content[bracketStart:], "]") + bracketStart
	if bracketEnd <= bracketStart {
		return fmt.Errorf("malformed @resources array in %s", erbFile)
	}

	arrayContent := content[bracketStart+1 : bracketEnd]
	var resources []string
	for _, line := range strings.Split(arrayContent, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimRight(line, ",")
		line = strings.Trim(line, `"`)
		if line != "" {
			resources = append(resources, line)
		}
	}

	insertPos := len(resources)
	for i, r := range resources {
		if r > resourceName {
			insertPos = i
			break
		}
	}

	newResources := make([]string, 0, len(resources)+1)
	newResources = append(newResources, resources[:insertPos]...)
	newResources = append(newResources, resourceName)
	newResources = append(newResources, resources[insertPos:]...)

	var newArray strings.Builder
	for _, r := range newResources {
		newArray.WriteString("\n    \"" + r + "\",")
	}
	newArray.WriteString("\n")

	newContent := content[:bracketStart+1] + newArray.String() + content[bracketEnd:]
	return os.WriteFile(erbFile, []byte(newContent), 0644)
}
