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

package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// genTestX509Cert generates a certificate for the given identity signed by the
// test fixtures CA.
func genTestX509Cert(t *testing.T, identity tlsca.Identity) *x509.Certificate {
	t.Helper()

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	subj, err := identity.Subject()
	require.NoError(t, err)
	certPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clockwork.NewRealClock(),
		PublicKey: priv.Public(),
		Subject:   subj,
		NotAfter:  time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	cert, err := tlsca.ParseCertificatePEM(certPEM)
	require.NoError(t, err)
	return cert
}

type fakeChallengeVerifier struct {
	mu      sync.Mutex
	calls   int
	lastReq *mfav2.VerifyValidatedMFAChallengeRequest
	err     error
}

func (f *fakeChallengeVerifier) VerifyValidatedMFAChallenge(ctx context.Context, req *mfav2.VerifyValidatedMFAChallengeRequest, opts ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &mfav2.VerifyValidatedMFAChallengeResponse{}, nil
}

func (f *fakeChallengeVerifier) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestInBandMFAParamsFromRequest(t *testing.T) {
	t.Parallel()

	f := &Forwarder{log: logtest.NewLogger()}
	userIdentity := tlsca.Identity{Username: "user-a", Groups: []string{"access"}, TeleportCluster: "local"}
	userCert := genTestX509Cert(t, userIdentity)
	proxyCert := genTestX509Cert(t, tlsca.Identity{Username: "host.local", Groups: []string{string(types.RoleProxy)}, TeleportCluster: "local"})
	sip := common.KubeClientCertFingerprint(userCert)
	encodedSIP := base64.RawURLEncoding.EncodeToString(sip)

	newReq := func(headers map[string]string) *http.Request {
		req := &http.Request{URL: &url.URL{}, Header: http.Header{}}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return req
	}
	ctxWithPeer := func(cert *x509.Certificate) context.Context {
		return authz.ContextWithUserCertificate(context.Background(), cert)
	}

	t.Run("no peer certificate is not capable", func(t *testing.T) {
		req := newReq(map[string]string{common.KubeInBandMFACapabilityHeader: common.KubeInBandMFACapabilityMFAv2})
		params := f.inBandMFAParamsFromRequest(context.Background(), req, &userIdentity)
		require.False(t, params.capable)
	})

	t.Run("user-facing hop derives fingerprint and overwrites header", func(t *testing.T) {
		req := newReq(map[string]string{
			common.KubeInBandMFACapabilityHeader:         common.KubeInBandMFACapabilityMFAv2,
			common.KubeInBandMFASessionFingerprintHeader: "forged-by-client",
			common.KubeInBandMFAChallengeResponseHeader:  "challenge-1",
		})
		params := f.inBandMFAParamsFromRequest(ctxWithPeer(userCert), req, &userIdentity)
		require.True(t, params.capable)
		require.Equal(t, sip, params.sessionFingerprint)
		require.Equal(t, "challenge-1", params.challengeName)
		require.Equal(t, encodedSIP, req.Header.Get(common.KubeInBandMFASessionFingerprintHeader))
	})

	t.Run("user-facing hop without capability deletes forged header", func(t *testing.T) {
		req := newReq(map[string]string{
			common.KubeInBandMFASessionFingerprintHeader: "forged-by-client",
		})
		params := f.inBandMFAParamsFromRequest(ctxWithPeer(userCert), req, &userIdentity)
		require.False(t, params.capable)
		require.Empty(t, req.Header.Get(common.KubeInBandMFASessionFingerprintHeader))
	})

	t.Run("forwarded hop reads fingerprint from header", func(t *testing.T) {
		req := newReq(map[string]string{
			common.KubeInBandMFACapabilityHeader:         common.KubeInBandMFACapabilityMFAv2,
			common.KubeInBandMFASessionFingerprintHeader: encodedSIP,
		})
		params := f.inBandMFAParamsFromRequest(ctxWithPeer(proxyCert), req, &userIdentity)
		require.True(t, params.capable)
		require.Equal(t, sip, params.sessionFingerprint)
	})

	t.Run("forwarded hop without fingerprint falls back to legacy", func(t *testing.T) {
		req := newReq(map[string]string{common.KubeInBandMFACapabilityHeader: common.KubeInBandMFACapabilityMFAv2})
		params := f.inBandMFAParamsFromRequest(ctxWithPeer(proxyCert), req, &userIdentity)
		require.False(t, params.capable)
	})

	t.Run("forwarded hop with invalid fingerprint falls back to legacy", func(t *testing.T) {
		req := newReq(map[string]string{
			common.KubeInBandMFACapabilityHeader:         common.KubeInBandMFACapabilityMFAv2,
			common.KubeInBandMFASessionFingerprintHeader: "!!!not-base64url!!!",
		})
		params := f.inBandMFAParamsFromRequest(ctxWithPeer(proxyCert), req, &userIdentity)
		require.False(t, params.capable)
	})
}

