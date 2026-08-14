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

package common

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils/cert"
)

// TestKubeCertIssuer_SingleCeremony verifies the main behavior:
// an MFA-gated fan-out runs one ceremony and replays its reusable response across the rest, concurrently.
func TestKubeCertIssuer_SingleCeremony(t *testing.T) {
	t.Parallel()

	const numClusters = 5
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var ceremonies, replays atomic.Int32
		var ceremonyResp proto.MFAAuthenticateResponse
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
				return nil, trace.BadParameter("unexpected requester %v", params.RequesterName)
			}
			if params.ReusableMFAResponse == nil {
				// Fresh ceremony.
				ceremonies.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:             keyRing,
					MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
					ReusableMFAResponse: &ceremonyResp,
				}, nil
			}
			if params.ReusableMFAResponse != &ceremonyResp {
				return nil, trace.BadParameter("unexpected reusable MFA response replayed")
			}
			replays.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		start := time.Now()
		certs, err := newTestKubeCertIssuer(cc).issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.Equal(t, int32(1), ceremonies.Load())
		require.Equal(t, int32(numClusters-1), replays.Load())
		// One single-flighted ceremony plus one concurrent replay wave.
		require.Equal(t, 2*time.Second, time.Since(start))
		require.Equal(t, 1, cc.dials)

		// The idle connection closes one linger after the burst.
		time.Sleep(clusterConnLinger)
		synctest.Wait()
		require.Equal(t, 1, cc.closes)
	})
}

