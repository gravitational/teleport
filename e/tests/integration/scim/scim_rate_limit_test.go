package scim

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestSCIMRateLimit(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	scimToken := createGenericSCIMPlugin(t, sut, withRateLimit(&types.PluginSCIMRateLimit{
		Average:       2,
		Burst:         2,
		PeriodSeconds: 60,
	}))

	listURL := url.URL{
		Scheme: "https",
		Host:   sut.ProxyAddr,
		Path:   "/v1/webapi/scim/generic/Users",
	}
	client := newBearerClient(scimToken)

	resp := mustHTTPGet(t, client, listURL.String())
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = mustHTTPGet(t, client, listURL.String())
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Third request must be rate-limited.
	resp = mustHTTPGet(t, client, listURL.String())
	resp.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get("retry-after"))
}
