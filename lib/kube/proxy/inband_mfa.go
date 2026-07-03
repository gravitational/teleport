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
	"cmp"
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidatedMFAChallengeVerifier verifies that a validated MFA challenge resource exists in
// order to determine if the user has completed an in-band MFA ceremony for the session.
type ValidatedMFAChallengeVerifier interface {
	VerifyValidatedMFAChallenge(ctx context.Context, req *mfav2.VerifyValidatedMFAChallengeRequest, opts ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error)
}

// errInBandMFAChallengeRequired signals that access would be granted once the client
// completes an in-band MFA ceremony. It is translated into a 401 response carrying
// the challenge header, which the kube local proxy intercepts and satisfies.
var errInBandMFAChallengeRequired = &trace.AccessDeniedError{
	Message: "per-session MFA is required: complete an in-band MFA ceremony and retry",
}

// inBandMFAParams holds the in-band per-session MFA parameters extracted from a request.
type inBandMFAParams struct {
	// capable is true when the client signaled in-band MFA capability and a session
	// fingerprint could be resolved for the request.
	capable bool
	// sessionFingerprint is the session identifying payload binding MFA challenges to the
	// client's local proxy session.
	sessionFingerprint []byte
	// challengeName is the validated challenge referenced by the client, if any.
	challengeName string
}

// inBandMFAParamsFromRequest resolves the in-band MFA parameters for a request and
// sanitizes the session fingerprint header.
//
// At the hop that terminates the user's mTLS connection the fingerprint is derived from
// the peer certificate and overwrites any client-supplied header value, which both
// sanitizes the header and forwards the derived value to the next hop alongside the
// identity forwarding headers. At a downstream hop the peer certificate belongs to the
// proxy that forwarded the identity, so the fingerprint is read from the header instead.
//
// Any resolution failure returns not-capable, which preserves the legacy
// per-session-MFA-certificate enforcement (fail closed).
func (f *Forwarder) inBandMFAParamsFromRequest(ctx context.Context, req *http.Request, identity *tlsca.Identity) inBandMFAParams {
	peerCert, err := authz.UserCertificateFromContext(ctx)
	if err != nil {
		return inBandMFAParams{}
	}
	peerIdentity, err := tlsca.FromSubject(peerCert.Subject, peerCert.NotAfter)
	if err != nil {
		return inBandMFAParams{}
	}

	// When the peer certificate belongs to the authenticated user this hop terminates the
	// user's mTLS connection; otherwise the identity was forwarded by a proxy and the peer
	// certificate is the proxy's.
	peerIsUser := peerIdentity.Username == identity.Username
	if peerIsUser {
		// The fingerprint header is only trusted across the proxy -> kube service hop; a
		// user-facing hop always discards whatever the client sent.
		req.Header.Del(common.KubeInBandMFASessionFingerprintHeader)
	}

	if req.Header.Get(common.KubeInBandMFACapabilityHeader) != common.KubeInBandMFACapabilityMFAv2 {
		return inBandMFAParams{}
	}

	var fingerprint []byte
	if peerIsUser {
		fingerprint = common.KubeClientCertFingerprint(peerCert)
		req.Header.Set(common.KubeInBandMFASessionFingerprintHeader, base64.RawURLEncoding.EncodeToString(fingerprint))
	} else {
		fingerprint, err = base64.RawURLEncoding.DecodeString(req.Header.Get(common.KubeInBandMFASessionFingerprintHeader))
		if err != nil || len(fingerprint) == 0 {
			// An older proxy between the client and this service does not forward the
			// fingerprint; fall back to legacy enforcement rather than challenge in a loop
			// the client can never satisfy.
			return inBandMFAParams{}
		}
	}

	return inBandMFAParams{
		capable:            true,
		sessionFingerprint: fingerprint,
		challengeName:      req.Header.Get(common.KubeInBandMFAChallengeResponseHeader),
	}
}

// inBandMFACacheTTL bounds how long a successful backend verification is reused before
// re-verifying. The authoritative lifetime is the ValidatedMFAChallenge resource TTL:
// once the resource expires re-verification fails and the client is re-challenged.
const inBandMFACacheTTL = time.Minute

// inBandMFACacheMaxSize bounds the verification cache. When full, verification still
// works but results are not cached.
const inBandMFACacheMaxSize = 10000

// inBandMFACacheKey is the full verification tuple. Caching is keyed on every field sent
// to VerifyValidatedMFAChallenge so a cache hit proves exactly what a backend
// verification would.
type inBandMFACacheKey struct {
	challengeName      string
	sessionFingerprint string
	username           string
	sourceCluster      string
}

