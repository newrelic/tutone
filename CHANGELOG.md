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

[Unreleased]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.1...HEAD
[v0.1.1]: https://github.com/newrelic/newrelic-client-go/compare/v0.1.0...v0.1.1