// TestKubeCertIssuer_UnknownScopeFallback verifies the permanent fallback
// when the auth server rejects the ceremony's challenge scope with the typed error,
// as servers that validate challenge scopes but predate this scope do.
func TestKubeCertIssuer_UnknownScopeFallback(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var multiAttempts, legacyCeremonies atomic.Int32
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RequesterName == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
				multiAttempts.Add(1)
				// The auth server does not know the reusable kube scope and rejects it with the typed error.
				return nil, trace.Wrap(&mfa.ErrUnknownChallengeScope)
			}
			legacyCeremonies.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		start := time.Now()
		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.True(t, fallbackActive(issuer.mfa), "a typed scope rejection is unambiguous, so the fallback must be permanent")
		require.Equal(t, int32(1), multiAttempts.Load(), "only the first ceremony should try the MULTI requester")
		require.Equal(t, int32(numClusters), legacyCeremonies.Load())
		// One rejected MULTI attempt plus three serial legacy ceremonies.
		require.Equal(t, 4*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_MaskedRejectionFallback verifies the per-issuance fallback
// when the auth server masks its reuse rejection behind the generic challenge-creation failure message,
// as servers that predate challenge scope validation do.
// The masked message could equally be a transient failure of a current server,
// so every fresh ceremony probes the MULTI requester again instead of falling back for good.
func TestKubeCertIssuer_MaskedRejectionFallback(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var multiAttempts, legacyCeremonies atomic.Int32
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RequesterName == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
				multiAttempts.Add(1)
				// An old auth server rejects the unknown reusable scope at challenge creation,
				// masked behind its generic challenge failure message.
				return nil, trace.AccessDenied("unable to create MFA challenges")
			}
			legacyCeremonies.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		start := time.Now()
		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.False(t, fallbackActive(issuer.mfa), "a masked rejection is ambiguous, so the fallback must not be permanent")
		require.Equal(t, int32(numClusters), multiAttempts.Load(), "every fresh ceremony should probe the MULTI requester")
		require.Equal(t, int32(numClusters), legacyCeremonies.Load())
		// Every issuance pays one rejected MULTI attempt and one serial legacy ceremony.
		require.Equal(t, 6*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_TransientCeremonyFailureRecovers verifies that
// a transient challenge-creation failure of a current auth server,
// which arrives as the same masked message as an old server's reuse rejection, cannot degrade the issuer.
// The affected issuance falls back to one legacy ceremony,
// and the next fresh ceremony takes the MULTI requester again and restores the single-ceremony flow.
func TestKubeCertIssuer_TransientCeremonyFailureRecovers(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var multiAttempts, legacyCeremonies, replays atomic.Int32
		var ceremonyResp proto.MFAAuthenticateResponse
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
				legacyCeremonies.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:     keyRing,
					MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
				}, nil
			}
			if params.ReusableMFAResponse != nil {
				replays.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:     keyRing,
					MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
				}, nil
			}
			if multiAttempts.Add(1) == 1 {
				// The auth server hiccups exactly once,
				// with the same masked message an old server uses for its reuse rejection.
				return nil, trace.AccessDenied("unable to create MFA challenges")
			}
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:             keyRing,
				MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
				ReusableMFAResponse: &ceremonyResp,
			}, nil
		}

		start := time.Now()
		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.False(t, fallbackActive(issuer.mfa))
		require.Equal(t, int32(2), multiAttempts.Load(), "the ceremony after the transient failure should probe the MULTI requester again")
		require.Equal(t, int32(1), legacyCeremonies.Load(), "only the issuance hit by the transient failure should pay a legacy ceremony")
		require.Equal(t, int32(numClusters-2), replays.Load())
		// One failed MULTI attempt, one legacy ceremony, one fresh MULTI ceremony, one replay wave.
		require.Equal(t, 4*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_ReplayRejectedFallback verifies the fallback
// when the ceremony succeeds but its replays are rejected, as in a mixed-version auth pool.
func TestKubeCertIssuer_ReplayRejectedFallback(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var multiCeremonies, rejectedReplays, legacyCeremonies atomic.Int32
		var ceremonyResp proto.MFAAuthenticateResponse
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
				legacyCeremonies.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:     keyRing,
					MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
				}, nil
			}
			if params.ReusableMFAResponse == nil {
				// The ceremony lands on a new auth server.
				multiCeremonies.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:             keyRing,
					MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
					ReusableMFAResponse: &ceremonyResp,
				}, nil
			}
			// The replays land on an old auth server that does not know the response's scope.
			rejectedReplays.Add(1)
			return nil, trace.AccessDenied(`required scope "CHALLENGE_SCOPE_USER_SESSION" is not satisfied by the given webauthn session with scope "CHALLENGE_SCOPE_KUBE_LOCAL_PROXY_MULTI"`)
		}

		start := time.Now()
		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.True(t, fallbackActive(issuer.mfa))
		require.Equal(t, int32(1), multiCeremonies.Load())
		require.Equal(t, int32(numClusters-1), rejectedReplays.Load())
		require.Equal(t, int32(numClusters-1), legacyCeremonies.Load())
		// One ceremony, one concurrent wave of rejected replays, two serial legacy ceremonies.
		require.Equal(t, 4*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_NoReusableResponse verifies the fan-out when ceremonies return no reusable response.
// With nothing to replay, every issuance may prompt, so all run serially.
func TestKubeCertIssuer_NoReusableResponse(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var ceremonies atomic.Int32
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.ReusableMFAResponse != nil {
				return nil, trace.BadParameter("no reusable MFA response should ever be replayed")
			}
			ceremonies.Add(1)
			// An old auth server: valid cert, no reusable response.
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		start := time.Now()
		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.False(t, fallbackActive(issuer.mfa))
		require.Equal(t, int32(numClusters), ceremonies.Load())
		// Every ceremony prompts, so they must run one at a time.
		require.Equal(t, 3*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_HeadlessSerialCeremonies verifies that
// headless issuances keep their dedicated requester and prompt one at a time.
// Headless approvals cannot be replayed.
func TestKubeCertIssuer_HeadlessSerialCeremonies(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var ceremonies atomic.Int32
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS {
				return nil, trace.BadParameter("unexpected requester %v", params.RequesterName)
			}
			if params.ReusableMFAResponse != nil {
				return nil, trace.BadParameter("headless issuances must not replay MFA responses")
			}
			ceremonies.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		issuer := newTestKubeCertIssuer(cc)
		issuer.tc.AllowHeadless = true

		start := time.Now()
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.Equal(t, int32(numClusters), ceremonies.Load())
		// Every headless issuance prompts for approval, one at a time.
		require.Equal(t, 3*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_ExpiredResponseSingleRefresh verifies the fan-out
// when the reusable response expires mid-wave.
// Exactly one issuance refreshes it and the rest replay the new one.
func TestKubeCertIssuer_ExpiredResponseSingleRefresh(t *testing.T) {
	t.Parallel()

	const numClusters = 5
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var ceremonies, staleReplays, freshReplays atomic.Int32
		var staleResp, freshResp proto.MFAAuthenticateResponse
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			switch params.ReusableMFAResponse {
			case nil:
				// A ceremony: the first produces the expiring response, the second its replacement.
				if ceremonies.Add(1) == 1 {
					return &client.IssueUserCertsWithMFAResult{
						KeyRing:             keyRing,
						MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
						ReusableMFAResponse: &staleResp,
					}, nil
				} else {
					return &client.IssueUserCertsWithMFAResult{
						KeyRing:             keyRing,
						MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
						ReusableMFAResponse: &freshResp,
					}, nil
				}
			case &staleResp:
				if !params.FailOnExpiredReusableMFAResponse {
					return nil, trace.BadParameter("replays must fail on expired reusable MFA responses instead of running their own ceremonies")
				}
				staleReplays.Add(1)
				return nil, trace.Wrap(&mfa.ErrExpiredReusableMFAResponse)
			case &freshResp:
				freshReplays.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:     keyRing,
					MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
				}, nil
			default:
				return nil, trace.BadParameter("unexpected reusable MFA response replayed")
			}
		}

		start := time.Now()
		certs, err := newTestKubeCertIssuer(cc).issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.Equal(t, int32(2), ceremonies.Load(), "exactly one issuance should refresh the expired response")
		require.Equal(t, int32(numClusters-1), staleReplays.Load())
		require.Equal(t, int32(numClusters-2), freshReplays.Load())
		// Ceremony, concurrent stale replays, refresh ceremony, concurrent fresh replays.
		require.Equal(t, 4*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_MFAOffNoCeremony verifies that
// clusters without per-session MFA share one unrouted cert issued with no ceremony,
// held in memory rather than saved to the key store.
func TestKubeCertIssuer_MFAOffNoCeremony(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var issuances int
		cc := &fakeKubeCertClient{mfaRequired: false}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			issuances++
			if params.ReusableMFAResponse != nil {
				return nil, trace.BadParameter("no MFA ceremony expected for MFA-off clusters")
			}
			if params.KubernetesCluster != "" {
				return nil, trace.BadParameter("expected an unrouted request, got cluster %q", params.KubernetesCluster)
			}
			if params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
				return nil, trace.BadParameter("unexpected requester %v", params.RequesterName)
			}
			if params.MFACheck.GetRequired() {
				return nil, trace.BadParameter("unrouted issuance must assert MFA is not required")
			}
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)

		require.Equal(t, 1, issuances)
		require.Len(t, certs, 1)
		require.Contains(t, certs, alpnproxy.KubeClusterKey{TeleportCluster: "root", KubeCluster: ""})
		require.Zero(t, cc.saves)
	})
}

// TestKubeCertIssuer_RootAndLeafTeleportClusters verifies a fan-out spanning Teleport clusters:
// per-cluster MFA checks, per-cluster cert routing, one ceremony replayed across all.
func TestKubeCertIssuer_RootAndLeafTeleportClusters(t *testing.T) {
	t.Parallel()

	clusters := kubeconfig.LocalProxyClusters{
		{TeleportCluster: "root", KubeCluster: "kube-root-0"},
		{TeleportCluster: "root", KubeCluster: "kube-root-1"},
		{TeleportCluster: "leaf", KubeCluster: "kube-leaf-0"},
	}
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		routes := make(map[string]string, len(clusters))
		for _, cluster := range clusters {
			routes[cluster.KubeCluster] = cluster.TeleportCluster
		}

		var ceremonies, replays atomic.Int32
		var ceremonyResp proto.MFAAuthenticateResponse
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.RouteToCluster != routes[params.KubernetesCluster] {
				return nil, trace.BadParameter("unexpected route %q for kube cluster %q", params.RouteToCluster, params.KubernetesCluster)
			}
			if params.ReusableMFAResponse == nil {
				ceremonies.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:             keyRing,
					MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
					ReusableMFAResponse: &ceremonyResp,
				}, nil
			}
			replays.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		start := time.Now()
		certs, err := newTestKubeCertIssuer(cc).issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, len(clusters))
		require.Equal(t, int32(1), ceremonies.Load(), "one ceremony should cover all Teleport clusters")
		require.Equal(t, int32(len(clusters)-1), replays.Load())
		// Only the prefetch connects auth clients, one per Teleport cluster. The issuance wave connects nothing.
		require.ElementsMatch(t, []string{"root", "leaf"}, cc.connects, "MFA requirements should be fetched from each Teleport cluster")
		// One single-flighted ceremony plus one concurrent replay wave.
		require.Equal(t, 2*time.Second, time.Since(start))
	})
}

