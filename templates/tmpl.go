// Package templates embeds all tutone generator templates into the binary,
// making tutone self-contained when run from a repo that doesn't have a local
// templates/ directory (e.g. newrelic-client-go, terraform-provider-newrelic).
package templates

import "embed"

//go:embed clientgo command nerdgraphclient terraform typegen
var FS embed.FS
