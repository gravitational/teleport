/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package alpnproxy

import (
	"crypto"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
)

// AzureMSIMiddleware implements a simplified version of MSI server serving auth tokens.
type AzureMSIMiddleware struct {
	DefaultLocalProxyHTTPMiddleware

	// Identity is the Azure identity to be served by the server. Only single identity will be provided.
	Identity string
	// TenantID to be returned in a claim. Doesn't have to match actual TenantID as recognized by Azure.
	TenantID string
	// ClientID to be returned in a claim.
	ClientID string

	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// Log is the Logger.
	Log *slog.Logger
	// Secret to be provided by the client.
	Secret string

	// privateKey used to sign JWT
	privateKey   crypto.Signer
	privateKeyMu sync.RWMutex
}

var _ LocalProxyHTTPMiddleware = &AzureMSIMiddleware{}

func (m *AzureMSIMiddleware) CheckAndSetDefaults() error {
	if m.Clock == nil {
		m.Clock = clockwork.NewRealClock()
	}
	if m.Log == nil {
		m.Log = slog.With(teleport.ComponentKey, "azure_msi")
	}

	if m.Secret == "" {
		return trace.BadParameter("missing Secret")
	}
	if m.Identity == "" {
		return trace.BadParameter("missing Identity")
	}
	if m.TenantID == "" {
		return trace.BadParameter("missing TenantID")
	}
	if m.ClientID == "" {
		return trace.BadParameter("missing ClientID")
	}
	return nil
}

func (m *AzureMSIMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	if req.Host == types.TeleportAzureMSIEndpoint {
		if err := m.msiEndpoint(rw, req); err != nil {
			m.Log.WarnContext(req.Context(), "Bad MSI request", "error", err)
			trace.WriteError(rw, trace.Wrap(err))
		}
		return true
	}

	return false
}

// SetPrivateKey updates the private key.
func (m *AzureMSIMiddleware) SetPrivateKey(privateKey crypto.Signer) {
	m.privateKeyMu.Lock()
	defer m.privateKeyMu.Unlock()
	m.privateKey = privateKey
}
func (m *AzureMSIMiddleware) getPrivateKey() (crypto.Signer, error) {
	m.privateKeyMu.RLock()
	defer m.privateKeyMu.RUnlock()
	if m.privateKey == nil {
		// Use a plain error to return status code 500.
		return nil, trace.Errorf("missing private key set in AzureMSIMiddleware")
	}
	return m.privateKey, nil
}

func (m *AzureMSIMiddleware) msiEndpoint(rw http.ResponseWriter, req *http.Request) error {
	// request validation
	if req.URL.Path != ("/" + m.Secret) {
		return trace.BadParameter("invalid secret")
	}

	metadata := req.Header.Get("Metadata")
	if metadata != "true" {
		return trace.BadParameter("expected Metadata header with value 'true'")
	}

	err := req.ParseForm()
	if err != nil {
		return trace.Wrap(err)
	}

	resource := req.Form.Get("resource")
	if resource == "" {
		return trace.BadParameter("missing value for parameter 'resource'")
	}

	// check that msi_res_id matches expected Azure Identity
	requestedAzureIdentity := req.Form.Get("msi_res_id")
	if requestedAzureIdentity != m.Identity {
		m.Log.WarnContext(req.Context(), "Requested unexpected identity", "requested_identity", requestedAzureIdentity, "expected_identity", m.Identity)
		return trace.BadParameter("unexpected value for parameter 'msi_res_id': %v", requestedAzureIdentity)
	}

	respBody, err := m.fetchMSILoginResp(resource)
	if err != nil {
		return trace.Wrap(err)
	}

	m.Log.InfoContext(req.Context(), "MSI: returning token for identity", "identity", m.Identity)

	rw.Header().Add("Content-Type", "application/json; charset=utf-8")
	rw.Header().Add("Content-Length", fmt.Sprintf("%v", len(respBody)))
	rw.WriteHeader(200)
	_, _ = rw.Write(respBody)
	return nil
}

func (m *AzureMSIMiddleware) fetchMSILoginResp(resource string) ([]byte, error) {
	now := m.Clock.Now()

	notBefore := now.Add(-10 * time.Second)
	expiresOn := now.Add(time.Hour * 24 * 365)
	expiresIn := int64(expiresOn.Sub(now).Seconds())

	accessToken, err := m.toJWT(jwt.AzureTokenClaims{
		TenantID: m.TenantID,
		Resource: resource,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := map[string]any{
		"access_token":   accessToken,
		"client_id":      m.ClientID,
		"not_before":     notBefore.Unix(),
		"expires_on":     expiresOn.Unix(),
		"expires_in":     expiresIn,
		"ext_expires_in": expiresIn,
		"token_type":     "Bearer",
		"resource":       resource,
	}

	out, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

func (m *AzureMSIMiddleware) toJWT(claims jwt.AzureTokenClaims) (string, error) {
	privateKey, err := m.getPrivateKey()
	if err != nil {
		return "", trace.Wrap(err)
	}
	// Create a new key that can sign and verify tokens.
	key, err := jwt.New(&jwt.Config{
		Clock:       m.Clock,
		PrivateKey:  privateKey,
		ClusterName: types.TeleportAzureMSIEndpoint, // todo get cluster name
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := key.SignAzureToken(claims)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}
