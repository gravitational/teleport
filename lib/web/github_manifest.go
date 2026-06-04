/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
)

const githubManifestRedirectHTML = `<!DOCTYPE html>
<html><body>
<p>Redirecting to GitHub...</p>
<form id="manifest-form" method="post" action="%s">
  <input type="hidden" name="manifest" id="manifest-input">
</form>
<script>
document.getElementById("manifest-input").value = JSON.stringify(%s)
document.getElementById("manifest-form").submit()
</script>
</body></html>`

// githubManifestRedirect builds the GitHub App manifest and serves an HTML page
// that auto-submits a form POST to GitHub to create the app.
func (h *Handler) githubManifestRedirectRaw(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	org := r.URL.Query().Get("org")
	if org == "" {
		http.Error(w, "missing org parameter", http.StatusBadRequest)
		return
	}
	sshEnabled := r.URL.Query().Get("ssh") == "true"
	httpEnabled := r.URL.Query().Get("http") == "true"

	proxyAddr := h.cfg.ProxyPublicAddrs[0].String()
	proxyHost := h.cfg.ProxyPublicAddrs[0].Host()
	baseURL := fmt.Sprintf("https://%s", proxyAddr)

	state := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(
		`{"org":%q,"ssh":%t,"http":%t}`, org, sshEnabled, httpEnabled,
	)))

	manifest := map[string]interface{}{
		"name":         truncateAppName(fmt.Sprintf("Teleport %s", proxyHost), 34),
		"url":          baseURL,
		"redirect_url": fmt.Sprintf("%s/web/integrations/new/github", baseURL),
		"callback_urls": []string{
			fmt.Sprintf("%s/v1/webapi/github/callback", baseURL),
		},
		"hook_attributes": map[string]interface{}{
			"url":    fmt.Sprintf("%s/webapi/github/webhook", baseURL),
			"active": false,
		},
		"public": false,
		"default_permissions": map[string]string{
			"contents":      "write",
			"issues":        "write",
			"pull_requests": "write",
		},
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	action := fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new?state=%s", org, state)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; form-action https://github.com")
	fmt.Fprintf(w, githubManifestRedirectHTML, action, string(manifestJSON))
}

// githubManifestExchangeRequest is the request body for the manifest code exchange.
type githubManifestExchangeRequest struct {
	Code        string `json:"code"`
	Org         string `json:"org"`
	SSHEnabled  bool   `json:"sshEnabled"`
	HTTPEnabled bool   `json:"httpEnabled"`
}

// githubManifestExchangeResponse is returned to the web UI after exchanging
// the manifest code and creating resources.
type githubManifestExchangeResponse struct {
	Integration string `json:"integration"`
	GitServer   string `json:"gitServer"`
	AppSlug     string `json:"appSlug"`
}

// githubManifestConversionsResponse is the response from GitHub's
// POST /app-manifests/{code}/conversions endpoint.
type githubManifestConversionsResponse struct {
	ID            int    `json:"id"`
	Slug          string `json:"slug"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	PEM           string `json:"pem"`
	WebhookSecret string `json:"webhook_secret"`
	Name          string `json:"name"`
}

// githubIntegrationManifest exchanges the manifest code with GitHub for app
// credentials, then creates the integration and git server resources.
func (h *Handler) githubIntegrationManifest(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (interface{}, error) {
	var req githubManifestExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.BadParameter("invalid request body: %v", err)
	}
	if req.Code == "" {
		return nil, trace.BadParameter("missing manifest code")
	}
	if req.Org == "" {
		return nil, trace.BadParameter("missing organization name")
	}

	ghResp, err := exchangeManifestCode(req.Code)
	if err != nil {
		return nil, trace.Wrap(err, "failed to exchange manifest code with GitHub")
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := fmt.Sprintf("github-%s", req.Org)
	integration, err := types.NewIntegrationGitHub(
		types.Metadata{Name: integrationName},
		&types.GitHubIntegrationSpecV1{
			Organization: req.Org,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cred := types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_IdSecret{
			IdSecret: &types.PluginIdSecretCredential{
				Id:     ghResp.ClientID,
				Secret: ghResp.ClientSecret,
			},
		},
	}
	if err := integration.SetCredentials(&cred); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := clt.CreateIntegration(r.Context(), integration); err != nil {
		if trace.IsAlreadyExists(err) {
			if _, err := clt.UpdateIntegration(r.Context(), integration); err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			return nil, trace.Wrap(err)
		}
	}

	var allowProtocols []string
	if req.SSHEnabled {
		allowProtocols = append(allowProtocols, types.GitProtocolSSH)
	}
	if req.HTTPEnabled {
		allowProtocols = append(allowProtocols, types.GitProtocolHTTP)
	}
	gitServer, err := types.NewGitHubServerWithName(integrationName, types.GitHubServerMetadata{
		Organization:   req.Org,
		Integration:    integrationName,
		AllowProtocols: allowProtocols,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := clt.GitServerClient().CreateGitServer(r.Context(), gitServer); err != nil {
		if trace.IsAlreadyExists(err) {
			if _, err := clt.GitServerClient().UpsertGitServer(r.Context(), gitServer); err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			return nil, trace.Wrap(err)
		}
	}

	return &githubManifestExchangeResponse{
		Integration: integrationName,
		GitServer:   integrationName,
		AppSlug:     ghResp.Slug,
	}, nil
}

func truncateAppName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen]
}

// exchangeManifestCode calls GitHub's API to exchange a manifest code for
// app credentials. No authentication is required.
func exchangeManifestCode(code string) (*githubManifestConversionsResponse, error) {
	url := fmt.Sprintf("https://api.github.com/app-manifests/%s/conversions", code)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, trace.BadParameter("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var result githubManifestConversionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}
