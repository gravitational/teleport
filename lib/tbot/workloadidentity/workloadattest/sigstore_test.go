package workloadattest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/sigstore"
	"github.com/gravitational/teleport/lib/utils"
)

func TestSigstoreAttestorConfig_CheckAndSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		cfg workloadattest.SigstoreAttestorConfig
		err string
	}{
		"credentials_path does not exist": {
			cfg: workloadattest.SigstoreAttestorConfig{
				Enabled:         true,
				CredentialsPath: "/does/not/exist",
			},
			err: "no such file or directory",
		},
		"credentials_path is a directory": {
			cfg: workloadattest.SigstoreAttestorConfig{
				Enabled:         true,
				CredentialsPath: t.TempDir(),
			},
			err: "cannot be a directory",
		},
		"additional_registries.host is empty": {
			cfg: workloadattest.SigstoreAttestorConfig{
				Enabled: true,
				AdditionalRegistries: []workloadattest.SigstoreRegistryConfig{
					{Host: ""},
				},
			},
			err: "additional_registries[0].host cannot be blank",
		},
		"additional_registries.host is invalid": {
			cfg: workloadattest.SigstoreAttestorConfig{
				Enabled: true,
				AdditionalRegistries: []workloadattest.SigstoreRegistryConfig{
					{Host: "/////"},
				},
			},
			err: "registries must be valid RFC 3986 URI authorities",
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			err := tc.cfg.CheckAndSetDefaults()
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestSigstoreAttestor_Attest_WithCredentials(t *testing.T) {
	registry := sigstore.RunTestRegistry(t,
		sigstore.BasicAuth("foo", "bar"),
	)
	dockerConfig, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			registry: map[string]string{
				"username": "foo",
				"password": "bar",
			},
		},
	})
	require.NoError(t, err)

	dockerConfigFile := filepath.Join(t.TempDir(), "docker-config.json")
	err = os.WriteFile(
		dockerConfigFile,
		dockerConfig,
		os.ModePerm,
	)
	require.NoError(t, err)

	attestor, err := workloadattest.NewSigstoreAttestor(
		workloadattest.SigstoreAttestorConfig{
			Enabled:         true,
			CredentialsPath: dockerConfigFile,
		},
		utils.NewSlogLoggerForTests(),
	)
	require.NoError(t, err)

	att, err := attestor.Attest(context.Background(), testContainer{
		image:       fmt.Sprintf("%s/simple-signing:v1", registry),
		imageDigest: "sha256:21c76c650023cac8d753af4cb591e6f7450c6e2b499b5751d4a21e26e2fc5012",
	})
	require.NoError(t, err)
	require.Len(t, att.Payloads, 2)
}

func TestSigstoreAttestor_Attest_Caching(t *testing.T) {
	ctx := context.Background()

	// Run a fake registry that just counts the number of requests to make sure
	// we don't hit it once we're cached.
	var requests int
	registryServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			http.NotFound(w, r)
		}),
	)

	registryURL, err := url.Parse(registryServer.URL)
	require.NoError(t, err)

	attestor, err := workloadattest.NewSigstoreAttestor(
		workloadattest.SigstoreAttestorConfig{
			Enabled: true,
		},
		utils.NewSlogLoggerForTests(),
	)
	require.NoError(t, err)

	ctr := testContainer{
		image:       fmt.Sprintf("%s/simple-signing:v1", registryURL.Host),
		imageDigest: "sha256:21c76c650023cac8d753af4cb591e6f7450c6e2b499b5751d4a21e26e2fc5012",
	}

	_, err = attestor.Attest(ctx, ctr)
	require.NoError(t, err)
	require.NotZero(t, requests)

	// Check we make no requests after it's cached.
	requests = 0

	_, err = attestor.Attest(ctx, ctr)
	require.NoError(t, err)
	require.Zero(t, requests)

	// Evict it from the cache and check we make more requests.
	attestor.EvictFromCache(ctx, ctr)

	_, err = attestor.Attest(ctx, ctr)
	require.NoError(t, err)
	require.NotZero(t, requests)
}

type testContainer struct{ image, imageDigest string }

func (c testContainer) GetImage() string       { return c.image }
func (c testContainer) GetImageDigest() string { return c.imageDigest }

var _ workloadattest.Container = (*testContainer)(nil)
