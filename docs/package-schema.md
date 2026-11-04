
### Package Schema

The following YAML schema facilitates adding and updating Go package in a project.

```yaml
name: string, required                              # The top-level name of the GraphQL endpoint scope - example: "alerts"

path: string, required                              # Directory path to generate (Required) - example: "pkg/alerts"

import_path: string, optional                       # Full Go import path of the package - example: "github.com/newrelic/newrelic-client-go/v2/pkg/alerts"

imports: []string, optional                         # Optional array of Go package imports to inject. - example: "- encoding/json"

generators: []string, required                      # Which generators to utilize. At least one of [typegen, nerdgraphclient, command]

commands: []object, optional                        # Commands for the `command` generator. Keys: name, fileName, shortDescription, longDescription, example, inputType, clientPackageName, clientMethod, path, subcommands, and flags (name, type, defaultValue, description, variableName, required)

file_name: string, optional                         # Stem for {{.FileName}} in a generator's fileName template. Default: name. Set it when the file on disk is spelled differently, e.g. name "agentApplications" with file_name "agentapplications"
go_package_name: string, optional                   # Identifier on the generated `package` line. Default: name lower-cased. Example: name "scorecards" written into package "servicearchintelligence"
client_type_name: string, optional                  # Receiver type of the generated client methods. Default: name with its first letter upper-cased. Example: "OrganizationManagement"
preserve_hand_written: bool, optional               # Default true. See "Hand-written code in generated files" below

# Queries schema: The GraphQL queries you want to generate code for. Requires the `nerdgraphclient` generator.
queries: []object, optional
  - path: []object, required                        # The path to follow for the query - example: ["actor", "cloud"] maps to actor.cloud GraphQL query scope
    endpoints: []object,                            # The endpoints to generate code for.
      - name: string, required                      # Must be endpoints under the associated query path - example: "linkedAccounts" (under cloud query scope)
        max_query_field_depth: int, optional        # Max recursion iterations for inferring required arguments and associated types. Typically is set to 2
        include_arguments: []string, optional       # Query arguments to include
        exclude_fields: []string, optional          # Fields to leave out of the query
        exclude_arguments: []string, optional       # Optional arguments to leave out of the Go method and the query, e.g. "isCanceled". A name the endpoint does not take, or that the API requires, is an error
        method_name: string, optional               # Complete Go method name, default "Get" + name. Required on each of two endpoints that share a name under different paths, e.g. "GetDestinationsAccount"
        argument_go_types: object, optional         # Map of GraphQL argument name to the Go type of its parameter, e.g. cursor: "*string". An unknown name is an error
        context_method_name: string, optional       # Name of the context.Context variant for templates that generate one. Default: method_name + "WithContext". Example: "GetDestinationsWithContextAccount"
        include_implementations: []string, optional # Keep only these implementations of the interface this endpoint returns, for this package only
        argument_type_overrides: []object, deprecated # Accepted so existing configs load, but ignored on endpoints (only mutations use it). A warning is logged; remove it

# Mutations schema: The GraphQL mutations you want to generate code for. Requires the `nerdgraphclient` generator.
mutations: []object, optional
  - name: string, required                          # The name of the mutation to generator code for - example: "alertsPolicyCreate"
    max_query_field_depth: int, optional            # Max depth of the result fields the mutation's query selects. Typically is set to 2
    argument_type_overrides: object, optional       # A key:value map of where the key is the argument name and the value is the GraphQL type override
    exclude_fields: []string, optional              # A list of fields to exclude from the mutation
    exclude_arguments: []string, optional           # Arguments to leave out of the Go method and the query, e.g. an optional `version` that must not be sent when unset. An unknown name is an error
    argument_go_types: object, optional             # Map of GraphQL argument name to the Go type of its parameter, e.g. cursor: "*string". An unknown name is an error
    rename_arguments: object, optional              # Map of GraphQL argument name to Go parameter name. Go keywords get a safe name automatically ("type" becomes "typeArg"); this chooses a different one. An unknown name is an error

# Types schema: The GraphQL types to use for generating Go types. Requires the `typegen` generator
types: []object, optional
  - name: string, required                          # The type to generate
    field_type_override: string, optional           # Go type to use for fields of this type instead of a generated one - example: "common.EntityGUID"
    skip_type_create: bool, optional                # Skips creating the Go type. Usually specified along with `field_type_override`
    skip_fields: []string, optional                 # A list of fields to exclude from the Go type
    create_as: string, optional                     # Used when creating a new scalar type to determine which Go type to use
    interface_methods: []string, optional           # List of additional methods that are added to an interface definition. The methods are not defined in the code, so must be implemented by the user.
    generate_struct_getters: bool, optional         # Generates a getter for each field, e.g. GetName() for name
    struct_tags: object, optional                   # Tags on every field: tags, e.g. ["json", "yaml"], and omitempty (false leaves omitempty out). An entry in fields overrides one field
    include_implementations: []string, optional     # For an interface type: keep only these implementations, for this package only. A name that does not implement the interface is an error
    fields: []object, optional                      # Per-field overrides. An entry naming a field the type does not have is an error
      - name: string, required                      # GraphQL field name, e.g. "enabled"
        pointer: bool, optional                     # true makes the Go type a pointer. Ignored for interface-typed fields
        omitempty: bool, optional                   # true adds, false removes, `omitempty` on this field's struct tags
        go_type: string, optional                   # Replaces the whole Go type, slice and pointer markers included
        go_name: string, optional                   # Replaces the Go field name
```

### Unknown keys

A key that is not in this schema fails loading and the error names it, for example
`field pacakge_path not found in type config.PackageConfig`. Before this, a misspelled key was
ignored and the output silently differed from what the config asked for.

### Hand-written code in generated files

When a generated file already exists, regenerating it keeps:

- every top-level declaration (function, method, type, const, var) the generator does not produce, and
- every declaration whose doc comment contains `DO NOT DELETE` or `DO NOT REGENERATE` (any case), exactly
  as written. If the generator also produces a declaration of that name, the existing text replaces it, in place.

Everything else is replaced by the newly generated version. Imports the kept code needs are carried over.
If the existing file does not parse, generation fails and the file is left as it was.

A declaration the generator used to produce and no longer does (for example a type removed from the config)
cannot be told apart from hand-written code, so it is kept as well. To drop such leftovers, set
`preserve_hand_written: false` for the package, or delete them by hand.

### Other generation rules

- Structs with the same name that arise from different query paths are merged into one. Two fields of the
  same name must have the same type, otherwise generation fails and names the field.
- Structs that contain each other by value (an invalid recursive type in Go) get a pointer on one field.
  A `pointer: true` field override takes effect first.
- Generation fails if two methods in a package would get the same Go name.
- Generating `agentApplications_api.go` when `agentapplications_api.go` exists writes to
  `agentapplications_api.go` and logs a warning naming both; set `file_name` so they agree.
