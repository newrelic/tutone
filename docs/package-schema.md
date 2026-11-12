
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
go_package_name: string, optional                   # Identifier on the generated `package` line. Default: name lower-cased. Example: name "scorecards" written into package "servicearchintelligence". See "Package and client type names" below
client_type_name: string, optional                  # Receiver type of the generated client methods. Default: name with its first letter upper-cased. Example: "OrganizationManagement". See "Package and client type names" below
preserve_hand_written: bool, optional               # Default true. See "Hand-written code in generated files" below
keep_removed_fields: bool, optional                 # Default false. Keep the struct fields and enum values the existing types file has and the schema no longer lists, each marked with a note that the schema does not list it. Needs preserve_hand_written. See "Fields and values the schema dropped" below

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
        include_optional_arguments: bool, optional  # Declare every optional argument in the query and take each as a pointer (a slice for a list) a caller can leave unset; an unset one is not sent. For endpoints a package adds: it changes parameter types. See "Optional arguments" below
        context_method_name: string, optional       # Name of the context.Context variant for templates that generate one. Default: method_name + "WithContext". Example: "GetDestinationsWithContextAccount"
        include_implementations: []string, optional # Keep only these implementations of the interface this endpoint returns, for this package only
        keep_existing: bool, optional               # Do not generate this endpoint: its methods and query constant stay in the file as hand-written code. See "Hand-written code in generated files" below
        argument_type_overrides: []object, deprecated # Accepted so existing configs load, but ignored on endpoints (only mutations use it). A warning is logged; remove it

# Mutations schema: The GraphQL mutations you want to generate code for. Requires the `nerdgraphclient` generator.
mutations: []object, optional
  - name: string, required                          # The name of the mutation to generator code for - example: "alertsPolicyCreate"
    max_query_field_depth: int, optional            # Max depth of the result fields the mutation's query selects. Typically is set to 2
    argument_type_overrides: object, optional       # A key:value map of where the key is the argument name and the value is the GraphQL type override
    exclude_fields: []string, optional              # A list of fields to exclude from the mutation
    exclude_arguments: []string, optional           # Arguments to leave out of the Go method and the query, e.g. an optional `version` that must not be sent when unset. An unknown name is an error
    argument_go_types: object, optional             # Map of GraphQL argument name to the Go type of its parameter, e.g. cursor: "*string". An unknown name is an error
    include_optional_arguments: bool, optional      # Take each optional argument as a pointer (a slice for a list) a caller can leave unset; an unset one is not sent. For mutations a package adds: it changes parameter types. See "Optional arguments" below
    rename_arguments: object, optional              # Map of GraphQL argument name to Go parameter name. Go keywords get a safe name automatically ("type" becomes "typeArg"); this chooses a different one. An unknown name is an error
    keep_existing: bool, optional                   # Do not generate this mutation: its method and query constant stay in the file as hand-written code. See "Hand-written code in generated files" below

# Types schema: The GraphQL types to use for generating Go types. Requires the `typegen` generator
types: []object, optional
  - name: string, required                          # The type to generate
    field_type_override: string, optional           # Go type to use for fields of this type instead of a generated one - example: "common.EntityGUID"
    skip_type_create: bool, optional                # Skips creating the Go type. Usually specified along with `field_type_override`. See "Unions" below for a union
    skip_fields: []string, optional                 # A list of fields to exclude from the Go type
    create_as: string, optional                     # Used when creating a new scalar type to determine which Go type to use
    interface_methods: []string, optional           # List of additional methods that are added to an interface or union definition. The methods are not defined in the code, so must be implemented by the user.
    generate_struct_getters: bool, optional         # Generates a getter for each field, e.g. GetName() for name
    struct_tags: object, optional                   # Tags on every field: tags, e.g. ["json", "yaml"], and omitempty (false leaves omitempty out). An entry in fields overrides one field
    include_implementations: []string, optional     # For an interface or union type: keep only these implementations (members), for this package only. A name that is not one of them is an error. An interface's decoder returns the others as the interface's own struct, see "Narrowed interfaces" below
    keep_existing: bool, optional                   # Keep this type's declaration in the existing file exactly as it is, as if it were marked DO NOT REGENERATE, and its methods and companion declarations too. See "Hand-written code in generated files" below
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
- every declaration marked `DO NOT DELETE` or `DO NOT REGENERATE` (any case), exactly as written. The marker
  can be anywhere in the declaration: its doc comment, a field comment or its body. If the generator also
  produces a declaration of that name, the existing text replaces it, in place. A marker does not count when
  the newly generated declaration contains the same phrase, because a description from the schema can say
  "do not delete" about something else, and