func newInBandTestAuthContext(t *testing.T, identity tlsca.Identity, params inBandMFAParams) *authContext {
	t.Helper()
	user, err := types.NewUser(identity.Username)
	require.NoError(t, err)
	authCtx := authz.Context{
		User:             user,
		Identity:         authz.WrapIdentity(identity),
		UnmappedIdentity: authz.WrapIdentity(identity),
	}
	return &authContext{
		ScopedContext: authz.ScopedContextFromUnscopedContext(&authCtx),
		inBandMFA:     params,
	}
}

func TestSatisfyInBandMFA(t *testing.T) {
	t.Parallel()

	sip := []byte("test-session-fingerprint")
	identity := tlsca.Identity{Username: "user-a", TeleportCluster: "local"}

	newTestForwarder := func(verifier *fakeChallengeVerifier, clock clockwork.Clock) *Forwarder {
		return &Forwarder{
			log: logtest.NewLogger(),
			cfg: ForwarderConfig{
				Clock:                         clock,
				ValidatedMFAChallengeVerifier: verifier,
			},
			inBandMFAVerified: make(map[inBandMFACacheKey]time.Time),
		}
	}
	capableParams := inBandMFAParams{capable: true, sessionFingerprint: sip, challengeName: "challenge-1"}

	t.Run("not capable is not satisfied", func(t *testing.T) {
		verifier := &fakeChallengeVerifier{}
		f := newTestForwarder(verifier, clockwork.NewFakeClock())
		actx := newInBandTestAuthContext(t, identity, inBandMFAParams{})
		require.False(t, f.satisfyInBandMFA(context.Background(), actx))
		require.Zero(t, verifier.callCount())
	})

	t.Run("no challenge reference is not satisfied", func(t *testing.T) {
		verifier := &fakeChallengeVerifier{}
		f := newTestForwarder(verifier, clockwork.NewFakeClock())
		actx := newInBandTestAuthContext(t, identity, inBandMFAParams{capable: true, sessionFingerprint: sip})
		require.False(t, f.satisfyInBandMFA(context.Background(), actx))
		require.Zero(t, verifier.callCount())
	})

	t.Run("verification failure is not satisfied and not cached", func(t *testing.T) {
		verifier := &fakeChallengeVerifier{err: trace.NotFound("no validated challenge")}
		f := newTestForwarder(verifier, clockwork.NewFakeClock())
		actx := newInBandTestAuthContext(t, identity, capableParams)
		require.False(t, f.satisfyInBandMFA(context.Background(), actx))
		require.False(t, f.satisfyInBandMFA(context.Background(), actx))
		require.Equal(t, 2, verifier.callCount())
	})

	t.Run("verification success is cached until TTL", func(t *testing.T) {
		verifier := &fakeChallengeVerifier{}
		clock := clockwork.NewFakeClock()
		f := newTestForwarder(verifier, clock)
		actx := newInBandTestAuthContext(t, identity, capableParams)

		require.True(t, f.satisfyInBandMFA(context.Background(), actx))
		require.True(t, f.satisfyInBandMFA(context.Background(), actx))
		require.Equal(t, 1, verifier.callCount(), "second call must be served from cache")

		clock.Advance(inBandMFACacheTTL + time.Second)
		require.True(t, f.satisfyInBandMFA(context.Background(), actx))
		require.Equal(t, 2, verifier.callCount(), "expired cache entry must be re-verified")
	})

	t.Run("verification request carries the full binding tuple", func(t *testing.T) {
		verifier := &fakeChallengeVerifier{}
		f := newTestForwarder(verifier, clockwork.NewFakeClock())
		routed := identity
		routed.RouteToCluster = "root"
		actx := newInBandTestAuthContext(t, routed, capableParams)

		require.True(t, f.satisfyInBandMFA(context.Background(), actx))
		req := verifier.lastReq
		require.Equal(t, "challenge-1", req.GetName())
		require.Equal(t, sip, req.GetPayload().GetKubeClientCertFingerprint())
		require.Equal(t, "user-a", req.GetUsername())
		require.Equal(t, "root", req.GetSourceCluster(), "RouteToCluster must take precedence as the source cluster")
	})
}

