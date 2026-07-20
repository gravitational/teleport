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
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/utils/cert"
)

// TestKubeCertIssuer_SingleCeremony verifies the main behavior:
// an MFA-gated fan-out runs exactly one MFA ceremony, replays its reusable
// response across the other issuances, and issues them concurrently.
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
		certs, err := newTestKubeCertIssuer().issueCerts(t.Context(), cc, clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.Equal(t, int32(1), ceremonies.Load())
		require.Equal(t, int32(numClusters-1), replays.Load())
		// One serial ceremony plus one concurrent replay wave.
		require.Equal(t, 2*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_OldServerFallback verifies that when the auth server
// rejects reusable MFA for the MULTI requester (a server that predates it),
// the issuer falls back to per-cluster non-reusable ceremonies, serially, with no user-facing error.
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
		issuer := newTestKubeCertIssuer()
		certs, err := issuer.issueCerts(t.Context(), cc, clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.True(t, issuer.fallbackActive())
		require.Equal(t, int32(1), multiAttempts.Load(), "only the first issuance should try the MULTI requester")
		require.Equal(t, int32(numClusters), legacyCeremonies.Load())
		// One rejected MULTI attempt plus three legacy ceremonies, all serial:
		// per-cluster ceremonies prompt the user, so they must not run concurrently.
		require.Equal(t, 4*time.Second, time.Since(start))
	})
}

// TestKubeCertIssuer_MFAOffNoCeremony verifies that clusters without
// per-session MFA are issued with no ceremony, their certs are saved to the key store,
// and the issuances run serially: they write the key store, which does not tolerate concurrent use.
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

		var saves atomic.Int32
		issuer := newTestKubeCertIssuer()
		issuer.saveKeyRing = func(keyRing *client.KeyRing) error {
			saves.Add(1)
			return nil
		}

		start := time.Now()
		certs, err := issuer.issueCerts(t.Context(), cc, clusters)
		require.NoError(t, err)
		require.Len(t, certs, numClusters)
		require.Equal(t, int32(numClusters), saves.Load())
		// Serial issuance: key-store-writing issuances must not run concurrently.
		require.Equal(t, 3*time.Second, time.Since(start))
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

func newTestKubeCertIssuer() *kubeCertIssuer {
	return &kubeCertIssuer{
		tc:          &client.TeleportClient{},
		requester:   proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
		saveKeyRing: func(keyRing *client.KeyRing) error { return nil },
	}
}

type fakeMFAAuthClient struct {
	authclient.ClientI
	required bool
}

func (f *fakeMFAAuthClient) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	if f.required {
		return &proto.IsMFARequiredResponse{Required: true, MFARequired: proto.MFARequired_MFA_REQUIRED_YES}, nil
	}
	return &proto.IsMFARequiredResponse{Required: false, MFARequired: proto.MFARequired_MFA_REQUIRED_NO}, nil
}

func (f *fakeMFAAuthClient) Close() error { return nil }

type fakeKubeCertClient struct {
	mfaRequired bool
	issueFn     func(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error)
}

func (f *fakeKubeCertClient) IssueUserCertsWithMFA(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error) {
	// Every issuance takes one second of fake time
	// so tests can assert scheduling through elapsed time.
	time.Sleep(time.Second)
	return f.issueFn(ctx, params)
}

func (f *fakeKubeCertClient) ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	return &fakeMFAAuthClient{required: f.mfaRequired}, nil
}
