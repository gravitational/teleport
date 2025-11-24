package scim

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestSCIMPatchScaffolding(t *testing.T) {
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	scimToken := createGenericSCIMPlugin(t, sut)

	baseURL := url.URL{
		Scheme: "https",
		Host:   sut.ProxyAddr,
		Path:   "/v1/webapi/scim/generic",
	}

	httpClient := &http.Client{
		Transport: &bearerAuthTransport{
			Token: scimToken,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
	patchOps := map[string]any{
		"schemas":    []string{scimsdk.PatchOpSchema},
		"Operations": "{}",
	}

	resp, err := doPatchResource(httpClient, baseURL.String(), "Users", "user-id-123", patchOps)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	resp, err = doPatchResource(httpClient, baseURL.String(), "Groups", "group-id-123", patchOps)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	httpClientWithWrongToken := &http.Client{
		Transport: &bearerAuthTransport{
			Token: "wrong-token",
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	resp, err = doPatchResource(httpClientWithWrongToken, baseURL.String(), "Group", "group-id-123", patchOps)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func doPatchResource(httpClient *http.Client, baseURL, resourceType, resourceID string, ops any) (*http.Response, error) {
	u := baseURL + "/" + resourceType + "/" + resourceID
	payload, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPatch, u, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/scim+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
