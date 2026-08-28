package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/gravitational/trace"

	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
)

func runDeniedAppAccessCredentialMintProperty(ctx context.Context, params *TestCaseParams) error {
	denied, err := workloadclient.NewAPIClient(ctx, params.Identity)
	if err != nil {
		return trace.Wrap(err, "creating api client")
	}
	defer denied.Close()

	cert, err := mintAppCert(ctx, denied, deniedBotUsername, params.App)
	details := params.Details(map[string]any{"error": err})
	assert.Sometimes(err == nil,
		"A denied identity can mint an app credential before access is rejected",
		details)
	if err != nil {
		return trace.Wrap(err, "minting app cert")
	}

	sessionID := appSessionID(cert)
	body, statusCode, err := fetchAppBody(ctx, cert, params.App.PublicAddr, "denied-app-access")
	details = params.Details(map[string]any{
		"status_code": statusCode,
		"body":        body,
		"error":       err,
		"session_id":  sessionID,
	})

	assert.Sometimes(err == nil,
		"A denied app request can receive an HTTP response",
		details)
	if err != nil {
		return trace.Wrap(err, "fetching app body")
	}

	assert.AlwaysOrUnreachable(
		(statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices),
		"Every reachable app request using denied credentials is rejected with a non-2xx status",
		details)

	assert.AlwaysOrUnreachable(!strings.Contains(body, appExpectedBody),
		"Every reachable app request using denied credentials excludes the protected app response body",
		details)

	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices:
		return trace.Errorf("unexpected success status code %d", statusCode)
	case strings.Contains(body, appExpectedBody):
		return trace.Errorf("returned body contains app contents")
	}

	return nil
}