// TestIsMFAReuseRejected verifies that every rejection shape produced by auth servers
// that predate the reusable kube MFA flow is recognized.
func TestIsMFAReuseRejected(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		err      error
		rejected bool
	}{
		{
			name:     "typed error, the server validates challenge scopes",
			err:      trace.Wrap(&mfa.ErrUnknownChallengeScope),
			rejected: true,
		},
		{
			name:     "response scope unknown to the server, rejected at validation",
			err:      trace.AccessDenied(`required scope "CHALLENGE_SCOPE_USER_SESSION" is not satisfied by the given webauthn session with scope "CHALLENGE_SCOPE_KUBE_LOCAL_PROXY_MULTI"`),
			rejected: true,
		},
		{
			name:     "server knows the scope but forbids reuse for the requester",
			err:      trace.AccessDenied("the given webauthn session allows reuse, but reuse is not permitted in this context"),
			rejected: true,
		},
		{
			name: "creation-time rejection message, which the server masks before it ever crosses the wire",
			err:  trace.BadParameter("mfa challenges with scope CHALLENGE_SCOPE_KUBE_LOCAL_PROXY_MULTI cannot allow reuse"),
			// Not matched: the server's generic challenge-creation mask replaces it,
			// and the masked message is ambiguous, handled by isMFAReuseRejectionSuspected.
			rejected: false,
		},
		{
			name:     "masked challenge-creation failure is ambiguous, not an unambiguous rejection",
			err:      trace.AccessDenied("unable to create MFA challenges"),
			rejected: false,
		},
		{
			name:     "unrelated access denied",
			err:      trace.AccessDenied("access to kube cluster denied"),
			rejected: false,
		},
		{
			name:     "matching message but wrong error type",
			err:      trace.ConnectionProblem(nil, "reuse is not permitted"),
			rejected: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.rejected, isMFAReuseRejected(tt.err))
		})
	}
}

