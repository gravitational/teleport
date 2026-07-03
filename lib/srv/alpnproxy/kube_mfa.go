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
	"io"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeMFACeremony performs an in-band session MFA ceremony bound to the fingerprint of the
// local proxy's client certificate and returns the validated challenge name. The
// teleportCluster is the Teleport cluster the challenged request was routed to.
type KubeMFACeremony func(ctx context.Context, teleportCluster string, certFingerprint []byte) (string, error)

// kubeMFAReplayBodyLimit bounds how much of a request body is buffered so the request can
// be replayed after an in-band MFA challenge. Larger request bodies are streamed without
// buffering and cannot be replayed; the Kubernetes API server itself rejects objects far
// smaller than this limit.
const kubeMFAReplayBodyLimit = 8 << 20 // 8 MiB

// WrapTransport implements [LocalProxyHTTPMiddleware]. When an MFA ceremony is configured,
// it intercepts in-band MFA challenges from the kube forwarder, runs the ceremony, and
// replays the challenged request; kubectl never sees the challenge.
func (m *KubeMiddleware) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	if m.mfaCeremony == nil {
		return rt
	}
	return &kubeMFARoundTripper{m: m, inner: rt}
}

// kubeMFARoundTripper signals in-band MFA capability on every outbound request and
// satisfies challenge responses from the kube forwarder.
type kubeMFARoundTripper struct {
	m     *KubeMiddleware
	inner http.RoundTripper
}

func (r *kubeMFARoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fingerprint, err := r.m.requestCertFingerprint(req)
	if err != nil {
		// Without the client certificate the session fingerprint cannot be derived, so
		// in-band MFA is not offered and the request keeps legacy behavior.
		return r.inner.RoundTrip(req)
	}

	req = req.Clone(req.Context())
	req.Header.Set(common.KubeInBandMFACapabilityHeader, common.KubeInBandMFACapabilityMFAv2)
	sipKey := string(fingerprint)
	if name := r.m.getChallengeName(sipKey); name != "" {
		req.Header.Set(common.KubeInBandMFAChallengeResponseHeader, name)
	}

	// Make the body replayable within a bound so a challenged request can be retried.
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	if !replayable {
		data, rest, buffered, err := bufferRequestBody(req.Body, kubeMFAReplayBodyLimit)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if buffered {
			getBody := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
			req.Body, _ = getBody()
			req.GetBody = getBody
			replayable = true
		} else {
			req.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(data), rest), Closer: rest}
		}
	}

	resp, err := r.inner.RoundTrip(req)
	if err != nil || !isKubeMFAChallenge(resp) {
		return resp, err
	}

	// Drain the challenge response so the underlying connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()

	if !replayable {
		return nil, trace.LimitExceeded(
			"request requires an in-band MFA ceremony but its body exceeds the %d byte replay buffer; run a smaller request first to complete MFA, then retry",
			kubeMFAReplayBodyLimit,
		)
	}

	teleportCluster, _, err := r.m.resolveClusterKey(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	name, err := r.m.runMFACeremony(teleportCluster, sipKey, fingerprint)
	if err != nil {
		return nil, trace.Wrap(err, "completing in-band MFA ceremony")
	}

	retry := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		retry.Body = body
	}
	retry.Header.Set(common.KubeInBandMFAChallengeResponseHeader, name)
	return r.inner.RoundTrip(retry)
}

// requestCertFingerprint returns the session fingerprint of the client certificate the
// middleware supplies for this request.
func (m *KubeMiddleware) requestCertFingerprint(req *http.Request) ([]byte, error) {
	cert, err := m.getCertForRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	leaf, err := utils.TLSCertLeaf(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return common.KubeClientCertFingerprint(leaf), nil
}

// runMFACeremony runs the configured MFA ceremony once per session fingerprint, no matter
// how many challenged requests arrive concurrently; followers share the leader's result.
// It runs on the middleware close context so an individual canceled kubectl request does
// not abort the ceremony other requests are waiting on.
func (m *KubeMiddleware) runMFACeremony(teleportCluster, sipKey string, fingerprint []byte) (string, error) {
	name, err, _ := m.mfaCeremonyGroup.Do(sipKey, func() (any, error) {
		// The stored challenge, if any, just failed server-side verification; drop it.
		m.setChallengeName(sipKey, "")
		name, err := m.mfaCeremony(m.closeContext, teleportCluster, fingerprint)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		m.setChallengeName(sipKey, name)
		return name, nil
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return name.(string), nil
}

func (m *KubeMiddleware) getChallengeName(sipKey string) string {
	m.challengeNamesMu.Lock()
	defer m.challengeNamesMu.Unlock()
	return m.challengeNames[sipKey]
}

func (m *KubeMiddleware) setChallengeName(sipKey, name string) {
	m.challengeNamesMu.Lock()
	defer m.challengeNamesMu.Unlock()
	if name == "" {
		delete(m.challengeNames, sipKey)
		return
	}
	m.challengeNames[sipKey] = name
}

// isKubeMFAChallenge reports whether the response is the kube forwarder's distinguishable
// in-band MFA challenge, as opposed to any other 401.
func isKubeMFAChallenge(resp *http.Response) bool {
	return resp.StatusCode == http.StatusUnauthorized &&
		resp.Header.Get(common.KubeInBandMFAChallengeHeader) == common.KubeInBandMFAChallengeValueRequired
}

// bufferRequestBody reads up to limit bytes from body. If the body ends within the limit
// it is fully consumed and buffered=true is returned. Otherwise the bytes read so far and
// the remainder of the body are returned with buffered=false.
func bufferRequestBody(body io.ReadCloser, limit int64) (data []byte, rest io.ReadCloser, buffered bool, err error) {
	data, err = io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		_ = body.Close()
		return nil, nil, false, trace.Wrap(err)
	}
	if int64(len(data)) <= limit {
		_ = body.Close()
		return data, nil, true, nil
	}
	return data, body, false, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}