- every type set to `keep_existing: true` that the file declares, exactly as written, as if it were marked,
  and what the templates declare for it: its methods (getters, `Implements...`, `UnmarshalJSON`) whether the
  receiver is the type or a pointer to it, an enum's `<Name>Types` variable, and an interface's
  `<Name>Interface` type and `Unmarshal<Name>Interface` function. Each is kept only if the file declares it,
  and generated normally otherwise. The type is still expanded, so types that refer to it are generated as
  before.

Everything else is replaced by the newly generated version. Imports the kept code needs are carried over.
If the existing file does not parse, generation fails and the file is left as it was.

A declaration the generator used to produce and no longer does (for example a type removed from the config)
cannot be told apart from hand-written code, so it is kept as well. To drop such leftovers, set
`preserve_hand_written: false` for the package, or delete them by hand.

A mutation or endpoint set to `keep_existing: true` is not generated, and it takes no part in the check for
two methods with one name. Its methods and query constant stay in the file as hand-written code, by the first
rule above. The types it uses, and an endpoint's response struct (in the types file, read by the hand-written
methods), are still generated. Set it on both of two endpoints that share a name under different paths and were
renamed by hand, such as `destinations` under `[actor, account, aiNotifications]` and
`[actor, organization, aiNotifications]`, which otherwise fail with
`method "GetDestinations" is generated more than once`.

`keep_existing` needs `preserve_hand_written` to be true, as it is by default. Otherwise the file is replaced
as a whole, and loading the config logs a warning.

### Fields and values the schema dropped

Regenerating a types file drops a struct field or enum value that the schema no longer lists, and code that
uses it stops compiling. With `keep_removed_fields: true` on the package, regenerating keeps them instead:

- A struct field the existing declaration has and the generated one lacks is copied from the existing file as
  it is written there (Go name, type, tags, comments). It is matched to the schema by the name in its `json`
  tag, so a field the schema still lists is never added a second time, and one whose Go type changed is
  generated as the schema says.
- An enum value the existing `<Name>Types` variable has and the generated one lacks is kept in the variable's
  struct type and in its literal.
- Each kept field or value gets a paragraph at the end of its doc comment:
  `// Not in the NerdGraph schema; kept so existing code compiles.` It is not marked `Deprecated:`, because the
  field may be one the schema hides from the key that generated it rather than one it removed.
- Kept members are placed where the generator would have sorted them: by Go name, among the generated ones. The
  same input always gives the same file.

Only types that the existing file and the generated source both declare are looked at, so nothing is added that
the existing file lacks. A field listed in the type's `skip_fields` is left out on purpose and is not kept. A field
is also not kept, with a warning, when the generated struct already uses its Go name for another JSON name, and
fields without a `json` name, embedded fields and fields declared together (`A, B int`) are never kept. A type
set to `keep_existing: true` or marked `DO NOT REGENERATE` is already kept as written.

A struct that gets a generated `UnmarshalJSON` (see "`UnmarshalJSON` for interface and union fields" below)
decodes the fields the schema lists, so that method does not decode a kept field, which stays in the struct. The
queries tutone generates never select a field the schema lacks, so their responses do not carry one. A
hand-written query that selects a kept field into such a struct does not have it decoded.