func TestIsMFAReuseRejectionSuspected(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		err       error
		suspected bool
	}{
		{
			name:      "masked challenge-creation failure",
			err:       trace.AccessDenied("unable to create MFA challenges"),
			suspected: true,
		},
		{
			name:      "unrelated access denied",
			err:       trace.AccessDenied("access to kube cluster denied"),
			suspected: false,
		},
		{
			name:      "matching message but wrong error type",
			err:       trace.ConnectionProblem(nil, "unable to create MFA challenges"),
			suspected: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.suspected, isMFAReuseRejectionSuspected(tt.err))
		})
	}
}

// TestKubeCertIssuer_CanceledContextFailsIssuance verifies that
// an MFA-gated issuance whose context is canceled while it takes the ceremony lock
// returns the cancellation error instead of retrying the lock forever.
// A canceled context is how a concurrent peer's failure reaches the other issuances.
func TestKubeCertIssuer_CanceledContextFailsIssuance(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ceremonyStarted := make(chan struct{})
		finishCeremony := make(chan struct{})
		cc := &fakeKubeCertClient{mfaRequired: true}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			close(ceremonyStarted)
			<-finishCeremony
			return nil, trace.ConnectionProblem(nil, "the ceremony fails once released")
		}
		issuer := newTestKubeCertIssuer(cc)
		mfaCheck := &proto.IsMFARequiredResponse{MFARequired: proto.MFARequired_MFA_REQUIRED_YES, Required: true}

		// The first issuance takes the ceremony lock and blocks mid-ceremony.
		firstDone := make(chan struct{})
		var firstErr error
		go func() {
			defer close(firstDone)
			_, firstErr = issuer.IssueCert(t.Context(), "root", "kube-a", mfaCheck)
		}()
		<-ceremonyStarted

		// The second issuance waits for the lock.
		// Canceling its context, as a concurrent peer's failure does, must fail it while the holder still runs.
		ctx, cancel := context.WithCancel(t.Context())
		secondDone := make(chan struct{})
		var secondErr error
		go func() {
			defer close(secondDone)
			_, secondErr = issuer.IssueCert(ctx, "root", "kube-b", mfaCheck)
		}()
		synctest.Wait() // the second issuance is parked on the ceremony lock
		cancel()
		<-secondDone
		require.ErrorIs(t, secondErr, context.Canceled)

		// The ceremony holder is unaffected by the canceled waiter.
		close(finishCeremony)
		<-firstDone
		require.ErrorContains(t, firstErr, "the ceremony fails once released")
	})
}

