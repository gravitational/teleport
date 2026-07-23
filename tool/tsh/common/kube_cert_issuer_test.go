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
		require.Equal(t, 1, cc.closes)
	})
}

// TestKubeCertIssuer_OldServerFallback verifies that
// when the auth server rejects the MULTI requester's ceremony,
// the issuer falls back to serial legacy per-cluster ceremonies.
func TestKubeCertIssuer_OldServerFallback(t *testing.T) {
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
				return nil, trace.AccessDenied("the given webauthn session allows reuse, but reuse is not permitted in this context")
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
		require.True(t, issuer.mfa.FallbackActive())
		require.Equal(t, int32(1), multiAttempts.Load(), "only the first ceremony should try the MULTI requester")
		require.Equal(t, int32(numClusters), legacyCeremonies.Load())
		// One rejected MULTI attempt plus three serial legacy ceremonies.
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
			// The replays land on an old auth server that rejects reuse.
			rejectedReplays.Add(1)
			return nil, trace.AccessDenied("the given webauthn session allows reuse, but reuse is not permitted in this context")
		}

		start := time.Now()
		issuer := newTestKubeCertIssuer(cc)
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.True(t, issuer.mfa.FallbackActive())
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
		require.False(t, issuer.mfa.FallbackActive())
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
// clusters without per-session MFA issue with no ceremony, serially, saving their certs to the key store.
func TestKubeCertIssuer_MFAOffNoCeremony(t *testing.T) {
	t.Parallel()

	const numClusters = 3
	clusters := newTestKubeClusters(numClusters)
	keyRing := newTestKubeKeyRing(t, clusters)

	synctest.Test(t, func(t *testing.T) {
		cc := &fakeKubeCertClient{mfaRequired: false}
		cc.issueFn = func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
			if params.ReusableMFAResponse != nil {
				return nil, trace.BadParameter("no MFA ceremony expected for MFA-off clusters")
			}
			return &client.IssueUserCertsWithMFAResult{
				KeyRing:     keyRing,
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		issuer := newTestKubeCertIssuer(cc)

		start := time.Now()
		certs, err := issuer.issueCerts(t.Context(), clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.Equal(t, numClusters, cc.saves)
		// Key-store-writing issuances must not run concurrently.
		require.Equal(t, 3*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_DistinctTeleportClusters verifies a fan-out spanning Teleport clusters:
// per-cluster MFA checks, per-cluster cert routing, one ceremony replayed across all.
func TestKubeCertIssuer_DistinctTeleportClusters(t *testing.T) {
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
	return keyRing
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
}

func (f *fakeMFAAuthClient) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	if f.required {
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

	mu       sync.Mutex
	connects []string
	dials    int
	closes   int
	saves    int
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
	return &fakeMFAAuthClient{required: f.mfaRequired}, nil
}
