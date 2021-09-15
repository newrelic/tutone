//go:build unit
// +build unit

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVersion simply ensures that Version is not empty
func TestVersion(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, Version)
}