// TestKubeCertIssuer_MixedFleet verifies that
// only the clusters without per-session MFA collapse onto the shared cert,
// while MFA-gated ones keep their own routed certs.
func TestKubeCertIssuer_MixedFleet(t *testing.T) {
	t.Parallel()

	clusters := newTestKubeClusters(4)
	keyRing := newTestKubeKeyRing(t, clusters)
	// kube-0 and kube-1 are MFA-gated; kube-2 and kube-3 are not.
	mfaOn := func(kubeCluster string) bool {
		return kubeCluster == "kube-0" || kubeCluster == "kube-1"
	}

	synctest.Test(t, func(t *testing.T) {
		var routed, unrouted atomic.Int32
		var ceremonyResp proto.MFAAuthenticateResponse
		cc := &fakeKubeCertClient{mfaRequiredFor: mfaOn}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.KubernetesCluster == "" {
				unrouted.Add(1)
				return &client.IssueUserCertsWithMFAResult{
					KeyRing:     keyRing,
					MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
				}, nil
			}
			if !mfaOn(params.KubernetesCluster) {
				return nil, trace.BadParameter("cluster %q has no MFA and must use the shared cert", params.KubernetesCluster)
			}
			routed.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:             keyRing,
				MFARequired:         proto.MFARequired_MFA_REQUIRED_YES,
				ReusableMFAResponse: &ceremonyResp,
			}, nil
		}

		certs, err := newTestKubeCertIssuer(cc).issueCerts(t.Context(), clusters)
		require.NoError(t, err)

		require.Equal(t, int32(2), routed.Load())
		require.Equal(t, int32(1), unrouted.Load())
		require.Len(t, certs, 3)
		require.Zero(t, cc.saves)
	})
}

// TestKubeCertIssuer_SharedCertPerTeleportCluster verifies that
// the shared cert is scoped to a Teleport cluster.
// A fleet spanning root and leaf gets one unrouted cert each, never one for both.
func TestKubeCertIssuer_SharedCertPerTeleportCluster(t *testing.T) {
	t.Parallel()

	clusters := kubeconfig.LocalProxyClusters{
		{TeleportCluster: "root", KubeCluster: "kube-root-0"},
		{TeleportCluster: "root", KubeCluster: "kube-root-1"},
		{TeleportCluster: "leaf", KubeCluster: "kube-leaf-0"},
	}
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var routes []string
		cc := &fakeKubeCertClient{mfaRequired: false}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.KubernetesCluster != "" {
				return nil, trace.BadParameter("expected an unrouted request, got cluster %q", params.KubernetesCluster)
			}
			mu.Lock()
			routes = append(routes, params.RouteToCluster)
			mu.Unlock()
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		certs, err := newTestKubeCertIssuer(cc).issueCerts(t.Context(), clusters)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, []string{"leaf", "root"}, routes)
		require.Len(t, certs, 2)
	})
}