The option applies to the types file (`typegen`), not to the structs the client file declares for responses. It
needs `preserve_hand_written` to be true, as it is by default; loading the config logs a warning otherwise.
Whatever the existing file declares that the schema dropped as a whole type is kept by the rules above, with or
without this option. With this option such a type is marked too: when its doc comment starts with the form
tutone writes, `// <Name> - <description>` (for an interface type `<Name>Interface`, the name without the suffix),
the same note follows it as its own paragraph, after an empty `//` line, and the type is listed in the log line of
what was kept. A declaration written by hand (another doc form, or none), one marked `DO NOT DELETE` or
`DO NOT REGENERATE`, and one set to `keep_existing: true` are left as they are.

### Optional arguments

By default an optional (nullable) argument of a query endpoint is part of the Go method's parameters but not of
its GraphQL query, unless `include_arguments` names it, so the method accepts a value it never sends. A mutation
declares every argument in its query, but an optional one is a plain parameter, so a caller with no value sends
the type's zero value (`version: 0`).

`include_optional_arguments: true` on an endpoint or a mutation changes that for the operation:

- An endpoint declares every optional argument in its query (less those in `exclude_arguments`).
- The method takes each nullable argument as a pointer, `*T`, so `nil` means unset. A list stays `[]T`, which is
  `nil` when unset. Required arguments, path parameters and an `accountId` argument shared with the path are as
  they were. A type from `argument_go_types` is used as written and is never made a pointer to a pointer; a
  `field_type_override` that is a pointer already, such as `*Filter`, stays as it is.
- The request's variables leave out a parameter that is `nil`: the generated method deletes its key from `vars`
  right after the map is built (`if first == nil { delete(vars, "first") }`), so the API applies its own default
  (55 optional arguments in the schema have one, e.g. `first: 10`) rather than receiving an explicit `null`. The
  check is written for the method and, for a template that has one, its `context.Context` variant, under the names
  `method_name` and `context_method_name` give them. If the template prints neither, generation logs a warning
  that unset arguments are sent as `null`. A parameter whose `argument_go_types` entry cannot be `nil` (such as
  `string`) is always sent.

The key changes the parameter types of a method that exists, which breaks its callers, so set it on the
operations a package is adding. Operations without it generate as they did.

### Narrowed interfaces

`include_implementations` on a `types` entry narrows an interface to some of its implementations, so the
`Unmarshal<Name>Interface` function lists only those in its `switch`. The API still sends the others, and the
function used to end by returning `interface <Name> was not matched against all PossibleTypes`, which fails a
call that may already have taken effect. For a narrowed interface the function now ends by decoding the response
into the `<Name>` struct, which holds the fields every implementation shares, and returning it as
`<Name>Interface`, as the cases of the `switch` return theirs. Only the end of the function changes; the branch
for a response with no `__typename` still returns its error. If the file does not declare both
`type <Name> struct` and `func (x *<Name>) Implements<Name>()`, for example because the type is skipped, the
function is left as it was. An interface that is not narrowed is not touched. A narrowing set only on an endpoint
(`include_implementations` there) does not do this; set it on the type.

### Unions

A GraphQL union is generated the way an interface is: a Go interface `<Name>Interface` with the marker method
`Implements<Name>()`, an `Unmarshal<Name>Interface` function that picks the member by `__typename`, and the
marker method on each member type. A union has no fields, so no `<Name>` struct is made. A field of a union
type has the Go type `<Name>Interface` (`[]<Name>Interface` for a list), and its struct gets an `UnmarshalJSON`.
A query that selects such a field asks for `__typename` and, in an inline fragment per member, each member's
fields. Members are expanded with the union. A member the package does not generate (`skip_type_create`) is
left out of the union with a warning.

Nothing is generated for a union whose Go type the package already has, and its fields keep that type. The
package has it when

