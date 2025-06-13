package jamf_test

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
)

func TestClient_redirectNotAllowed(t *testing.T) {
	env := testenv.MustNew(nil /* opts */)
	defer env.Close()

	// Create 2 HTTP servers:
	// * An initial, safe-looking TLS server that passes for the Jamf API.
	//   This server will downgrade the client to HTTP and redirect.
	// * A non-TLS server that captures the credentials.
	//
	// This is mainly a Jamf Plugin security scenario.
	// See https://github.com/gravitational/teleport-private/issues/916.

	var capturedUser, capturedPass string
	downgradeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser, capturedPass, _ = r.BasicAuth()
		http.Error(w, "oopsie", 500)
	}))
	defer downgradeServer.Close()

	redirServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		maps.Copy(w.Header(), r.Header)

		url := downgradeServer.URL + r.URL.Path
		http.Redirect(w, r, url, http.StatusFound)
	}))
	defer redirServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create the client. This causes automatic credential validation, thus hits
	// redirServer.
	if _, err := jamf.NewClient(ctx, jamf.ClientOpts{
		Clock:      env.Clock,
		Logger:     env.Logger,
		HTTPClient: redirServer.Client(),
		APIURL:     redirServer.URL,
		Username:   testenv.DefaultUsers[0].Username,
		Password:   testenv.DefaultUsers[0].Password,
	}); err == nil {
		t.Error("NewClient returned err=nil, want non-nil")
	}

	if capturedUser != "" || capturedPass != "" {
		t.Errorf("Username or password captured during redirect, user=%q, pass=%q", capturedUser, capturedPass)
	}
}