// TestKubeCertIssuer_HeadlessSkipsSharedCert verifies that
// headless keeps issuing per-cluster certs.
// Its requester carries distinct server-side handling and cannot request an unrouted cert.
func TestKubeCertIssuer_HeadlessSkipsSharedCert(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var issuances int
		cc := &fakeKubeCertClient{mfaRequired: false}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			issuances++
			if params.KubernetesCluster == "" {
				return nil, trace.BadParameter("headless must not request an unrouted cert")
			}
			if params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS {
				return nil, trace.BadParameter("unexpected requester %v", params.RequesterName)
			}
			if params.MFACheck == nil {
				return nil, trace.BadParameter("headless issuance must carry a prefetched MFA check")
			}
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		issuer := newTestKubeCertIssuer(cc)
		issuer.tc.AllowHeadless = true

		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Equal(t, numClusters, issuances)
		require.Len(t, certs, numClusters)
		require.Equal(t, numClusters, cc.saves)
	})
}

// TestKubeCertIssuer_UnroutedRequesterGuard verifies that
// the issuer refuses to send an unrouted request under any other requester.
// Without a cluster route the requester is the only thing marking it as Kubernetes usage,
// so the wrong one yields an unrestricted certificate rather than an error from the server.
func TestKubeCertIssuer_UnroutedRequesterGuard(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		cc := &fakeKubeCertClient{}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			return nil, trace.BadParameter("issuance must not be attempted")
		}

		_, err := newTestKubeCertIssuer(cc).requestCert(t.Context(), cc, client.ReissueParams{
			RouteToCluster: "root",
			RequesterName:  proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY,
		})
		require.True(t, trace.IsBadParameter(err), "expected bad parameter but got %v", err)
		require.ErrorContains(t, err, "unrouted Kubernetes certificates can only be requested by")
	})
}

// TestKubeCertIssuer_HeadlessUnroutedRejected verifies that
// a headless session cannot issue a shared cert even if asked directly.
// Headless carries its own requester, which auth does not accept for an unrouted request.
func TestKubeCertIssuer_HeadlessUnroutedRejected(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		cc := &fakeKubeCertClient{}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			return nil, trace.BadParameter("issuance must not be attempted")
		}
		issuer := newTestKubeCertIssuer(cc)
		issuer.tc.AllowHeadless = true

		_, err := issuer.IssueCert(t.Context(), "root", "" /*kubeCluster*/, nil /*mfaCheck*/)
		require.True(t, trace.IsBadParameter(err), "expected bad parameter but got %v", err)
		require.ErrorContains(t, err, "unrouted Kubernetes certificates can only be requested by")
	})
}

// TestKubeCertIssuer_UnroutedRejectedFallback verifies that
// an auth server that refuses the unrouted request does not break the proxy.
// The issuer falls back to per-cluster certs and latches,
// so a later burst does not retry a request that server refuses.
func TestKubeCertIssuer_UnroutedRejectedFallback(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	for _, tt := range []struct {
		name      string
		rejection error
	}{
		{
			// Every auth server predating the shared cert.
			name:      "old auth server",
			rejection: trace.BadParameter("missing KubernetesCluster field in a kubernetes-only UserCertsRequest"),
		},
		{
			// A scoped identity is only allowed a Kubernetes cert that names a cluster.
			name:      "scoped identity",
			rejection: trace.Wrap(services.ErrScopedIdentity, "generating scoped user cert for unsupported usage %q", proto.UserCertsRequest_Kubernetes),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var unroutedAttempts, routedIssuances atomic.Int32
				cc := &fakeKubeCertClient{mfaRequired: false}
				cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
					if params.KubernetesCluster == "" {
						unroutedAttempts.Add(1)
						return nil, tt.rejection
					}
					routedIssuances.Add(1)
					return &client.IssueUserCertsWithMFAResult{
						KeyRing:     keyRing,
						MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
					}, nil
				}

				issuer := newTestKubeCertIssuer(cc)
				start := time.Now()
				certs, err := issuer.issueCerts(t.Context(), clusters)
				require.NoError(t, err)
				require.Equal(t, int32(1), unroutedAttempts.Load())
				require.Equal(t, int32(numClusters), routedIssuances.Load())

				// Every cluster is served by its own cert, and no shared entry lingers from the attempt.
				require.Len(t, certs, numClusters)
				require.NotContains(t, certs, alpnproxy.KubeClusterKey{TeleportCluster: "root", KubeCluster: ""})
				require.Equal(t, numClusters, cc.saves)
				// One rejected unrouted attempt, then the per-cluster issuances.
				// Those write to the key store, so they must not run concurrently.
				require.Equal(t, (1+numClusters)*time.Second, time.Since(start))

				// A later burst must not retry the shape this server already refused.
				certs, err = issuer.issueCerts(t.Context(), clusters)
				require.NoError(t, err)
				require.Equal(t, int32(1), unroutedAttempts.Load(), "the rejection must latch")
				require.Equal(t, int32(2*numClusters), routedIssuances.Load())
				require.Len(t, certs, numClusters)
			})
		})
	}
}