- the union is set to `field_type_override` or `skip_type_create`, or
- a hand-written file in the package directory (not a test, and not marked `Code generated ... DO NOT EDIT`)
  declares a type called `<Name>` or, failing that, `<Name>Interface`.

A package with only `<Name>Interface` gets that type for the fields of the union and for what the methods return,
as with a generated union: tutone sets the union's `field_type_override` to it. No generated function decodes
those fields by `__typename`, and `encoding/json` cannot decode into an interface, so the package declares the
`UnmarshalJSON` of the structs that hold them. Tutone leaves a hand-written `UnmarshalJSON` alone (see below).

The members of such a union are not expanded because of it. The response struct of a query endpoint that returns
a type the package supplies, a union or any other, has the endpoint's field, which the method reads.

### `UnmarshalJSON` for interface and union fields

A struct with a field of an interface or union type gets an `UnmarshalJSON` that decodes the field by
`__typename`, for a single value and for a list. That includes the structs on the way to a query endpoint
(for example `EntityManagementActorStitchedFields`, whose `entity` is a single interface value), and the
`<Name>QueryResponse` struct of a mutation that sends no variables and returns an interface or a union, as long
as the package generates the `Unmarshal<Name>Interface` function for every interface field of the struct. When
the package supplies the Go type of an interface by hand there is no such function, and the struct gets no
`UnmarshalJSON`.

The `<Name>QueryResponse` struct of a mutation that sends variables is declared by the client template, which
has no `UnmarshalJSON`. Such a mutation does not decode a result that is an interface or a union: the call fails
with `json: cannot unmarshal object into Go struct field ...`. The package declares `UnmarshalJSON` for that
struct in a hand-written file.

It is also not generated when a hand-written file of the package, as described above, already declares
`UnmarshalJSON` on that type, so the package never declares it twice. An `UnmarshalJSON` in the file being
generated follows the rules for hand-written code above.

### Package and client type names

`go_package_name` and `client_type_name` apply whatever the template writes. After a template is rendered,
tutone sets the `package` line of the file to `go_package_name`, and, in `nerdgraphclient` output, gives the
methods whose receiver is the default client type (the package name with its first letter upper-cased, which is
what a template's `title` produces) the type `client_type_name`. Templates written before these keys existed
(`package {{.PackageName | lower}}` and `{{$packageName | title}}` as the receiver) therefore need no change. Only
receivers are renamed. A key that is not set leaves what the template wrote as it is.

### Other generation rules

- Structs with the same name that arise from different query paths are merged into one. Two fields of the
  same name must have the same type, otherwise generation fails and names the field.
- Structs that contain each other by value (an invalid recursive type in Go) get a pointer on one field.
  A `pointer: true` field override takes effect first.
- Generation fails if two methods in a package would get the same Go name, or if two parameters of one method
  would.
- A mutation or query endpoint whose type is a scalar or an enum, or a list of them, is queried without a
  selection set.
- A mutation whose Go return type is not an exported identifier, such as `bool`, `[]string` or
  `nrtime.EpochSeconds`, returns that type, read from the `Result` field of its `<Name>QueryResponse` struct. The
  client template can only print a type that is an identifier, so tutone renders the method with a stand-in and
  puts the type back.
- A mutation that sends no variables is queried without parentheses (`mutation { name { ... } }`), and its
  `<Name>QueryResponse` struct is declared in the types file, because the client template declares that struct
  only for a mutation with variables. A package without the `nerdgraphclient` generator gets none.
- An endpoint below `account(id:)` that has an `accountId` argument of the same type as the account takes one
  `accountID` parameter and sends it for both. When the types differ, generation fails and names the parameter;
  `exclude_arguments` resolves it.
- Generating `agentApplications_api.go` when `agentapplications_api.go` exists writes to
  `agentapplications_api.go` and logs a warning naming both; set `file_name` so they agree.
