<a name="unreleased"></a>
## [Unreleased]

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

[Unreleased]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.2...HEAD
[v0.1.2]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.0...v0.1.1
