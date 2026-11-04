package fetch

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetch_ReturnsFetchError(t *testing.T) {
	t.Parallel()

	err := Fetch("", true, "", "", filepath.Join(t.TempDir(), "schema.json"), false)

	assert.EqualError(t, err, "unable to fetch from empty URL")
}

func TestFetch_ReturnsSaveError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"__schema":{"types":[]}}}`))
	}))
	defer server.Close()

	// The directory does not exist, so saving the schema fails.
	err := Fetch(server.URL, true, "", "", filepath.Join(t.TempDir(), "missing", "schema.json"), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema.json")
}