// TestKubeCertIssuer_ReissueUnroutedRejectedFallback verifies that
// an auth server that refuses the unrouted request cannot strand a running proxy,
// as one reached mid-session during a rolling upgrade would.
// The reissue gives the requesting cluster its own cert and latches,
// so the clusters the shared cert served convert as their own reissues come due.
func TestKubeCertIssuer_ReissueUnroutedRejectedFallback(t *testing.T) {
	t.Parallel()

	const kubeCluster = "kube-0"
	clusters := newTestKubeClusters(1)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var unroutedAttempts, routedIssuances atomic.Int32
		cc := &fakeKubeCertClient{mfaRequired: false}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.KubernetesCluster == "" {
				unroutedAttempts.Add(1)
				return nil, trace.BadParameter("missing KubernetesCluster field in a kubernetes-only UserCertsRequest")
			}
			routedIssuances.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		issuer := newTestKubeCertIssuer(cc)
		cert, storeUnder, err := issuer.ReissueSharedCert(t.Context(), "root", kubeCluster)
		require.NoError(t, err, "the reissue must recover instead of leaving the cluster unreachable")
		require.NotNil(t, cert)
		require.Equal(t, kubeCluster, storeUnder, "the cluster must be moved onto its own cert")
		require.Equal(t, int32(1), unroutedAttempts.Load())
		require.Equal(t, int32(1), routedIssuances.Load())
		require.True(t, issuer.sharedCertUnsupported.Load(), "the rejection must latch")

		// A later reissue must not retry the shape this server already refused.
		_, storeUnder, err = issuer.ReissueSharedCert(t.Context(), "root", kubeCluster)
		require.NoError(t, err)
		require.Equal(t, kubeCluster, storeUnder)
		require.Equal(t, int32(1), unroutedAttempts.Load(), "the rejection must latch")
		require.Equal(t, int32(2), routedIssuances.Load())
	})
}

// TestKubeCertIssuer_UnroutedErrorNotSwallowed verifies that
// only the old-server rejection triggers the per-cluster fallback.
// Any other failure has to surface, or a real problem would show up as a silent loss of the shared cert.
func TestKubeCertIssuer_UnroutedErrorNotSwallowed(t *testing.T) {
	t.Parallel()

	clusters := newTestKubeClusters(3)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		var routedIssuances atomic.Int32
		cc := &fakeKubeCertClient{mfaRequired: false}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.KubernetesCluster == "" {
				return nil, trace.AccessDenied("user has no access to Kubernetes clusters")
			}
			routedIssuances.Add(1)
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		issuer := newTestKubeCertIssuer(cc)
		_, err := issuer.issueCerts(t.Context(), clusters)
		require.True(t, trace.IsAccessDenied(err), "expected access denied but got %v", err)
		require.Zero(t, routedIssuances.Load(), "an unrelated failure must not fall back to per-cluster issuance")
		require.False(t, issuer.sharedCertUnsupported.Load(), "an unrelated failure must not latch")
	})
}
func newTestKubeClusters(n int) kubeconfig.LocalProxyClusters {
	clusters := make(kubeconfig.LocalProxyClusters, 0, n)
	for i := range n {
		clusters = append(clusters, kubeconfig.LocalProxyCluster{
			TeleportCluster: "root",
			KubeCluster:     fmt.Sprintf("kube-%d", i),
		})
	}
	return clusters
}

