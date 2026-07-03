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
	"crypto/sha256"
	"crypto/x509"
)

// Headers negotiating in-band per-session MFA between the tsh kube local proxy
// and the Teleport kube forwarder (RFD 0234 extended to Kubernetes access).
const (
	// KubeInBandMFACapabilityHeader is sent by the kube local proxy on every
	// request to signal that it can run an in-band mfav2 MFA ceremony. Without
	// it the forwarder enforces the legacy per-session-MFA-certificate flow.
	KubeInBandMFACapabilityHeader = "Teleport-Kube-Mfa-Capability"
	// KubeInBandMFACapabilityMFAv2 is the capability header value for clients
	// speaking the mfav2 session challenge protocol.
	KubeInBandMFACapabilityMFAv2 = "mfav2"

	// KubeInBandMFAChallengeHeader is set by the kube forwarder on a 401
	// response to signal that the request requires an in-band MFA ceremony.
	// The kube local proxy intercepts it, runs the ceremony, and retries;
	// kubectl never sees the challenge.
	KubeInBandMFAChallengeHeader = "Teleport-Kube-Mfa-Challenge"
	// KubeInBandMFAChallengeValueRequired is the challenge header value.
	KubeInBandMFAChallengeValueRequired = "required"

	// KubeInBandMFAChallengeResponseHeader carries the name of a validated
	// mfav2 challenge. The local proxy sets it on every request after
	// completing a ceremony; the forwarder verifies the named challenge
	// against the backend, bound to the session fingerprint.
	KubeInBandMFAChallengeResponseHeader = "Teleport-Kube-Mfa-Challenge-Response"

	// KubeInBandMFASessionFingerprintHeader carries the base64url-encoded
	// session fingerprint from the Teleport proxy to a downstream kube
	// service, which cannot derive it because the proxy terminates the user's
	// mTLS connection. It rides the same trust plane as the identity
	// forwarding headers: the user-facing hop always overwrites it with the
	// value derived from the peer certificate, and downstream hops only trust
	// it when the identity itself was forwarded by a proxy.
	KubeInBandMFASessionFingerprintHeader = "Teleport-Kube-Mfa-Session-Fingerprint"
)

// KubeClientCertFingerprint computes the session identifying payload for
// in-band kube MFA: the SHA-256 digest of the DER-encoded client certificate
// the kube local proxy presents to the Teleport proxy. The local proxy
// computes it from its own certificate and the kube forwarder computes it
// from the mTLS peer certificate, so both sides derive the same value
// independently.
func KubeClientCertFingerprint(cert *x509.Certificate) []byte {
	sum := sha256.Sum256(cert.Raw)
	return sum[:]
}
