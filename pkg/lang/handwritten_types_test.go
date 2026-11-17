//go:build unit
// +build unit

package lang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

// generatedNames lists every type name GenerateGoTypesForPackage declares for the package,
// with the variable an enum brings and the Go name of an interface.
func generatedNames(t *testing.T, s *schema.Schema, pkg *config.PackageConfig) map[string]bool {
	t.Helper()

	expanded, err := schema.ExpandTypes(s, pkg)
	require.NoError(t, err)

	structs, enums, scalars, interfaces, err := GenerateGoTypesForPackage(s, &config.GeneratorConfig{}, pkg, expanded)
	require.NoError(t, err)

	names := map[string]bool{}
	for _, x := range *structs {
		names[x.Name] = true
	}

	for _, x := range *enums {
		names[x.Name] = true
		names[x.Name+"Types"] = true
	}

	for _, x := range *scalars {
		names[x.Name] = true
	}

	for _, x := range *interfaces {
		names[x.Name+"Interface"] = true
	}

	return names
}

func TestGenerateGoTypes_ATypeDeclaredByHandInThePackageIsNotGeneratedAgain(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind     schema.Kind
		hand     string
		wantGone []string
	}{
		"object":                      {schema.KindObject, "type Supplied struct{ ID string }", []string{"Supplied"}},
		"scalar":                      {schema.KindScalar, "type Supplied string", []string{"Supplied"}},
		"enum by its type":            {schema.KindENUM, "type Supplied string", []string{"Supplied", "SuppliedTypes"}},
		"enum by its values variable": {schema.KindENUM, "var SuppliedTypes = struct{ A string }{A: \"A\"}", []string{"Supplied", "SuppliedTypes"}},
		"interface":                   {schema.KindInterface, "type SuppliedInterface interface{ ImplementsSupplied() }", []string{"SuppliedInterface"}},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg := &config.PackageConfig{
				Name:    "things",
				Path:    t.TempDir(),
				Queries: []config.Query{{Path: []string{"actor", "thing"}, Endpoints: []config.EndpointConfig{{Name: "value", MaxQueryFieldDepth: 2}}}},
			}

			s := suppliedSchema(tc.kind)
			if tc.kind == schema.KindENUM {
				s.Types[3].EnumValues = []schema.EnumValue{{Name: "A"}}
			}

			before := generatedNames(t, s, pkg)
			for _, n := range tc.wantGone {
				require.True(t, before[n], "without the hand-written file %s is generated", n)
			}

			require.NoError(t, os.WriteFile(filepath.Join(pkg.Path, "supplied.go"), []byte("package things\n\n"+tc.hand+"\n"), 0o644))

			after := generatedNames(t, s, pkg)
			for _, n := range tc.wantGone {
				assert.False(t, after[n], "%s is declared by hand, so it is not generated", n)
			}

			assert.True(t, after["Thing"], "the other types are still generated")
		})
	}
}