func newTestKubeKeyRing(t *testing.T, clusters kubeconfig.LocalProxyClusters) *client.KeyRing {
	t.Helper()
	creds, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil, nil, nil)
	require.NoError(t, err)
	priv, err := keys.ParsePrivateKey(creds.PrivateKey)
	require.NoError(t, err)
	keyRing := &client.KeyRing{KubeTLSCredentials: make(map[string]client.TLSCredential)}
	for _, cluster := range clusters {
		keyRing.KubeTLSCredentials[cluster.KubeCluster] = client.TLSCredential{PrivateKey: priv, Cert: creds.Cert}
	}
	// An unrouted request is keyed by its empty Kubernetes cluster,
	// as the real client does when storing the credential it gets back.
	keyRing.KubeTLSCredentials[""] = client.TLSCredential{PrivateKey: priv, Cert: creds.Cert}
	return keyRing
}

// fallbackActive reports whether the issuer permanently dropped to the legacy per-cluster requester.
func fallbackActive(m *reusableMFA) bool {
	requester, _ := m.State()
	return requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
}

func newTestKubeCertIssuer(cc *fakeKubeCertClient) *kubeCertIssuer {
	return &kubeCertIssuer{
		tc:       &client.TeleportClient{},
		mfa:      newReusableMFA(),
		keyStore: cc,
		conn:     &clusterConn{dialer: cc},
	}
}

type fakeMFAAuthClient struct {
	authclient.ClientI
	required bool
	// requiredFor overrides required per Kubernetes cluster when set.
	requiredFor func(kubeCluster string) bool
	// err fails the check instead of answering it, when set.
	err error
}

func (f *fakeMFAAuthClient) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	required := f.required
	if f.requiredFor != nil {
		required = f.requiredFor(req.GetKubernetesCluster())
	}
	if required {
		return &proto.IsMFARequiredResponse{
			Required:    true,
			MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
		}, nil
	} else {
		return &proto.IsMFARequiredResponse{
			Required:    false,
			MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}, nil
	}
}

func (f *fakeMFAAuthClient) Close() error { return nil }

type fakeKubeCertClient struct {
	mfaRequired bool
	issueFn     func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error)
	keyRings    map[string]*client.KeyRing
	// mfaRequiredFor overrides mfaRequired per Kubernetes cluster, for fleets
	// where only some clusters are MFA-gated.
	mfaRequiredFor func(kubeCluster string) bool
	// mfaCheckErr fails the MFA requirement check instead of answering it, when set.
	mfaCheckErr error

	mu       sync.Mutex
	connects []string
	dials    int
	closes   int
	saves    int
}

// GetKeyRing implements [kubeKeyStore] to serve the injected key rings.
func (f *fakeKubeCertClient) GetKeyRing(clusterName string, opts ...client.CertOption) (*client.KeyRing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if keyRing, ok := f.keyRings[clusterName]; ok {
		return keyRing, nil
	}
	return nil, trace.NotFound("no key ring for cluster %q", clusterName)
}

// AddKubeKeyRing implements [kubeKeyStore] to count the saves.
func (f *fakeKubeCertClient) AddKubeKeyRing(keyRing *client.KeyRing) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves++
	return nil
}

// DialCluster implements [clusterDialer] to count dials.
func (f *fakeKubeCertClient) DialCluster(ctx context.Context) (kubeCertClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dials++
	return f, nil
}

// IssueUserCertsWithMFA implements [kubeCertClient] to call the injected issueFn.
func (f *fakeKubeCertClient) IssueUserCertsWithMFA(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
	// Each issuance takes one second of fake time, so tests assert scheduling through elapsed time.
	time.Sleep(time.Second)
	return f.issueFn(ctx, params)
}

// Close implements [kubeCertClient] to count closes.
func (f *fakeKubeCertClient) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
	return nil
}

// ConnectToCluster implements [clusterConnector] to record the Teleport cluster name and return a fake auth client.
func (f *fakeKubeCertClient) ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	f.mu.Lock()
	f.connects = append(f.connects, clusterName)
	f.mu.Unlock()
	return &fakeMFAAuthClient{required: f.mfaRequired, requiredFor: f.mfaRequiredFor, err: f.mfaCheckErr}, nil
}
