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
	"crypto/tls"
	"errors"
	"maps"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

// kubeCertIssuer issues per-cluster Kubernetes certificates.
// Under per-session MFA it runs one reusable USER_SESSION ceremony and replays the response across issuances.
// Any issuance that may prompt the user runs under a single-flight lock,
// so at most one prompt is active at any time, however the issuances are scheduled.
type kubeCertIssuer struct {
	tc *client.TeleportClient
	// keyStore persists certs not gated by MFA.
	keyStore kubeKeyStore
	// conn is the cluster connection shared by in-flight operations, so none is held while idle.
	conn *clusterConn
	// mfa is the reusable MFA state shared across issuances.
	mfa *reusableMFA
}

// kubeKeyStore is the subset of [client.LocalKeyAgent] the issuer saves certs through.
type kubeKeyStore interface {
	AddKubeKeyRing(keyRing *client.KeyRing) error
}

func newKubeCertIssuer(tc *client.TeleportClient) *kubeCertIssuer {
	return &kubeCertIssuer{
		tc:       tc,
		keyStore: tc.LocalAgent(),
		conn:     newClusterConn(tc),
		mfa:      newReusableMFA(),
	}
}

// LoadOrIssueCerts returns certs for the given clusters, loading them from
// the key store where a valid cert is stored and issuing the rest.
func (issuer *kubeCertIssuer) LoadOrIssueCerts(ctx context.Context, clusters kubeconfig.LocalProxyClusters) (alpnproxy.KubeClientCerts, error) {
	ctx, span := issuer.tc.Tracer.Start(ctx, "kubeCertIssuer/loadOrIssueCerts")
	defer span.End()

	kubeKeys, err := issuer.loadKubeKeyRings(clusters.TeleportClusters())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs := make(alpnproxy.KubeClientCerts)
	var missing kubeconfig.LocalProxyClusters
	for _, cluster := range clusters {
		// Try load from store.
		if key := kubeKeys[cluster.TeleportCluster]; key != nil {
			cert, err := kubeCertFromKeyRing(key, cluster.KubeCluster)
			if err == nil {
				logger.DebugContext(ctx, "Client cert loaded from keystore for cluster", "cluster", cluster)
				certs.Add(cluster.TeleportCluster, cluster.KubeCluster, cert)
				continue
			}
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		}
		missing = append(missing, cluster)
	}
	if len(missing) == 0 {
		return certs, nil
	}

	issued, err := issuer.issueCerts(ctx, missing)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	maps.Copy(certs, issued)
	return certs, nil
}

