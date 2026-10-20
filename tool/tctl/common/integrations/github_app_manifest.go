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

package integrations

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type githubApp struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Owner        struct {
		ID int64 `json:"id"`
	} `json:"owner"`
}

// githubAppLocalServer is the local server for the GitHub App manifest flow.
// "/" posts the manifest to GitHub, which redirects to "/callback" with a code
// to exchange for the created app.
type githubAppLocalServer struct {
	*http.Server
	// url is the URL of the local server to open in the browser.
	url string
	// state protects the callback from requests not redirected by GitHub for
	// this app creation.
	state string
	// plan is the plan of the app to create.
	plan *githubAddPlan
	// exchangeOnce makes sure the code from GitHub is exchanged once, which
	// sends the created app to appC or the error to errC.
	exchangeOnce sync.Once
	appC         chan *githubApp
	errC         chan error
	// githubClient is the HTTP client to exchange the code with GitHub.
	githubClient *http.Client
}

func newGitHubAppLocalServer(plan *githubAddPlan) (*githubAppLocalServer, error) {
	s, err := newUnstartedGitHubAppLocalServer(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.url = "http://" + listener.Addr().String()
	go s.Server.Serve(listener)
	return s, nil
}

func newUnstartedGitHubAppLocalServer(plan *githubAddPlan) (*githubAppLocalServer, error) {
	githubClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	githubClient.Timeout = apidefaults.DefaultIOTimeout
	state, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &githubAppLocalServer{
		state:        state,
		plan:         plan,
		appC:         make(chan *githubApp, 1),
		errC:         make(chan error, 1),
		githubClient: githubClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /callback", s.handleCallback)
	s.Server = &http.Server{
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
	}
	return s, nil
}

// wait waits for GitHub to redirect back with the created app, for as long as
// an SSO login.
func (s *githubAppLocalServer) wait(ctx context.Context) (*githubApp, error) {
	timeout := defaults.SSOCallbackTimeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case app := <-s.appC:
		return app, nil
	case err := <-s.errC:
		return nil, trace.Wrap(err, "creating GitHub App")
	case <-timer.C:
		return nil, trace.LimitExceeded("timed out after %v waiting for GitHub App creation", timeout)
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}

// handleIndex serves the page that posts the manifest to GitHub. The values are
// escaped for the double-quoted attributes.
//
// https://docs.github.com/en/apps/sharing-github-apps/registering-a-github-app-from-a-manifest#github-app-manifest-parameters
func (s *githubAppLocalServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	createAppURL := fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new?state=%s", url.PathEscape(s.plan.org), url.QueryEscape(s.state))
	manifest, err := json.Marshal(map[string]any{
		"name":          s.plan.appName,
		"url":           s.plan.proxyURL,
		"redirect_url":  s.url + "/callback",
		"callback_urls": []string{s.plan.oauthCallbackURL},
		"setup_url":     s.plan.proxyURL + "/web/msg/info/integration_setup_complete",
		"public":        false,
		"default_permissions": map[string]string{
			"contents":      "write",
			"issues":        "write",
			"pull_requests": "write",
		},
	})
	if err != nil {
		http.Error(w, "Failed to create the GitHub App manifest.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><body>
<p>Redirecting to GitHub to create the app...</p>
<form id="manifest-form" method="post" action="%s">
  <input type="hidden" name="manifest" value="%s">
</form>
<script>document.getElementById("manifest-form").submit();</script>
</body></html>`, html.EscapeString(createAppURL), html.EscapeString(string(manifest)))
}

// handleCallback exchanges the code from GitHub for the created app, and
// redirects to the app installation page.
func (s *githubAppLocalServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Ignore requests with a mismatched state, so they cannot abort the app
	// creation.
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(s.state)) != 1 {
		http.Error(w, "Invalid state.", http.StatusBadRequest)
		return
	}
	// The code can only be exchanged once, e.g. not again on a refresh of the
	// page.
	exchanged := false
	s.exchangeOnce.Do(func() {
		exchanged = true
		app, err := exchangeGitHubManifestCode(r.Context(), s.githubClient, r.URL.Query().Get("code"))
		if err != nil {
			s.errC <- err
			http.Error(w, "Failed to create the GitHub App, check the terminal for details.", http.StatusInternalServerError)
			return
		}
		s.appC <- app
		installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new/permissions?target_id=%d&target_type=Organization", app.Slug, app.Owner.ID)
		http.Redirect(w, r, installURL, http.StatusFound)
	})
	if !exchanged {
		http.Error(w, "The GitHub App creation was already handled, check the terminal for details.", http.StatusConflict)
	}
}

func exchangeGitHubManifestCode(ctx context.Context, clt *http.Client, code string) (*githubApp, error) {
	if code == "" {
		return nil, trace.BadParameter("GitHub redirect did not include a code")
	}
	reqURL := fmt.Sprintf("https://api.github.com/app-manifests/%s/conversions", url.PathEscape(code))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := clt.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, trace.BadParameter("GitHub returned %d: %s", resp.StatusCode, body)
	}

	var app githubApp
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &app, nil
}
