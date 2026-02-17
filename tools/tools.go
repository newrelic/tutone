//go:build tools
// +build tools

package tools

// Note: github.com/psampaz/go-mod-outdated is not imported here because it has no
// importable library package. It's installed directly via build/tools.mk and tracked
// in go.mod's require section.

import (
	// build/test.mk
	_ "github.com/stretchr/testify/assert"
	_ "gotest.tools/gotestsum/testjson"

	// build/lint.mk
	_ "github.com/client9/misspell"
	_ "github.com/golangci/golangci-lint/pkg/golinters"
	_ "golang.org/x/tools/go/packages"

	// build/document.mk
	_ "github.com/git-chglog/git-chglog"
	_ "golang.org/x/tools/go/packages"

	// build/release.mk
	_ "github.com/caarlos0/svu/pkg/svu"
	_ "github.com/goreleaser/goreleaser/pkg/config"
	_ "github.com/x-motemen/gobump"
)
