
### Package Schema

The following YAML schema facilitates adding and updating Go package in a project.

```yaml
name: string, required                              # The top-level name of the GraphQL endpoint scope - example: "alerts"

path: string, required                              # Directory path to generate (Required) - example: "pkg/alerts"

imports: []string, optional                         # Optional array of Go package imports to inject. - example: "- encoding/json"

generators: []string, required                      # Which generators to utilize. At least one of [typegen, nerdgraphclient, command]

# Queries schema: The GraphQL queries you want to generate code for. Requires the `nerdgraphclient` generator.
queries: []object, optional
  - path: []object, required                        # The path to follow for the query - example: ["actor", "cloud"] maps to actor.cloud GraphQL query scope
    endpoints: []object,                            # The endpoints to generate code for.
      - name: string, required                      # Must be endpoints under the associated query path - example: "linkedAccounts" (under cloud query scope)
        max_query_field_depth: int, optional        # Max recursion iterations for inferring required arguments and associated types. Typically is set to 2
        include_arguments: []string, optional       # Query arguments to include
        argument_type_overrides: []object, optional # Array of key:values where the key is the argument name and the value is the GraphQL type override

# Mutations schema: The GraphQL mutations you want to generate code for. Requires the `nerdgraphclient` generator.
mutations: []object, optional
  - name: string, required                          # The name of the mutation to generator code for - example: "alertsPolicyCreate"
    argument_type_overrides: object, optional       # A key:value map of where the key is the argument name and the value is the GraphQL type override
    exclude_fields: []string, optional              # A list of fields to exclude from the mutation

# Types schema: The GraphQL types to use for generating Go types. Requires the `typegen` generator
types: []object, optional
  - name: string, required                          # The type to generate
    field_type_override: string, optional           # A list of fields to exclude from the query
    skip_type_create: bool, optional                # Skips creating the Go type. Usually specified along with `field_type_override`
    skip_fields: []string, optional                 # A list of fields to exclude from the Go type
    create_as: string, optional                     # Used when creating a new scalar type to determine which Go type to use
    interface_methods: []string, optional           # List of additional methods that are added to an interface definition. The methods are not defined in the code, so must be implemented by the user.
```
