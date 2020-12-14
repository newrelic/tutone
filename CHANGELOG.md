<a name="v0.3.0"></a>
## [v0.3.0] - 2020-12-04
### Bug Fixes
- **golang:** Missing 0 == more allocations...
- **golang:** interface method memory usage can be right-sized
- **golang:** pass input prereqs for method signature
- **golang:** skip_type_create should skip for all types, not just scalars
- **golang:** ensure prefix for method arguments
- **schema:** ensure proper casing of mutation names
- **schema:** reduce handling of query args to only non-null
- **schema:** ensure proper handling of query args
- **typegen:** Avoid panic on nil pointer unmarshal

### Features
- **command:** use schema types to build CLI command
- **command:** add ability to generate READ commands (amend this commit with cleanup)
- **format:** programmatically run goimports on generated code, template updates
- **query:** Enable nullable fields in query (this enables all of them...)
- **schema:** implement helper to gatehr input variables from path
- **typegen:** Add package import path to config
- **typegen:** Allow custom methods to be added to an interface definition

<a name="v0.2.5"></a>
## [v0.2.5] - 2020-10-08
### Bug Fixes
- **build:** update changelog action for improved standards
- **deps:** use v3 package for sprig
- **schema:** use better comparison when overriding field names

<a name="v0.2.4"></a>
## [v0.2.4] - 2020-10-07
### Bug Fixes
- **expander:** ensure expansion of type arguments
- **golang:** ensure list kinds are represented as slices
- **golang:** ensure more sorting for deterministic output
- **golang:** sort the methods before return
- **schema:** ensure possibleTypes on interfaces are expanded

### Documentation Updates
- fix casing in README and format
- tidy up on some documentation

### Features
- add new generator for generating CLI commands
- include query string handling for golang
- begin method to build a query string from a Type

<a name="v0.2.3"></a>
## [v0.2.3] - 2020-09-04
<a name="v0.2.2"></a>
## [v0.2.2] - 2020-09-03
### Bug Fixes
- **golang:** move Interface reference to template
- **release:** update project name for goreleaser

<a name="v0.2.1"></a>
## [v0.2.1] - 2020-09-03
### Bug Fixes
- **changelog:** update changelog on release only, drop reviewer spec
- **golang:** ensure Name references use goFormatName()
- **schema:** use correct name for lookups

<a name="v0.2.0"></a>
## [v0.2.0] - 2020-09-02
### Bug Fixes
- ensure only specific package types are generated when passing --package option
- **codegen:** update package ref for go mod usage
- **generate:** ensure correct generator client
- **lang:** remove pointer reference from return type
- **nerdgraphclient:** move condition block end to exclude mutation
- **schema:** avoid recursing forever when handling interface kinds
- **schema:** ensure recursive expansion for additional Kinds
- **schema:** ensure proper handling of list interfaces

### Features
- **codegen:** implement sprig community template functions
- **lang:** begin GoMethod implementation
- **schema:** implement type expansion based on method name

<a name="v0.1.2"></a>
## [v0.1.2] - 2020-08-14
<a name="v0.1.1"></a>
## [v0.1.1] - 2020-07-23
### Bug Fixes
- **schema:** avoid expanding a type twice

<a name="v0.1.0"></a>
## v0.1.0 - 2020-07-23
### Bug Fixes
- **fetch:** exit non-zero on fatal log message
- **generate:** dont double prepend [] for list types - i.e. [][]type
- **generate:** remove generate.yml and all instances of it's reference
- **schema:** ensure proper handling of LIST types
- **util:** ensure fields of nested types are also expanded

### Documentation Updates
- **tutone:** include a couple doc strings
- **tutone:** include a what? section

### Features
- **fetch:** Fetch root mutation type
- **fetch:** Generic schema fetching and cache to file
- **generate:** format the generated source code according Go standards
- **generate:** WIP - first attempt at getting tutone to generate types
- **generate:** Generate command reading configs
- **generate:** implement --refetch flag
- **generate:** fetch if schema not present
- **generator:** introduce a generator concept
- **tutone:** default path for tutone config file

[Unreleased]: https://github.com/newrelic/newrelic-client-go/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/newrelic/newrelic-client-go/compare/v0.2.5...v0.3.0
[v0.2.5]: https://github.com/newrelic/newrelic-client-go/compare/v0.2.4...v0.2.5
[v0.2.4]: https://github.com/newrelic/newrelic-client-go/compare/v0.2.3...v0.2.4
[v0.2.3]: https://github.com/newrelic/newrelic-client-go/compare/v0.2.2...v0.2.3
[v0.2.2]: https://github.com/newrelic/newrelic-client-go/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/newrelic/newrelic-client-go/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.2...v0.2.0
[v0.1.2]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.0...v0.1.1