func TestAuthorizeInBandMFA(t *testing.T) {
	t.Parallel()

	const username = "user-a"
	const clusterName = "local"

	nc, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout: types.NewDuration(time.Hour),
	})
	require.NoError(t, err)
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{})
	require.NoError(t, err)

	userIdentity := tlsca.Identity{Username: username, Groups: []string{"access"}, TeleportCluster: clusterName}
	userCert := genTestX509Cert(t, userIdentity)
	sip := common.KubeClientCertFingerprint(userCert)
	encodedSIP := base64.RawURLEncoding.EncodeToString(sip)

	kubeServers := newKubeServersFromKubeClusters(t, &types.KubernetesClusterV3{
		Metadata: types.Metadata{Name: clusterName},
	})

	tests := []struct {
		desc            string
		requireMFA      bool
		mfaStampedCert  bool
		headers         map[string]string
		verifierErr     error
		wantChallenge   bool
		wantDenied      bool
		wantInBandMFAOK bool
	}{
		{
			desc:       "capable client without challenge is challenged",
			requireMFA: true,
			headers: map[string]string{
				common.KubeInBandMFACapabilityHeader: common.KubeInBandMFACapabilityMFAv2,
			},
			wantChallenge: true,
		},
		{
			desc:       "capable client with verified challenge is granted",
			requireMFA: true,
			headers: map[string]string{
				common.KubeInBandMFACapabilityHeader:        common.KubeInBandMFACapabilityMFAv2,
				common.KubeInBandMFAChallengeResponseHeader: "challenge-1",
			},
			wantInBandMFAOK: true,
		},
		{
			desc:       "capable client with failing verification is challenged",
			requireMFA: true,
			headers: map[string]string{
				common.KubeInBandMFACapabilityHeader:        common.KubeInBandMFACapabilityMFAv2,
				common.KubeInBandMFAChallengeResponseHeader: "challenge-1",
			},
			verifierErr:   trace.NotFound("no validated challenge"),
			wantChallenge: true,
		},
		{
			desc:       "legacy client without MFA cert is denied, not challenged",
			requireMFA: true,
			wantDenied: true,
		},
		{
			desc:           "legacy client with MFA-stamped cert is granted",
			requireMFA:     true,
			mfaStampedCert: true,
		},
		{
			desc: "capable client against non-MFA cluster is granted without challenge",
			headers: map[string]string{
				common.KubeInBandMFACapabilityHeader: common.KubeInBandMFACapabilityMFAv2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			roleSpec := types.RoleSpecV6{
				Allow: types.RoleConditions{
					KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					KubeGroups:       []string{"kube-group-a"},
				},
			}
			if tt.requireMFA {
				roleSpec.Options.RequireMFAType = types.RequireMFAType_SESSION
			}
			roles, err := services.RoleSetFromSpec("kube-access", roleSpec)
			require.NoError(t, err)

			identity := tlsca.Identity{
				Username:          username,
				Groups:            roles.RoleNames(),
				RouteToCluster:    clusterName,
				KubernetesCluster: clusterName,
				Expires:           time.Now().Add(time.Hour),
			}
			if tt.mfaStampedCert {
				identity.MFAVerified = "mfa-device-id"
			}

			user, err := types.NewUser(username)
			require.NoError(t, err)
			authCtx := authz.Context{
				User: user,
				Checker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
					Roles: roles.RoleNames(),
				}, clusterName, roles),
				Identity:         authz.WrapIdentity(identity),
				UnmappedIdentity: authz.WrapIdentity(identity),
			}

			verifier := &fakeChallengeVerifier{err: tt.verifierErr}
			f := &Forwarder{
				log: logtest.NewLogger(),
				cfg: ForwarderConfig{
					ClusterName: clusterName,
					CachingAuthClient: &mockAccessPoint{
						netConfig:       nc,
						recordingConfig: types.DefaultSessionRecordingConfig(),
						authPref:        authPref,
						kubeServers:     kubeServers,
					},
					TracerProvider:                otel.GetTracerProvider(),
					tracer:                        otel.Tracer(teleport.ComponentKube),
					ClusterFeatures:               fakeClusterFeatures,
					KubeServiceType:               ProxyService,
					Clock:                         clockwork.NewRealClock(),
					ValidatedMFAChallengeVerifier: verifier,
					ScopedAuthz:                   mockAuthorizer{scopedCtx: authz.ScopedContextFromUnscopedContext(&authCtx)},
				},
				inBandMFAVerified: make(map[inBandMFACacheKey]time.Time),
				clusterDetails: map[string]*kubeDetails{
					clusterName: {kubeCreds: &staticKubeCreds{targetAddr: "k8s.example.com"}},
				},
				getKubernetesServersForKubeCluster: func(ctx context.Context, name string) ([]types.KubeServer, error) {
					return kubeServers, nil
				},
			}

			req := &http.Request{
				Host:       "example.com",
				RemoteAddr: "user.example.com",
				URL:        &url.URL{Path: "/api/v1/pods"},
				Header:     http.Header{},
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{userCert},
				},
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			ctx := authz.ContextWithUser(context.Background(), authz.LocalUser{Username: username, Identity: identity})
			ctx = authz.ContextWithUserCertificate(ctx, userCert)
			req = req.WithContext(ctx)

			actx, err := f.authenticate(req)
			require.NoError(t, err)

			err = f.authorize(context.Background(), actx)
			switch {
			case tt.wantChallenge:
				require.ErrorIs(t, err, errInBandMFAChallengeRequired)
			case tt.wantDenied:
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
				require.NotErrorIs(t, err, errInBandMFAChallengeRequired)
			default:
				require.NoError(t, err)
				require.Equal(t, tt.wantInBandMFAOK, actx.inBandMFAVerified)
				if tt.wantInBandMFAOK {
					require.True(t, actx.accessState.MFAVerified)
				}
			}

			if _, capable := tt.headers[common.KubeInBandMFACapabilityHeader]; capable {
				require.Equal(t, encodedSIP, req.Header.Get(common.KubeInBandMFASessionFingerprintHeader),
					"user-facing hop must stamp the derived fingerprint for the downstream hop")
			}
		})
	}
}

