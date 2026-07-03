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

package alpnproxy

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const (
	testTeleportCluster = "local"
	testKubeCluster     = "kube-a"
	testChallengeName   = "challenge-abc"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// challengingUpstream returns 401 challenges until the request references
// testChallengeName, and records every request it sees.
type challengingUpstream struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
}

func (u *challengingUpstream) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}
	u.mu.Lock()
	u.requests = append(u.requests, req)
	u.bodies = append(u.bodies, body)
	u.mu.Unlock()

	if req.Header.Get(common.KubeInBandMFAChallengeResponseHeader) != testChallengeName {
		resp := newTestResponse(http.StatusUnauthorized, "Unauthorized")
		resp.Header.Set(common.KubeInBandMFAChallengeHeader, common.KubeInBandMFAChallengeValueRequired)
		return resp, nil
	}
	return newTestResponse(http.StatusOK, "ok"), nil
}

func (u *challengingUpstream) requestCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.requests)
}

func newTestResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestKubeMFAMiddleware(t *testing.T, ceremony KubeMFACeremony) (*KubeMiddleware, []byte) {
	t.Helper()

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	privKey, err := keys.NewPrivateKey(ecKey)
	require.NoError(t, err)
	cert, err := createLocalCA(privKey, time.Now().Add(time.Hour), testTeleportCluster)
	require.NoError(t, err)

	certs := make(KubeClientCerts)
	certs.Add(testTeleportCluster, testKubeCluster, cert)

	m := NewKubeMiddleware(KubeMiddlewareConfig{
		Certs:        certs,
		Logger:       logtest.NewLogger(),
		CloseContext: context.Background(),
		MFACeremony:  ceremony,
	}).(*KubeMiddleware)
	require.NoError(t, m.CheckAndSetDefaults())

	leaf, err := utils.TLSCertLeaf(cert)
	require.NoError(t, err)
	return m, common.KubeClientCertFingerprint(leaf)
}

func newKubeTestRequest(t *testing.T, method string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, "https://teleport.example.com"+common.KubeLocalProxyPathPrefix(testTeleportCluster, testKubeCluster)+"/api/v1/pods", body)
	require.NoError(t, err)
	// Server-side requests, which is what the local proxy forwards, have no GetBody.
	req.GetBody = nil
	return req
}