// satisfyInBandMFA reports whether the validated challenge referenced by the request
// proves per-session MFA for the request's session. Successful backend verifications are
// cached for inBandMFACacheTTL.
func (f *Forwarder) satisfyInBandMFA(ctx context.Context, actx *authContext) bool {
	p := actx.inBandMFA
	if !p.capable || p.challengeName == "" {
		return false
	}
	// Resolved lazily rather than in CheckAndSetDefaults so that partially-mocked auth
	// clients in tests never have their MFA service client touched.
	verifier := f.cfg.ValidatedMFAChallengeVerifier
	if verifier == nil {
		if f.cfg.AuthClient == nil {
			return false
		}
		verifier = f.cfg.AuthClient.MFAServiceClientV2()
	}

	identity := actx.Identity.GetIdentity()
	key := inBandMFACacheKey{
		challengeName:      p.challengeName,
		sessionFingerprint: string(p.sessionFingerprint),
		username:           identity.Username,
		sourceCluster:      cmp.Or(identity.RouteToCluster, identity.TeleportCluster),
	}

	f.inBandMFAMu.RLock()
	validUntil, ok := f.inBandMFAVerified[key]
	f.inBandMFAMu.RUnlock()
	if ok && f.cfg.Clock.Now().Before(validUntil) {
		return true
	}

	req := mfav2.VerifyValidatedMFAChallengeRequest_builder{
		Name: key.challengeName,
		Payload: mfav2.SessionIdentifyingPayload_builder{
			KubeClientCertFingerprint: p.sessionFingerprint,
		}.Build(),
		SourceCluster: key.sourceCluster,
		Username:      key.username,
	}.Build()
	if _, err := verifier.VerifyValidatedMFAChallenge(ctx, req); err != nil {
		f.log.DebugContext(ctx, "In-band MFA challenge verification failed",
			"error", err,
			"challenge", key.challengeName,
			"user", key.username,
		)
		f.inBandMFAMu.Lock()
		delete(f.inBandMFAVerified, key)
		f.inBandMFAMu.Unlock()
		return false
	}

	f.inBandMFAMu.Lock()
	defer f.inBandMFAMu.Unlock()
	if len(f.inBandMFAVerified) >= inBandMFACacheMaxSize {
		now := f.cfg.Clock.Now()
		for k, until := range f.inBandMFAVerified {
			if now.After(until) {
				delete(f.inBandMFAVerified, k)
			}
		}
	}
	if len(f.inBandMFAVerified) < inBandMFACacheMaxSize {
		f.inBandMFAVerified[key] = f.cfg.Clock.Now().Add(inBandMFACacheTTL)
	}
	return true
}

// accessWouldBeGrantedWithMFA reports whether a denied access would be granted if the
// session were MFA-verified, i.e. whether an in-band MFA ceremony can cure the denial.
// It re-runs the access evaluation with MFAVerified forced on, mirroring the semantics
// of the auth server's IsMFARequired check. This is a plain re-evaluation for now; the
// structured alternative is CheckConditionalAccess preconditions (the SSH pattern),
// which a future decision-service permit can feed without changing callers.
func (f *Forwarder) accessWouldBeGrantedWithMFA(ctx context.Context, actx *authContext, matchers ...services.RoleMatcher) bool {
	saved := actx.accessState
	actx.accessState.MFAVerified = true
	_, err := f.getKubeAccessDetails(ctx, actx, matchers...)
	actx.accessState = saved
	return err == nil
}

// writeInBandMFAChallenge writes the distinguishable 401 challenge response consumed by
// the kube local proxy. kubectl never sees it: the local proxy intercepts the challenge,
// runs the MFA ceremony, and retries the request.
func (f *Forwarder) writeInBandMFAChallenge(rw http.ResponseWriter) {
	rw.Header().Set(common.KubeInBandMFAChallengeHeader, common.KubeInBandMFAChallengeValueRequired)
	status := &kubeerrors.NewUnauthorized("Teleport per-session MFA is required: complete an in-band MFA ceremony and retry").ErrStatus
	data, err := runtime.Encode(globalKubeCodecs.LegacyCodec(), status)
	if err != nil {
		f.log.WarnContext(f.ctx, "Failed encoding error into kube Status object", "error", err)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	rw.Header().Set(responsewriters.ContentTypeHeader, "application/json")
	rw.WriteHeader(http.StatusUnauthorized)
	if _, err := rw.Write(data); err != nil && !utils.IsOKNetworkError(err) {
		f.log.WarnContext(f.ctx, "Failed writing kube error response body", "error", err)
	}
}