func TestCopyInBandMFAHeaders(t *testing.T) {
	t.Parallel()

	src := http.Header{}
	src.Set(common.KubeInBandMFACapabilityHeader, common.KubeInBandMFACapabilityMFAv2)
	src.Set(common.KubeInBandMFASessionFingerprintHeader, "fingerprint")
	src.Set(common.KubeInBandMFAChallengeResponseHeader, "challenge-1")

	dst := http.Header{}
	dst.Set(common.KubeInBandMFASessionFingerprintHeader, "stale-value")
	copyInBandMFAHeaders(dst, src)
	require.Equal(t, common.KubeInBandMFACapabilityMFAv2, dst.Get(common.KubeInBandMFACapabilityHeader))
	require.Equal(t, "fingerprint", dst.Get(common.KubeInBandMFASessionFingerprintHeader))
	require.Equal(t, "challenge-1", dst.Get(common.KubeInBandMFAChallengeResponseHeader))

	// Headers absent from the source are removed from the destination, not left stale.
	dst2 := http.Header{}
	dst2.Set(common.KubeInBandMFACapabilityHeader, common.KubeInBandMFACapabilityMFAv2)
	copyInBandMFAHeaders(dst2, http.Header{})
	require.Empty(t, dst2.Get(common.KubeInBandMFACapabilityHeader))
}

func TestFormatStatusResponseErrorInBandChallenge(t *testing.T) {
	t.Parallel()

	f := &Forwarder{log: logtest.NewLogger()}
	rec := httptest.NewRecorder()
	f.formatStatusResponseError(rec, trace.Wrap(errInBandMFAChallengeRequired))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, common.KubeInBandMFAChallengeValueRequired, rec.Header().Get(common.KubeInBandMFAChallengeHeader))
	require.Contains(t, rec.Body.String(), "in-band MFA ceremony")
}
