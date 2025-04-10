package sigstore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"
)

// NoAuth can be passed to RunTestRegistry to disable authentication.
func NoAuth(*http.Request) error { return nil }

// BasicAuth can be passed to RunTestRegistry to require HTTP basic authentication.
func BasicAuth(username, password string) func(*http.Request) error {
	return func(req *http.Request) error {
		un, pw, ok := req.BasicAuth()
		if !ok {
			return errors.New("no basic auth")
		}
		if un != username || pw != password {
			return errors.New("incorrect username or password")
		}
		return nil
	}
}

// RunTestRegistry starts a test registry server and returns its host.
func RunTestRegistry(t *testing.T, authFn func(*http.Request) error) string {
	t.Helper()

	// The test fixtures are in this package's directory, but the helper is used
	// from other packages (so we can't just use relative paths like usual).
	//
	// TODO: it'd be better to use `go:embed` here but for some reason it causes
	// HEAD requests to the blob paths to return 404s.
	_, helperFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(helperFile), "testdata")
	server := http.FileServer(http.Dir(fixtureDir))

	registry := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := authFn(r); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			// go-containerregistry checks the Content-Type header to determine
			// whether the registry supports the Referrers API.
			if strings.Contains(r.URL.Path, "referrers") {
				w.Header().Set("Content-Type", string(types.OCIImageIndex))
			}
			rt := &responseTracker{ResponseWriter: w}
			server.ServeHTTP(rt, r)
			t.Logf("%s %s (%d)", r.Method, r.URL.Path, rt.Status())
		}),
	)
	t.Cleanup(registry.Close)

	regURL, err := url.Parse(registry.URL)
	require.NoError(t, err)

	return regURL.Host
}

type responseTracker struct {
	http.ResponseWriter
	statusCode int
}

func (rt *responseTracker) WriteHeader(statusCode int) {
	rt.statusCode = statusCode
	rt.ResponseWriter.WriteHeader(statusCode)
}

func (rt *responseTracker) Status() int {
	if rt.statusCode == 0 {
		return http.StatusOK
	}
	return rt.statusCode
}
