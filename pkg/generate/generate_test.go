package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/tutone/internal/config"
)

var alphaAndBeta = []config.PackageConfig{{Name: "alpha"}, {Name: "beta"}}

func TestPackagesToGenerate_NoPackageFlags(t *testing.T) {
	t.Parallel()

	// What a run with neither flag passes: the --packages default.
	assert.Equal(t, []string{"alpha", "beta"}, packagesToGenerate(GeneratorOptions{PackageNames: []string{}}, alphaAndBeta))
	assert.Equal(t, []string{"alpha", "beta"}, packagesToGenerate(GeneratorOptions{}, alphaAndBeta))
}

func TestPackagesToGenerate_Packages(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"beta", "alpha"}, packagesToGenerate(GeneratorOptions{PackageNames: []string{"beta", "alpha"}}, alphaAndBeta))
}

func TestPackagesToGenerate_PackageWins(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"beta"}, packagesToGenerate(GeneratorOptions{PackageName: "beta", PackageNames: []string{"alpha"}}, alphaAndBeta))
}