func TestKubeMFARoundTripper(t *testing.T) {
	t.Parallel()

	t.Run("wrap transport without ceremony is a no-op", func(t *testing.T) {
		m, _ := newTestKubeMFAMiddleware(t, nil)
		inner := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })
		require.IsType(t, inner, m.WrapTransport(inner))
	})

	t.Run("capability header is set and responses pass through", func(t *testing.T) {
		var ceremonies atomic.Int32
		m, _ := newTestKubeMFAMiddleware(t, func(context.Context, string, []byte) (string, error) {
			ceremonies.Add(1)
			return testChallengeName, nil
		})
		var sawCapability string
		inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sawCapability = req.Header.Get(common.KubeInBandMFACapabilityHeader)
			return newTestResponse(http.StatusOK, "ok"), nil
		})

		resp, err := m.WrapTransport(inner).RoundTrip(newKubeTestRequest(t, http.MethodGet, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, common.KubeInBandMFACapabilityMFAv2, sawCapability)
		require.Zero(t, ceremonies.Load())
	})

	t.Run("challenge runs ceremony and replays the request", func(t *testing.T) {
		var ceremonies atomic.Int32
		var gotFingerprint []byte
		var gotCluster string
		m, wantFingerprint := newTestKubeMFAMiddleware(t, func(_ context.Context, cluster string, fp []byte) (string, error) {
			ceremonies.Add(1)
			gotCluster = cluster
			gotFingerprint = fp
			return testChallengeName, nil
		})
		upstream := &challengingUpstream{}
		rt := m.WrapTransport(upstream)

		const bodyContent = "apply-manifest-body"
		resp, err := rt.RoundTrip(newKubeTestRequest(t, http.MethodPost, strings.NewReader(bodyContent)))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.Equal(t, int32(1), ceremonies.Load())
		require.Equal(t, wantFingerprint, gotFingerprint)
		require.Equal(t, testTeleportCluster, gotCluster)
		require.Equal(t, 2, upstream.requestCount())
		require.Equal(t, []byte(bodyContent), upstream.bodies[0], "original request body")
		require.Equal(t, []byte(bodyContent), upstream.bodies[1], "replayed request body must match")

		// A later request reuses the validated challenge without another ceremony.
		resp, err = rt.RoundTrip(newKubeTestRequest(t, http.MethodGet, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, 3, upstream.requestCount())
		require.Equal(t, testChallengeName, upstream.requests[2].Header.Get(common.KubeInBandMFAChallengeResponseHeader))
		require.Equal(t, int32(1), ceremonies.Load())
	})

	t.Run("non-challenge 401 passes through untouched", func(t *testing.T) {
		var ceremonies atomic.Int32
		m, _ := newTestKubeMFAMiddleware(t, func(context.Context, string, []byte) (string, error) {
			ceremonies.Add(1)
			return testChallengeName, nil
		})
		inner := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return newTestResponse(http.StatusUnauthorized, "Unauthorized"), nil
		})

		resp, err := m.WrapTransport(inner).RoundTrip(newKubeTestRequest(t, http.MethodGet, nil))
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Zero(t, ceremonies.Load())
	})

	t.Run("concurrent challenges share one ceremony", func(t *testing.T) {
		var ceremonies atomic.Int32
		m, _ := newTestKubeMFAMiddleware(t, func(context.Context, string, []byte) (string, error) {
			ceremonies.Add(1)
			time.Sleep(50 * time.Millisecond)
			return testChallengeName, nil
		})
		upstream := &challengingUpstream{}
		rt := m.WrapTransport(upstream)

		var wg sync.WaitGroup
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := rt.RoundTrip(newKubeTestRequest(t, http.MethodGet, nil))
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)
			}()
		}
		wg.Wait()
		require.Equal(t, int32(1), ceremonies.Load())
	})

	t.Run("oversized body is not replayed", func(t *testing.T) {
		var ceremonies atomic.Int32
		m, _ := newTestKubeMFAMiddleware(t, func(context.Context, string, []byte) (string, error) {
			ceremonies.Add(1)
			return testChallengeName, nil
		})
		upstream := &challengingUpstream{}
		rt := m.WrapTransport(upstream)

		oversized := bytes.Repeat([]byte("x"), kubeMFAReplayBodyLimit+1)
		_, err := rt.RoundTrip(newKubeTestRequest(t, http.MethodPost, bytes.NewReader(oversized)))
		require.Error(t, err)
		require.ErrorContains(t, err, "replay buffer")
		require.Zero(t, ceremonies.Load())
		require.Equal(t, 1, upstream.requestCount())
		require.Len(t, upstream.bodies[0], len(oversized), "oversized body must still stream through in full")
	})

	t.Run("shared cert serves every kube cluster", func(t *testing.T) {
		m, wantFingerprint := newTestKubeMFAMiddleware(t, nil)
		// Re-key the cert under the shared (teleport cluster, "") entry.
		cert, err := m.getCert(testTeleportCluster, testKubeCluster)
		require.NoError(t, err)
		m.ClearCerts()
		m.certs.Add(testTeleportCluster, "", cert)
		m.sharedCerts = true

		for _, kubeCluster := range []string{testKubeCluster, "kube-b"} {
			req, err := http.NewRequest(http.MethodGet, "https://teleport.example.com"+common.KubeLocalProxyPathPrefix(testTeleportCluster, kubeCluster)+"/api/v1/pods", nil)
			require.NoError(t, err)
			certs, ok, err := m.GetClientCerts(req)
			require.NoError(t, err)
			require.True(t, ok)
			require.Len(t, certs, 1)
			fp, err := m.requestCertFingerprint(req)
			require.NoError(t, err)
			require.Equal(t, wantFingerprint, fp, "every cluster must be served by the same shared cert")
		}
	})

	t.Run("shared cert reissue is stored under the shared key", func(t *testing.T) {
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		privKey, err := keys.NewPrivateKey(ecKey)
		require.NoError(t, err)
		expiredCert, err := createLocalCA(privKey, time.Now().Add(-time.Minute), testTeleportCluster)
		require.NoError(t, err)
		freshCert, err := createLocalCA(privKey, time.Now().Add(time.Hour), testTeleportCluster)
		require.NoError(t, err)

		certs := make(KubeClientCerts)
		certs.Add(testTeleportCluster, "", expiredCert)
		var reissuedFor []string
		m := NewKubeMiddleware(KubeMiddlewareConfig{
			Certs: certs,
			CertReissuer: func(_ context.Context, teleportCluster, kubeCluster string) (tls.Certificate, error) {
				reissuedFor = append(reissuedFor, teleportCluster+"/"+kubeCluster)
				return freshCert, nil
			},
			Logger:       logtest.NewLogger(),
			CloseContext: context.Background(),
			SharedCerts:  true,
		}).(*KubeMiddleware)
		require.NoError(t, m.CheckAndSetDefaults())

		req, err := http.NewRequest(http.MethodGet, "https://teleport.example.com"+common.KubeLocalProxyPathPrefix(testTeleportCluster, testKubeCluster)+"/api/v1/pods", nil)
		require.NoError(t, err)
		handled := m.HandleRequest(httptest.NewRecorder(), req)
		require.False(t, handled, "request must proceed after reissue")
		require.Equal(t, []string{testTeleportCluster + "/"}, reissuedFor, "reissuer must be called for the shared unrouted cert")

		got, err := m.getCert(testTeleportCluster, testKubeCluster)
		require.NoError(t, err)
		require.Equal(t, freshCert.Certificate, got.Certificate, "fresh shared cert must serve all clusters")
	})

	t.Run("ceremony failure surfaces as an error", func(t *testing.T) {
		m, _ := newTestKubeMFAMiddleware(t, func(context.Context, string, []byte) (string, error) {
			return "", trace.AccessDenied("user canceled MFA")
		})
		upstream := &challengingUpstream{}

		_, err := m.WrapTransport(upstream).RoundTrip(newKubeTestRequest(t, http.MethodGet, nil))
		require.Error(t, err)
		require.ErrorContains(t, err, "user canceled MFA")
	})
}
