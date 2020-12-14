[![Community Project header](https://github.com/newrelic/open-source-office/raw/master/examples/categories/images/Community_Project.png)](https://github.com/newrelic/open-source-office/blob/master/examples/categories/index.md#category-community-project)

# Tutone

[![Testing](https://github.com/newrelic/tutone/workflows/Testing/badge.svg)](https://github.com/newrelic/tutone/actions)
[![Security Scan](https://github.com/newrelic/tutone/workflows/Security%20Scan/badge.svg)](https://github.com/newrelic/tutone/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/newrelic/tutone?style=flat-square)](https://goreportcard.com/report/github.com/newrelic/tutone)
[![GoDoc](https://godoc.org/github.com/newrelic/tutone?status.svg)](https://godoc.org/github.com/newrelic/tutone)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/newrelic/tutone/blob/master/LICENSE)
[![CLA assistant](https://cla-assistant.io/readme/badge/newrelic/tutone)](https://cla-assistant.io/newrelic/tutone)
[![Release](https://img.shields.io/github/release/newrelic/tutone/all.svg)](https://github.com/newrelic/tutone/releases/latest)

Code generation tool

Generate code from GraphQL schema introspection.

## Summary

At a high level, the following workflow is used to generate code.

-   `tutone fetch` calls the NerdGraph API to introspect the schema.
-   The schema is cached in `schema.json`.  This is information about the GraphQL schema
-   `tutone generate` uses the `schema.json` + the configuration + the templates to output generated text.

## Getting Started

1.  Create a project configuration file, see `configs/tutone.yaml` for an example.
2.  Generate a `schema.json` using the following command:

    ```bash
    $ tutone fetch \
      --config path/to/tutone.yaml \
      --cache \
      --output path/to/schema.json
    ```

3.  Add a `./path/to/package/typegen.yaml` configuration with the type you want generated:

    ```yaml
    ---
    types:
      - name: MyGraphQLTypeName
      - name: AnotherTypeInGraphQL
        createAs: map[string]int
    ```

4.  Add a generation command inside the `main.go` (or equivalent)

    ```go
    // Package CoolPackage provides cool stuff, based on generated types
    //go:generate tutone generate -p $GOPACKAGE
    package CoolPackage
    // ... implementation ...
    ```

5.  Run `go generate`
6.  Add the `./path/to/package/types.go` file to your repo

# Configuration

## Command Flags

Flags for running the typegen command:

| Flag                | Description                                                                    |
| ------------------- | ------------------------------------------------------------------------------ |
| `-p <Package Name>` | Package name used within the generated file. Overrides the configuration file. |
| `-v`                | Enable verbose logging                                                         |

## Configuration File

An example configuration can be found in [this project repo][example_config].

A configuration file is meant to represent a single project, with
specifications for which parts of the schema to process.

Please see the [config documentation][pkg_go_dev] for details about specific fields.

### packages

The `packages` field in the configuration contains the details about which
types and mutations to include from the schema, and where the package is located.

| Name       | Required | Description                                                             |
| ---------- | -------- | ----------------------------------------------------------------------- |
| name       | Yes      | The name of the package                                                 |
| path       | Yes      | Name of the package the output file will be part of (see `-p` flag)     |
| generators | Yes      | A list of generator names from the `generators` field                   |
| mutations  | No       | A list of mutations from which to infer types                           |
| types      | No       | A list of types from which to start expanding the inferred set of types |


#### Type Configuration

To fine-tune the types that are created, or not create them at all, the
following options are supported:

| Name                  | Required | Description |
| --------------------- | -------- | ----------- |
| `name`                | yes      | Name of the type to match |
| `create_as`           | no       | Used when creating a new scalar type to determine which Go type to use. |
| `field_type_override` | no       | Golang type to override whatever the default detected type would be for a given field. |
| `interface_methods`   | no       | List of additional methods that are added to an interface definition. The methods are not defined in the code, so must be implemented by the user. |
| `skip_type_create`    | no       | Allows the user to skip creating a type. |

### Generators

The `generators` field is used to describe a given generator.  The generator is
where the bulk of the work is done.  Note that the configuration name
referenced must match the name of the attached generated in the
`pkg/generate/generate.go` file.

The generator configuration specifies details about how the generator should adjust the output of the work.

| Name     | Required | Description                                                                  |
| -------- | -------- | ---------------------------------------------------------------------------- |
| name     | Yes      | The name of the generator used in `pkg/generate/generate.go` file            |
| fileName | No       | Where to write the output of the generated code within the specified package |

## Community

New Relic hosts and moderates an online forum where customers can interact with New Relic employees as well as other customers to get help and share best practices. 

-   [Roadmap](https://newrelic.github.io/developer-toolkit/roadmap/) - As part of the Developer Toolkit, the roadmap for this project follows the same RFC process
-   [Issues or Enhancement Requests](https://github.com/newrelic/tutone/issues) - Issues and enhancement requests can be submitted in the Issues tab of this repository. Please search for and review the existing open issues before submitting a new issue.
-   [Contributors Guide](CONTRIBUTING.md) - Contributions are welcome (and if you submit a Enhancement Request, expect to be invited to contribute it yourself :grin:).
-   [Community discussion board](https://discuss.newrelic.com/c/build-on-new-relic/developer-toolkit) - Like all official New Relic open source projects, there's a related Community topic in the New Relic Explorers Hub.

Keep in mind that when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. If you'd like to execute our corporate CLA, or if you have any questions, please drop us an email at opensource@newrelic.com.

## Development

### Requirements

-   Go 1.13.0+
-   GNU Make
-   git

### Building

    # Default target is 'build'
    $ make

    # Explicitly run build
    $ make build

    # Locally test the CI build scripts
    # make build-ci

### Testing

Before contributing, all linting and tests must pass.  Tests can be run directly via:

    # Tests and Linting
    $ make test

    # Only unit tests
    $ make test-unit

    # Only integration tests
    $ make test-integration

*Note:* You'll need to update `testdata/schema.json` to the latest GraphQL schema for tests to run
correctly.

### Commit Messages

Using the following format for commit messages allows for auto-generation of
the [CHANGELOG](CHANGELOG.md):

#### Format:

`<type>(<scope>): <subject>`

| Type       | Description           | Change log? |
| ---------- | --------------------- | :---------: |
| `chore`    | Maintenance type work |      No     |
| `docs`     | Documentation Updates |     Yes     |
| `feat`     | New Features          |     Yes     |
| `fix`      | Bug Fixes             |     Yes     |
| `refactor` | Code Refactoring      |      No     |

#### Scope

This refers to what part of the code is the focus of the work.  For example:

**General:**

-   `build` - Work related to the build system (linting, makefiles, CI/CD, etc)
-   `release` - Work related to cutting a new release

**Package Specific:**

-   `newrelic` - Work related to the New Relic package
-   `http` - Work related to the `internal/http` package
-   `alerts` - Work related to the `pkg/alerts` package

### Documentation

**Note:** This requires the repo to be in your GOPATH [(godoc issue)](https://github.com/golang/go/issues/26827)

    $ make docs

## Support

New Relic has open-sourced this project. This project is provided AS-IS WITHOUT WARRANTY OR SUPPORT, although you can report issues and contribute to the project here on GitHub.

_Please do not report issues with this software to New Relic Global Technical Support._

## Open Source License

This project is distributed under the [Apache 2 license](LICENSE).

[example_config]: https://github.com/newrelic/tutone/blob/master/configs/tutone.yml

[pkg_go_dev]: https://pkg.go.dev/github.com/newrelic/tutone@v0.2.3/internal/config?tab=doc