// IssueCert issues one cert for the given cluster.
// If the cluster has per-session MFA, it replays the shared reusable MFA response if one is held.
// If no reusable response is held, it takes the single-flight ceremony path, which may prompt the user.
func (issuer *kubeCertIssuer) IssueCert(ctx context.Context, teleportCluster, kubeCluster string, mfaCheck *proto.IsMFARequiredResponse) (*tls.Certificate, error) {
	// Hold one connection across the issuance. Every attempt below shares it.
	cc, release, err := issuer.conn.Acquire(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	params := client.ReissueParams{
		RouteToCluster:    teleportCluster,
		KubernetesCluster: kubeCluster,
		TTL:               issuer.tc.KeyTTL,
		MFACheck:          mfaCheck,
	}

	// Headless MFA responses cannot be reused: every issuance prompts, one at a time.
	// "proxy kube" sets AllowHeadless only when running with --headless.
	if issuer.tc.AllowHeadless {
		params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS
		release, err := issuer.mfa.AcquireCeremonyLock(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer release()
		return issuer.requestCert(ctx, cc, params)
	}

	// MFA is known to be off for this cluster: plain issuance, no prompt possible.
	if mfaCheck != nil && !mfaCheck.GetRequired() {
		params.RequesterName, _ = issuer.mfa.State()
		return issuer.requestCert(ctx, cc, params)
	}

	return issuer.issueMFAGatedCert(ctx, cc, params)
}

func (issuer *kubeCertIssuer) loadKubeKeyRings(teleportClusters []string) (map[string]*client.KeyRing, error) {
	kubeKeys := map[string]*client.KeyRing{}
	for _, teleportCluster := range teleportClusters {
		keyRing, err := issuer.tc.LocalAgent().GetKeyRing(teleportCluster, client.WithKubeCerts{})
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		kubeKeys[teleportCluster] = keyRing
	}
	return kubeKeys, nil
}

// issueCerts issues certs for the given clusters with at most one MFA ceremony.
// One issuance runs the ceremony and the rest replay its reusable response.
// Clusters without per-session MFA are issued serially, as their certs are saved to the key store.
func (issuer *kubeCertIssuer) issueCerts(ctx context.Context, clusters kubeconfig.LocalProxyClusters) (alpnproxy.KubeClientCerts, error) {
	// Hold one connection across the whole burst: the MFA prefetch and the issuances share it.
	cc, release, err := issuer.conn.Acquire(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	mfaChecks, err := issuer.fetchMFAChecks(ctx, cc, clusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Partition clusters into MFA-gated and prompt-free.
	var mfaOn, mfaOff kubeconfig.LocalProxyClusters
	for _, cluster := range clusters {
		if mfaChecks[localProxyClusterKey(cluster)].GetRequired() {
			mfaOn = append(mfaOn, cluster)
		} else {
			mfaOff = append(mfaOff, cluster)
		}
	}

	certs := make(alpnproxy.KubeClientCerts)
	var certsMu sync.Mutex
	issueAndAdd := func(ctx context.Context, cluster kubeconfig.LocalProxyCluster) error {
		mfaCheck := mfaChecks[localProxyClusterKey(cluster)]
		cert, err := issuer.IssueCert(ctx, cluster.TeleportCluster, cluster.KubeCluster, mfaCheck)
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(ctx, "Client cert issued for cluster", "cluster", cluster)
		certsMu.Lock()
		defer certsMu.Unlock()
		certs.Add(cluster.TeleportCluster, cluster.KubeCluster, *cert)
		return nil
	}

	// MFA-gated issuances fan out concurrently.
	group := newKubeClusterGroup(cc, mfaOn, kubeCertIssueConcurrency())
	defer group.Close(ctx)
	if err := group.ForEach(ctx, issueAndAdd); err != nil {
		return nil, trace.Wrap(err)
	}

	// Clusters without per-session MFA issue serially.
	for _, cluster := range mfaOff {
		if err := issueAndAdd(ctx, cluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return certs, nil
}

// issueMFAGatedCert issues one cert that may require MFA.
// It replays the shared reusable response when one is held, and takes the single-flight ceremony path otherwise.
// Rejected replays loop back into the ceremony path to refresh the response.
func (issuer *kubeCertIssuer) issueMFAGatedCert(ctx context.Context, cc kubeCertClient, params client.ReissueParams) (*tls.Certificate, error) {
	// Tolerate up to 3 rejected replays before giving up. Only two rejections are recoverable:
	// - The replayed response outlived the server's reuse window and expired.
	// - This auth server does not allow reuse at all (a mixed-version auth pool).
	// The worst honest sequence is one of each, so the bound leaves margin while
	// making termination unconditional. Any other error fails the issuance immediately.
	const maxRejections = 3
	var rejections int
	var lastErr error
	for rejections < maxRejections {
		requester, reusable := issuer.mfa.State()

		if reusable == nil {
			cert, done, err := issuer.issueWithCeremony(ctx, cc, params)
			if !done {
				// A peer's ceremony captured a fresh response. Replay it.
				continue
			}
			return cert, trace.Wrap(err)
		}

		params.RequesterName = requester
		params.ReusableMFAResponse = reusable
		params.FailOnExpiredReusableMFAResponse = true
		cert, err := issuer.requestCert(ctx, cc, params)
		switch {
		case errors.Is(err, &mfa.ErrExpiredReusableMFAResponse):
			// The response expired mid-flight. Drop it and loop into the ceremony path.
			issuer.mfa.Clear(reusable)
		case isMFAReuseRejected(err):
			// This auth server does not allow reuse (a mixed-version auth pool).
			issuer.mfa.FallbackToLegacy(ctx, err)
		default:
			return cert, trace.Wrap(err)
		}
		rejections++
		lastErr = err
		params.ReusableMFAResponse = nil
		params.FailOnExpiredReusableMFAResponse = false
	}
	return nil, trace.Wrap(lastErr, "issuing certificate for Kubernetes cluster %q: giving up after %d rejected MFA responses", params.KubernetesCluster, maxRejections)
}

// issueWithCeremony runs an issuance that may prompt the user, under the single-flight lock.
// It reports done=false without issuing when the ceremony it waited on captured a reusable response.
// The caller replays it instead, with no prompt.
func (issuer *kubeCertIssuer) issueWithCeremony(ctx context.Context, cc kubeCertClient, params client.ReissueParams) (cert *tls.Certificate, done bool, err error) {
	release, err := issuer.mfa.AcquireCeremonyLock(ctx)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	defer release()

	requester, reusable := issuer.mfa.State()
	if reusable != nil {
		return nil, false, nil
	}

	params.RequesterName = requester
	params.ReusableMFAResponse = nil
	params.FailOnExpiredReusableMFAResponse = false
	cert, err = issuer.requestCert(ctx, cc, params)
	if requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI && isMFAReuseRejected(err) {
		// An auth server that predates the requester rejected the ceremony.
		issuer.mfa.FallbackToLegacy(ctx, err)
		params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
		cert, err = issuer.requestCert(ctx, cc, params)
	}
	return cert, true, trace.Wrap(err)
}

// requestCert requests one cert from the cluster, with no single-flight lock.
func (issuer *kubeCertIssuer) requestCert(ctx context.Context, cc kubeCertClient, params client.ReissueParams) (*tls.Certificate, error) {
	result, err := cc.IssueUserCertsWithMFA(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Save the reusable MFA response produced by a fresh ceremony for the other issuances to replay.
	if result.ReusableMFAResponse != nil && params.RequesterName == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
		issuer.mfa.Capture(result.ReusableMFAResponse)
	}

	// Save it if MFA was not required.
	if result.MFARequired == proto.MFARequired_MFA_REQUIRED_NO {
		if err := issuer.keyStore.AddKubeKeyRing(result.KeyRing); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	cert, err := result.KeyRing.KubeTLSCert(params.KubernetesCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set leaf so we don't have to parse it on each request.
	if cert.Leaf, err = utils.TLSCertLeaf(cert); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cert, nil
}

// kubeCertClient is the subset of [client.ClusterClient] the issuer issues through.
type kubeCertClient interface {
	IssueUserCertsWithMFA(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error)
	ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error)
	Close() error
}

// fetchMFAChecks checks the per-session MFA requirement for every cluster concurrently,
// so issuance can be partitioned into prompt-free and MFA-gated work up front.
func (issuer *kubeCertIssuer) fetchMFAChecks(ctx context.Context, cc kubeCertClient, clusters kubeconfig.LocalProxyClusters) (map[string]*proto.IsMFARequiredResponse, error) {
	checks := make(map[string]*proto.IsMFARequiredResponse, len(clusters))
	var checksMu sync.Mutex

	group := newKubeClusterGroup(cc, clusters, kubeCertIssueConcurrency())
	defer group.Close(ctx)

	err := group.ForEach(ctx, func(ctx context.Context, cluster kubeconfig.LocalProxyCluster) error {
		authClient, err := group.AuthClient(ctx, cluster.TeleportCluster)
		if err != nil {
			return trace.Wrap(err)
		}
		resp, err := authClient.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: cluster.KubeCluster},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		checksMu.Lock()
		defer checksMu.Unlock()
		checks[localProxyClusterKey(cluster)] = resp
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checks, nil
}

func kubeCertIssueConcurrency() int {
	const (
		// defaultKubeCertIssueConcurrency bounds concurrent per-cluster cert
		// issuances so a large fan-out does not overwhelm the auth server.
		defaultKubeCertIssueConcurrency = 10
		// kubeCertIssueConcurrencyEnvVar overrides the concurrency bound.
		// It is a tuning and benchmarking knob, not a supported interface.
		kubeCertIssueConcurrencyEnvVar = "TELEPORT_UNSTABLE_KUBE_CERT_ISSUE_CONCURRENCY"
	)
	if v := os.Getenv(kubeCertIssueConcurrencyEnvVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultKubeCertIssueConcurrency
}

func kubeCertFromKeyRing(keyRing *client.KeyRing, kubeCluster string) (tls.Certificate, error) {
	x509cert, err := keyRing.KubeX509Cert(kubeCluster)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if time.Until(x509cert.NotAfter) <= time.Minute {
		return tls.Certificate{}, trace.NotFound("TLS cert is expiring in a minute")
	}
	cert, err := keyRing.KubeTLSCert(kubeCluster)
	return cert, trace.Wrap(err)
}

func localProxyClusterKey(cluster kubeconfig.LocalProxyCluster) string {
	return cluster.TeleportCluster + "/" + cluster.KubeCluster
}

func isMFAReuseRejected(err error) bool {
	return trace.IsAccessDenied(err) && strings.Contains(err.Error(), "reuse is not permitted")
}
