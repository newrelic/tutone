# Tutone

[![Testing](https://github.com/newrelic/tutone/workflows/Testing/badge.svg)](https://github.com/newrelic/tutone/actions)
[![Security Scan](https://github.com/newrelic/tutone/workflows/Security%20Scan/badge.svg)](https://github.com/newrelic/tutone/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/newrelic/tutone?style=flat-square)](https://goreportcard.com/report/github.com/newrelic/tutone)
[![GoDoc](https://godoc.org/github.com/newrelic/tutone?status.svg)](https://godoc.org/github.com/newrelic/tutone)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/newrelic/tutone/blob/master/LICENSE)
[![CLA assistant](https://cla-assistant.io/readme/badge/newrelic/tutone)](https://cla-assistant.io/newrelic/tutone)
[![Release](https://img.shields.io/github/release/newrelic/tutone/all.svg)](https://github.com/newrelic/tutone/releases/latest)

Code generation tool

Generate Golang types from GraphQL schema introspection

## Getting Started
1. Create a project configuration file, see `configs/tutone.yaml` for an example.
1. Generate a `schema.json` using the following command:

   ```bash
   $ tutone fetch \
     --config path/to/tutone.yaml \
     --cache \
     --output path/to/schema.json
   ```
   
1. Add a `./path/to/package/typegen.yaml` configuration with the type you want generated:

   ```yaml
   ---
   types:
     - name: MyGraphQLTypeName
     - name: AnotherTypeInGraphQL
       createAs: map[string]int
   ```
1. Add a generation command inside the `main.go` (or equivalent)

   ```go
   // Package CoolPackage provides cool stuff, based on generated types
   //go:generate tutone generate -p $GOPACKAGE
   package CoolPackage
   // ... implementation ...
   ```
1. Run `go generate`
1. Add the `./path/to/package/types.go` file to your repo

# Configuration

## Command Flags

Flags for running the typegen command:

| Flag | Description |
| ---- | ----------- |
| `-p <Package Name>` | Package name used within the generated file. Overrides the configuration file. |
| `-v` | Enable verbose logging |


## Per-package

Configuration on what types to generate, and any overrides from the schema
exist within the package directory in a file named `typegen.yaml`. The file has
a simple configuration format, and includes the following sections:

### types

Types is a list of the types to explicitly generate.  Any required sub-type will
also be generated until we hit a Golang type.

| Name | Required | Description |
| ---- | -------- | ----------- |
| name | Yes | The name of the field to search for and create |
| package | No | Name of the package the output file will be part of (see `-p` flag) |
| createAs | No | If you want to override the type that is created, use this to explicitly name the type |

**ORDER MATTERS:** Add types with overrides first, otherwise they might not get
created as you expect. If A => B => gotype, and you want to override B, you
must configure it first.  If you configure A first, B will be generated as a
dependency before we create B via configuration.

**Example:**

```yaml
---
types:
  - name: TheName
    createAs: int
  - name: ComplexType
    createAs: map[string]interface{}
  - name: AnotherName
```


## Community

New Relic hosts and moderates an online forum where customers can interact with New Relic employees as well as other customers to get help and share best practices. 

* [Roadmap](https://newrelic.github.io/developer-toolkit/roadmap/) - As part of the Developer Toolkit, the roadmap for this project follows the same RFC process
* [Issues or Enhancement Requests](https://github.com/newrelic/tutone/issues) - Issues and enhancement requests can be submitted in the Issues tab of this repository. Please search for and review the existing open issues before submitting a new issue.
* [Contributors Guide](CONTRIBUTING.md) - Contributions are welcome (and if you submit a Enhancement Request, expect to be invited to contribute it yourself :grin:).
* [Community discussion board](https://discuss.newrelic.com/c/build-on-new-relic/developer-toolkit) - Like all official New Relic open source projects, there's a related Community topic in the New Relic Explorers Hub.

Keep in mind that when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. If you'd like to execute our corporate CLA, or if you have any questions, please drop us an email at opensource@newrelic.com.


## Development

### Requirements

* Go 1.13.0+
* GNU Make
* git


### Building

```
# Default target is 'build'
$ make

# Explicitly run build
$ make build

# Locally test the CI build scripts
# make build-ci
```


### Testing

Before contributing, all linting and tests must pass.  Tests can be run directly via:

```
# Tests and Linting
$ make test

# Only unit tests
$ make test-unit

# Only integration tests
$ make test-integration
```

### Commit Messages

Using the following format for commit messages allows for auto-generation of
the [CHANGELOG](CHANGELOG.md):

#### Format:

`<type>(<scope>): <subject>`

| Type | Description | Change log? |
|------| ----------- | :---------: |
| `chore` | Maintenance type work | No |
| `docs` | Documentation Updates | Yes |
| `feat` | New Features | Yes |
| `fix`  | Bug Fixes | Yes |
| `refactor` | Code Refactoring | No |

#### Scope

This refers to what part of the code is the focus of the work.  For example:

**General:**

* `build` - Work related to the build system (linting, makefiles, CI/CD, etc)
* `release` - Work related to cutting a new release

**Package Specific:**

* `newrelic` - Work related to the New Relic package
* `http` - Work related to the `internal/http` package
* `alerts` - Work related to the `pkg/alerts` package



### Documentation

**Note:** This requires the repo to be in your GOPATH [(godoc issue)](https://github.com/golang/go/issues/26827)

```
$ make docs
```


## Support

New Relic has open-sourced this project. This project is provided AS-IS WITHOUT WARRANTY OR SUPPORT, although you can report issues and contribute to the project here on GitHub.

_Please do not report issues with this software to New Relic Global Technical Support._


## Open Source License

This project is distributed under the [Apache 2 license](LICENSE).
