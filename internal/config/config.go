package config

import (
	"errors"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Config is the information keeper for generating go structs from type names.
type Config struct {
	// LogLevel sets the logging level
	LogLevel string `yaml:"log_level,omitempty"`
	// Endpoint is the URL for the GraphQL API
	Endpoint string `yaml:"endpoint"`
	// Auth contains details about how to authenticate to the API in the case that it's required.
	Auth AuthConfig `yaml:"auth"`
	// Cache contains information on how and where to store the schema.
	Cache CacheConfig `yaml:"cache"`
	// Packages contain the information on how to break up the schema into code packages.
	Packages []PackageConfig `yaml:"packages,omitempty"`
	// Generators configure the work engine of this project.
	Generators []GeneratorConfig `yaml:"generators,omitempty"`
}

// AuthConfig is the information necessary to authenticate to the NerdGraph API.
type AuthConfig struct {
	// Header is the name of the API request header that is used to authenticate.
	Header string `yaml:"header,omitempty"`
	// EnvVar is the name of the environment variable to attach to the above header.
	EnvVar string `yaml:"api_key_env_var,omitempty"`
}

// CacheConfig is the information necessary to store the NerdGraph schema in JSON.
type CacheConfig struct {
	// Enable or disable the schema caching.
	Enable bool `yaml:",omitempty"`
	// SchemaFile is the location where the schema should be cached.
	SchemaFile string `yaml:"schema_file,omitempty"`
}

// PackageConfig is the information about a single package, which types to include from the schema, and which generators to use for this package.
type PackageConfig struct {
	// Name is the string that is used to refer to the name of the package.
	Name string `yaml:"name,omitempty"`
	// Path is the relative path within the project.
	Path string `yaml:"path,omitempty"`
	// ImportPath is the full path used for importing this package into a Go project
	ImportPath string `yaml:"import_path,omitempty"`
	// Types is a list of Type configurations to include in the package.
	Types []TypeConfig `yaml:"types,omitempty"`
	// Mutations is a list of Method configurations to include in the package.
	Mutations []MutationConfig `yaml:"mutations,omitempty"`
	// Generators is a list of names that reference a generator in the Config struct.
	Generators []string `yaml:"generators,omitempty"`
	// Imports is a list of strings to represent what pacakges to import for a given package.
	Imports []string `yaml:"imports,omitempty"`

	Commands []Command `yaml:"commands,omitempty"`

	Queries []Query `yaml:"queries,omitempty"`

	// Transient property which is set by using the --include-integration-test flag.
	IncludeIntegrationTest bool

	// Terraform holds the configuration for the terraform generator for this package.
	Terraform *TerraformConfig `yaml:"terraform,omitempty"`
}

// TerraformConfig is the specification for generating a Terraform resource from this package.
type TerraformConfig struct {
	ResourceName string `yaml:"resource_name"`

	// Explicit client-go method names for CRUD — use these because NerdGraph mutation
	// names don't match client-go method names (e.g. alertsMutingRuleCreate → CreateMutingRuleWithContext).
	// If omitted, the generator falls back to camelCase conversion of the mutations[] names.
	CreateMethod string `yaml:"create_method,omitempty"`
	UpdateMethod string `yaml:"update_method,omitempty"`
	DeleteMethod string `yaml:"delete_method,omitempty"`

	ReadMethod string `yaml:"read_method"`

	// id_type controls how d.SetId() and d.Id() are handled.
	// "int"    → serializeIDs([]int{accountID, result.ID}) / parseHashedIDs(d.Id())
	// "string" → d.SetId(result.ID) / id := d.Id()   (default)
	IDType string `yaml:"id_type,omitempty"`
	ReadType          string   `yaml:"read_type,omitempty"`           // direct | list_search | list_filter | guid_collection_search | entity_management | nested_traversal
	IDFields          []string `yaml:"id_fields,omitempty"`           // fields that form the Terraform resource ID
	RequiresAccountID bool     `yaml:"requires_account_id,omitempty"` // inject account_id into CRUD calls
	RequiresOrgID     bool     `yaml:"requires_org_id,omitempty"`     // pre-fetch org ID before CRUD calls
	ComputedFields    []string `yaml:"computed_fields,omitempty"`     // API-set fields (Computed: true)
	SensitiveFields   []string `yaml:"sensitive_fields,omitempty"`    // fields with Sensitive: true
	ImmutableFields   []string `yaml:"immutable_fields,omitempty"`    // fields with ForceNew: true
	SkipSetOnRead     []string `yaml:"skip_set_on_read,omitempty"`    // fields to omit from d.Set() in Read
	ReadAfterCreate   bool     `yaml:"read_after_create,omitempty"`   // call Read at end of Create
	NoUpdateMutation  bool     `yaml:"no_update_mutation,omitempty"`  // no Update exists; all non-computed fields become ForceNew
	BatchCreate       bool     `yaml:"batch_create,omitempty"`        // create mutation takes a slice input
	BatchDelete       bool     `yaml:"batch_delete,omitempty"`        // delete mutation takes a slice input

	// Not-found signal variants (see R1–R18 matrix)
	ReadNotFoundString  string `yaml:"read_not_found_string,omitempty"`   // string-match not-found (R12)
	ReadNotFoundAsError bool   `yaml:"read_not_found_as_error,omitempty"` // treat explicit error as not-found (R6)
	ReadDeletedField    string `yaml:"read_deleted_field,omitempty"`      // soft-delete field name (R4)

	// List / filter read variants
	ReadListMethod    string `yaml:"read_list_method,omitempty"`    // method for list_search (R4)
	ReadFilterType    string `yaml:"read_filter_type,omitempty"`    // filter struct type for list_filter (R5)
	ReadFilterIDField string `yaml:"read_filter_id_field,omitempty"` // field name on filter struct (R5)
	ReadResultPath    string `yaml:"read_result_path,omitempty"`    // dot-path to result within response (R5)
	ReadFilterIDPath  string `yaml:"read_filter_id_path,omitempty"` // nested path for ID in filter (R9)

	// Specialised read types
	ReadEntityType    string `yaml:"read_entity_type,omitempty"`    // entity management type assertion (R7)
	ReadTraversalPath string `yaml:"read_traversal_path,omitempty"` // dot-path for nested collection traversal (R16)

	// Retry
	ReadRetry       bool `yaml:"read_retry,omitempty"`        // retry read until non-nil / field populated (R10b)
	RetryOnCreate   bool `yaml:"retry_on_create,omitempty"`   // poll after create for eventual consistency (C5)
	RetryTimeoutSec int  `yaml:"retry_timeout_sec,omitempty"` // timeout for retry loops

	// Parent-child verification (R3)
	ParentVerifyMethod string `yaml:"parent_verify_method,omitempty"`
	ParentIDField      string `yaml:"parent_id_field,omitempty"`

	// Two-step create: create + immediate update (C4)
	PostCreateUpdateFields []string `yaml:"post_create_update_fields,omitempty"`

	// Cross-field constraints (U5a)
	ConflictingFields [][]string `yaml:"conflicting_fields,omitempty"` // [[fieldA, fieldB], ...]

	// Composite ID fallback (R2)
	IDFallback bool `yaml:"id_fallback,omitempty"`

	// Build tags emitted on the generated test file
	BuildTags []string `yaml:"build_tags,omitempty"`

	// Client packages to import in the generated file
	ClientPackages []string `yaml:"client_packages,omitempty"`

	// DataSource, when non-nil, also generates a data_source_newrelic_<name>.go file.
	DataSource *DataSourceConfig `yaml:"data_source,omitempty"`

	// IDResultField is the field name on the create/update result struct used for d.SetId().
	// Defaults to "ID". Set to "GUID" (or similar) when the result uses a different field.
	IDResultField string `yaml:"id_result_field,omitempty"`

	// IDCastType is the Go type to cast d.Id() to before passing it to go-client methods.
	// Needed when the go-client uses a named string type such as pathpoint.EntityGUID.
	// Example: "pathpoint.EntityGUID"
	IDCastType string `yaml:"id_cast_type,omitempty"`

	// ClientAccessor overrides the go-client field accessor derived from the package alias
	// (upperFirst(lastSegment(clientPackages[0]))). Set this when the field name in
	// newrelic.NewClient differs from the derived name — e.g. pkg/pathpoint yields alias
	// "pathpoint" → derived "Pathpoint", but the actual field is "PathPoint".
	ClientAccessor string `yaml:"client_accessor,omitempty"`

	// Gap 1: multi-arg create (e.g. pathpoint takes both PathPointFlowInput + PathPointScopeInput)
	CreateInputs []CreateInputConfig `yaml:"create_inputs,omitempty"`

	// Gap 2: fields that use pointer types in client-go structs (*T instead of T)
	PointerFields []string `yaml:"pointer_fields,omitempty"`

	// Gap 3: custom GraphQL scalar types that need domain-specific expand/flatten logic
	CustomScalarMappings map[string]ScalarMapping `yaml:"custom_scalar_mappings,omitempty"`

	// Gap 4: explicit argument lists per CRUD operation — overrides default accountID+id pattern
	CRUDArgs *CRUDArgsConfig `yaml:"crud_args,omitempty"`

	// WebsiteDir is the path to the provider's website directory relative to CWD.
	// Defaults to "website". Used for generating docs/r/<name>.html.markdown and
	// patching newrelic.erb.
	WebsiteDir string `yaml:"website_dir,omitempty"`

	// ProductMappingTag is the value written to integration_test_mappings.yaml for
	// this resource (e.g. "PATHPOINTS", "ALERTS"). Used by the Jenkins generate script.
	// If blank, the script requires PRODUCT_MAPPING_TAG to be set as an env var.
	ProductMappingTag string `yaml:"product_mapping_tag,omitempty"`

	// ImportIDFormat overrides the auto-derived import ID format string shown in
	// the generated docs (e.g. "<id>" for GUID-based resources instead of
	// "<account_id>:<id>"). Leave blank to use the derived value.
	ImportIDFormat string `yaml:"import_id_format,omitempty"`

	// ImportExample overrides the auto-derived import example value shown in
	// the generated docs. Leave blank to use the derived value.
	ImportExample string `yaml:"import_example,omitempty"`
}

// CreateInputConfig describes one argument to a multi-arg create mutation.
type CreateInputConfig struct {
	// Arg is the GraphQL mutation argument name (e.g. "pathpoint", "scope")
	Arg string `yaml:"arg"`
	// Type is the Go type name without package prefix (e.g. "PathPointFlowInput")
	Type string `yaml:"type"`
	// Source controls how the Terraform schema maps to this arg:
	//   "nested_block" (default) — sourced from a TypeList block named after Arg
	//   "flat"                   — sourced directly from top-level d.GetOk() fields
	Source string `yaml:"source,omitempty"`
}

// DataSourceConfig controls data source generation for a package.
type DataSourceConfig struct {
	// LookupFields are the schema attribute names the user provides to identify
	// the entity (marked Required in the generated schema).
	LookupFields []string `yaml:"lookup_fields,omitempty"`
	// OptionalLookupFields are lookup fields that are Optional+Computed (not Required).
	OptionalLookupFields []string `yaml:"optional_lookup_fields,omitempty"`
}

// ScalarMapping defines how a custom GraphQL SCALAR maps to a Terraform schema type
// and the Go expressions used in expand/flatten.
// Use $VALUE in Expand as placeholder for the extracted d.GetOk value.
// Use $FIELD in Flatten as placeholder for result.FieldName.
type ScalarMapping struct {
	TFType  string `yaml:"tf_type"`  // e.g. "schema.TypeInt"
	GoType  string `yaml:"go_type"`  // e.g. "nrtime.EpochMilliseconds"
	Expand  string `yaml:"expand"`   // e.g. "nrtime.EpochMilliseconds($VALUE)"
	Flatten string `yaml:"flatten"`  // e.g. "int64($FIELD)"
}

// CRUDArgsConfig provides explicit argument lists for each CRUD operation,
// controlling which variables are passed in each client call.
// Valid tokens: account_id, org_id, id, input, and any arg name from create_inputs.
type CRUDArgsConfig struct {
	Create []string `yaml:"create,omitempty"`
	Read   []string `yaml:"read,omitempty"`
	Update []string `yaml:"update,omitempty"`
	Delete []string `yaml:"delete,omitempty"`
}

// Query is the information necessary to build a query method.  The Paths
// reference the the place in the hierarchy, while the names reference the
// objects within those paths to query.
type Query struct {
	// Path is the path of TypeNames in GraphQL that precede the objects being queried.
	Path []string `yaml:"path,omitempty"`
	// Names is a list of TypeName entries that will be found at the above Path.
	Endpoints []EndpointConfig `yaml:"endpoints,omitempty"`
}

type Command struct {
	Name              string        `yaml:"name,omitempty"`
	FileName          string        `yaml:"fileName,omitempty"`
	ShortDescription  string        `yaml:"shortDescription,omitempty"`
	LongDescription   string        `yaml:"longDescription,omitempty"`
	Example           string        `yaml:"example,omitempty"`
	InputType         string        `yaml:"inputType,omitempty"`
	ClientPackageName string        `yaml:"clientPackageName,omitempty"`
	ClientMethod      string        `yaml:"clientMethod,omitempty"`
	Flags             []CommandFlag `yaml:"flags,omitempty"`
	Subcommands       []Command     `yaml:"subcommands,omitempty"`
	GraphQLPath       []string      `yaml:"path,omitempty"`
}

type CommandFlag struct {
	Name         string `yaml:"name,omitempty"`
	Type         string `yaml:"type,omitempty"`
	DefaultValue string `yaml:"defaultValue"`
	Description  string `yaml:"description"`
	VariableName string `yaml:"variableName"`
	Required     bool   `yaml:"required"`
}

// GeneratorConfig is the information necessary to execute a generator.
type GeneratorConfig struct {
	// Name is the string that is used to reference a generator.
	Name string `yaml:"name,omitempty"`
	// TemplateDir is the path to the directory that contains all of the templates.
	TemplateDir string `yaml:"templateDir,omitempty"`
	// FileName is the target file that is to be generated.
	FileName string `yaml:"fileName,omitempty"`
	// TemplateName is the name of the template within the TemplateDir.
	TemplateName string `yaml:"templateName,omitempty"`
	// TemplateURL is a URL to a downloadable file to use as a Go template
	TemplateURL string `yaml:"templateURL,omitempty"`
}

// MutationConfig is the information about the GraphQL mutations.
type MutationConfig struct {
	// Name is the name of the GraphQL method.
	Name                  string            `yaml:"name"`
	MaxQueryFieldDepth    int               `yaml:"max_query_field_depth,omitempty"`
	ArgumentTypeOverrides map[string]string `yaml:"argument_type_overrides,omitempty"`
	ExcludeFields         []string          `yaml:"exclude_fields,omitempty"`
}

type EndpointConfig struct {
	Name               string   `yaml:"name,omitempty"`
	MaxQueryFieldDepth int      `yaml:"max_query_field_depth,omitempty"`
	IncludeArguments   []string `yaml:"include_arguments,omitempty"`
	ExcludeFields      []string `yaml:"exclude_fields,omitempty"`
}

// TypeConfig is the information about which types to render and any data specific to handling of the type.
type TypeConfig struct {
	// InterfaceMethods is a list of additional methods that are added to an interface definition. The methods are not
	// defined in the code, so must be implemented by the user.
	InterfaceMethods []string `yaml:"interface_methods,omitempty"`
	// Name of the type (required)
	Name string `yaml:"name"`
	// FieldTypeOverride is the Golang type to override whatever the default detected type would be for a given field.
	FieldTypeOverride string `yaml:"field_type_override,omitempty"`
	// CreateAs is used when creating a new scalar type to determine which Go type to use.
	CreateAs string `yaml:"create_as,omitempty"`
	// SkipTypeCreate allows the user to skip creating a Scalar type.
	SkipTypeCreate bool `yaml:"skip_type_create,omitempty"`
	// SkipFields allows the user to skip generating specific fields within a type.
	SkipFields []string `yaml:"skip_fields,omitempty"`
	// GenerateStructGetters enables the auto-generation of field getters for all fields on a struct.
	// i.e. if a struct has a field `name` then a function would be created called `GetName()`
	GenerateStructGetters bool `yaml:"generate_struct_getters,omitempty"`
	// Applies to all fields of the struct
	StructTags *StructTags `yaml:"struct_tags,omitempty"`
}

type StructTags struct {
	// Set the type of struct tags - e.g. ["json"] or for multiple ["json", "yaml", etc...]
	// Note this will apply to ALL fields within the struct. Use with caution.
	Tags []string `yaml:"tags"`

	// Set to `false` to exclude `omitempty` from struct tags
	// Note this will apply to ALL fields within the struct. Use with caution.
	OmitEmpty *bool `yaml:"omitempty"`
}

const (
	DefaultCacheEnable     = false
	DefaultCacheSchemaFile = "schema.json"
	DefaultLogLevel        = "info"
	DefaultAuthHeader      = "Api-Key"
	DefaultAuthEnvVar      = "TUTONE_API_KEY"
)

// LoadConfig will load a config file at the specified path or error.
func LoadConfig(file string) (*Config, error) {
	if file == "" {
		return nil, errors.New("config file name required")
	}
	log.WithFields(log.Fields{
		"file": file,
	}).Debug("loading package definition")

	yamlFile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	log.Tracef("definition: %+v", config)

	return &config, nil
}

func (c *PackageConfig) GetDestinationPath() string {
	if c.Path != "" {
		return c.Path
	}

	return "./"
}

func (c *PackageConfig) GetTypeConfigByName(name string) *TypeConfig {
	for _, typeConfig := range c.Types {
		if typeConfig.Name == name {
			return &typeConfig
		}
	}

	return nil
}
