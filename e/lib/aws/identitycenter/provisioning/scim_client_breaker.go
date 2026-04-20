package provisioning

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/breaker"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/defaults"
)

const (
	authBreakerFailuresBeforeTrip = 1
	authBreakerTrippedPeriod      = 15 * time.Minute
)

type SCIMClientWithBreakerConfig struct {
	SCIMConfig scimsdk.Config
	Clock      clockwork.Clock
}

// NewSCIMClientWithBreaker constructs the breaker-wrapped SCIM client used for
// provisioning traffic in the AWS Identity Center integration.
func NewSCIMClientWithBreaker(cfg SCIMClientWithBreakerConfig) (scimsdk.Client, error) {
	scimConfig := cfg.SCIMConfig
	if scimConfig.HTTPClient == nil {
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		scimConfig.HTTPClient = httpClient
	}

	httpClientWithBreaker, err := wrapHTTPClientWithAuthBreaker(scimConfig.HTTPClient, cfg.Clock, scimConfig.Log, "SCIM provisioning")
	if err != nil {
		return nil, trace.Wrap(err, "creating SCIM breaker HTTP client")
	}
	scimConfig.HTTPClient = httpClientWithBreaker

	client, err := scimsdk.New(&scimConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// wrapHTTPClientWithAuthBreaker returns a copy of the supplied HTTP client with
// its transport wrapped by a circuit breaker that trips on HTTP auth responses.
func wrapHTTPClientWithAuthBreaker(inner *http.Client, clock clockwork.Clock, log *slog.Logger, scope string) (*http.Client, error) {
	cb, err := breaker.New(breaker.Config{
		Clock: clock,
		// If there are authBreakerFailuresBeforeTrip + 1 consecutive auth failures
		// within this interval, the breaker will trip.
		Interval:      1 * time.Hour,
		TrippedPeriod: authBreakerTrippedPeriod,
		Trip:          breaker.ConsecutiveFailureTripper(authBreakerFailuresBeforeTrip),
		// 1 failed probe will retrip the breaker.
		Recover: breaker.ConsecutiveFailureTripper(0),
		// 1 successful probe will reset the breaker to standby.
		RecoveryLimit: 1,
		IsSuccessful: func(v interface{}, err error) bool {
			// This breaker is scoped to invalid credentials, not generic
			// transport failures. Network/TLS/context errors should not trip it.
			if err != nil {
				return true
			}

			// If we did not get an HTTP response to classify, treat it as
			// non-auth-related and leave the breaker unchanged.
			resp, ok := v.(*http.Response)
			if !ok || resp == nil {
				return true
			}

			switch resp.StatusCode {
			case http.StatusUnauthorized, http.StatusForbidden:
				return false
			default:
				return true
			}
		},
		OnTripped: func() {
			log.WarnContext(context.Background(), "Tripping circuit breaker due to auth issues; suppressing further requests until the next recovery attempt",
				"scope", scope,
				"failures_to_trip", authBreakerFailuresBeforeTrip+1,
				"next_retry_after", authBreakerTrippedPeriod)
		},
		OnStandBy: func() {
			log.InfoContext(context.Background(), "Resetting circuit breaker", "scope", scope)
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	transport := inner.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	wrapped := *inner
	wrapped.Transport = breaker.NewRoundTripper(cb, transport)
	return &wrapped, nil
}
