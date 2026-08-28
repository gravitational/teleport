package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	workloadclient "github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/client"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
)

func runAllowedAppAccessTbotCredProperty(ctx context.Context, params *TestCaseParams) error {
	certDir := filepath.Join(testenv.CredsDir(), params.Identity)
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(certDir, "tlscert"),
		filepath.Join(certDir, "key"),
	)
	if err != nil {
		return trace.Wrap(err, "loading app cert")
	}
	sessionID := appSessionID(cert)

	assert.Always(sessionID != "",
		"Every issued app certificate contains a non-empty session ID",
		params.Details(nil))
	if sessionID == "" {
		return trace.BadParameter("issued app cert missing session id")
	}

	marker, err := uuid.NewV7()
	if err != nil {
		return trace.Wrap(err, "generating marker")
	}

	// Session chunk events are TTLd by session ID, when executing in parallel it is possible that
	// another concurrent run of this test already emitted this event. We add some window before and after
	// to allow the search to match the cached events.
	start := time.Now().Add(-(auditSessionChunkTTL*2 + maxTolerableClockJitter))
	body, statusCode, err := fetchAppBody(ctx, cert, params.App.PublicAddr, marker.String())
	// Padding to account for delay between session ending and the event
	// being emitted.
	end := time.Now().Add(auditEventEmitDeadline + (auditSessionChunkTTL * 2) + maxTolerableClockJitter)

	assert.Sometimes(err == nil && statusCode == http.StatusOK,
		"An HTTP app request using tbot credentials can return HTTP 200",
		params.Details(
			map[string]any{
				"status_code": statusCode,
				"body":        body,
				"error":       err,
				"session_id":  sessionID,
			},
		))

	if err != nil {
		return trace.Wrap(err, "fetching app")
	}

	if statusCode != http.StatusOK {
		return trace.Errorf("unexpected HTTP status code: %d", statusCode)
	}

	// If we are here then the body must contain the correct signature
	assert.AlwaysOrUnreachable(strings.Contains(body, appExpectedBody),
		"Every reachable authorized HTTP app request using tbot credentials includes the expected app response body",
		params.Details(
			map[string]any{
				"body":       body,
				"session_id": sessionID,
			},
		))

	return trace.Wrap(params.assertExistingSessionAppAccessAuditEvents(ctx, sessionID, start, end))
}

func runAllowedAppAccessMintedCredProperty(ctx context.Context, params *TestCaseParams) error {
	appClient, err := workloadclient.NewAPIClient(ctx, params.Identity)
	if err != nil {
		return trace.Wrap(err, "creating api client")
	}
	defer appClient.Close()

	start := time.Now().Add(-maxTolerableClockJitter)
	cert, err := mintAppCert(ctx, appClient, allowedBotUsername, params.App)
	assert.Sometimes(err == nil,
		"An authorized identity can mint an HTTP app credential",
		params.Details(map[string]any{"error": err}))
	if err != nil {
		return trace.Wrap(err, "minting app cert")
	}

	sessionID := appSessionID(cert)
	assert.Always(sessionID != "", "Every minted HTTP app credential contains a non-empty session ID", params.Details(nil))
	if sessionID == "" {
		return trace.BadParameter("minted app cert missing session id")
	}

	marker, err := uuid.NewV7()
	if err != nil {
		return trace.Wrap(err, "generating marker")
	}

	body, statusCode, err := fetchAppBody(ctx, cert, params.App.PublicAddr, marker.String())
	end := time.Now().Add(auditEventEmitDeadline + maxTolerableClockJitter)

	assert.Sometimes(err == nil && statusCode == http.StatusOK,
		"An HTTP app request using minted credentials can return HTTP 200",
		params.Details(map[string]any{
			"status_code": statusCode,
			"body":        body,
			"error":       err,
			"session_id":  sessionID,
		}))
	if err != nil {
		return trace.Wrap(err, "fetching app")
	}
	if statusCode != http.StatusOK {
		return trace.Errorf("unexpected HTTP status code: %d", statusCode)
	}

	assert.AlwaysOrUnreachable(strings.Contains(body, appExpectedBody),
		"Every reachable authorized HTTP app request using minted credentials includes the expected app response body",
		params.Details(map[string]any{
			"body":       body,
			"session_id": sessionID,
		}))

	return trace.Wrap(params.assertMintedAppAccessAuditEvents(ctx, sessionID, start, end))
}

func runAllowedTCPAppAccessMintedCredProperty(ctx context.Context, params *TestCaseParams) error {
	appClient, err := workloadclient.NewAPIClient(ctx, params.Identity)
	if err != nil {
		return trace.Wrap(err, "creating api client")
	}
	defer appClient.Close()

	start := time.Now().Add(-maxTolerableClockJitter)
	cert, err := mintAppCert(ctx, appClient, allowedBotUsername, params.App)
	assert.Sometimes(err == nil,
		"An authorized identity can mint a TCP app credential",
		params.Details(map[string]any{"error": err}))
	if err != nil {
		return trace.Wrap(err, "minting TCP app cert")
	}

	sessionID := appSessionID(cert)
	assert.Always(sessionID != "", "Every minted TCP app credential contains a non-empty session ID", params.Details(nil))
	if sessionID == "" {
		return trace.BadParameter("minted TCP app cert missing session id")
	}

	marker, err := uuid.NewV7()
	if err != nil {
		return trace.Wrap(err, "generating marker")
	}

	body, statusCode, err := fetchTCPAppBody(ctx, cert, params.App.PublicAddr, marker.String())
	end := time.Now().Add(auditEventEmitDeadline + maxTolerableClockJitter)

	assert.Sometimes(err == nil && statusCode == http.StatusOK,
		"A TCP app request using minted credentials can proxy to the app and return HTTP 200",
		params.Details(map[string]any{
			"status_code": statusCode,
			"body":        body,
			"error":       err,
			"session_id":  sessionID,
		}))
	if err != nil {
		return trace.Wrap(err, "fetching TCP app")
	}
	if statusCode != http.StatusOK {
		return trace.Errorf("unexpected HTTP status code from TCP app: %d", statusCode)
	}

	assert.AlwaysOrUnreachable(strings.Contains(body, appExpectedBody),
		"Every reachable authorized TCP app request using minted credentials includes the expected app response body",
		params.Details(map[string]any{
			"body":       body,
			"session_id": sessionID,
		}))

	return trace.Wrap(params.assertTCPAppAccessAuditEvents(ctx, sessionID, start, end))
}
