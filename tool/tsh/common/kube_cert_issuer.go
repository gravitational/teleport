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
// Under per-session MFA it runs one ceremony with the reusable KUBE_LOCAL_PROXY_MULTI challenge scope
// and replays the response across issuances.
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
	// sharedCertUnsupported latches once an auth server has rejected an unrouted request.
	sharedCertUnsupported bool
}

// kubeKeyStore is the subset of [client.LocalKeyAgent] the issuer loads and saves certs through.
type kubeKeyStore interface {
	GetKeyRing(clusterName string, opts ...client.CertOption) (*client.KeyRing, error)
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
// A nil mfaCheck means the MFA requirement is unknown: the issuance takes the MFA-gated path,
// which degrades to a plain issuance when the server reports MFA as not required.
// An empty kubeCluster issues the shared unrouted cert for teleportCluster,
// which the proxy path-routes to any of its Kubernetes clusters. It never prompts and is never persisted.
func (issuer *kubeCertIssuer) IssueCert(ctx context.Context, teleportCluster, kubeCluster string, mfaCheck *proto.IsMFARequiredResponse) (*tls.Certificate, error) {
	// Hold one connection across the issuance. Every attempt below shares it.
	cc, release, err := issuer.conn.Acquire(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	cert, err := issuer.issueCertOverConn(ctx, cc, teleportCluster, kubeCluster, mfaCheck)
	if err != nil && client.IsErrorResolvableWithRelogin(err) {
		issuer.conn.invalidate(ctx)
	}
	return cert, trace.Wrap(err)
}

// issueCertOverConn issues one cert for the given cluster over the given connection.
func (issuer *kubeCertIssuer) issueCertOverConn(ctx context.Context, cc kubeCertClient, teleportCluster, kubeCluster string, mfaCheck *proto.IsMFARequiredResponse) (*tls.Certificate, error) {
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

	// Unrouted certs are issued for the proxy to route to any of its clusters.
	if kubeCluster == "" {
		params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI
		params.MFACheck = &proto.IsMFARequiredResponse{
			Required:    false,
			MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}
		return issuer.requestCert(ctx, cc, params)
	}

	// MFA is known to be off for this cluster: plain issuance, no prompt possible.
	if mfaCheck != nil && !mfaCheck.GetRequired() {
		params.RequesterName, _ = issuer.mfa.State()
		return issuer.requestCert(ctx, cc, params)
	}

	return issuer.issueMFAGatedCert(ctx, cc, params)
}

// ReissueSharedCert reissues the shared unrouted cert on behalf of the cluster whose request needs it,
// and reports the kube cluster to store the result under.
//
// A shared cert only serves clusters whose per-session MFA is off, and that can change while the proxy runs.
// So the reissue rechecks the requesting cluster: if MFA is now required, that cluster gets a cert routed to itself,
// since a shared cert can carry no MFA state. The rest of the fleet keeps using the shared one.
func (issuer *kubeCertIssuer) ReissueSharedCert(ctx context.Context, teleportCluster, requestedKubeCluster string) (*tls.Certificate, string, error) {
	cc, release, err := issuer.conn.Acquire(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer release()

	authClient, err := cc.ConnectToCluster(ctx, teleportCluster)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer authClient.Close()

	check, err := authClient.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: requestedKubeCluster},
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	if check.GetRequired() {
		logger.DebugContext(ctx, "Cluster now requires per-session MFA, giving it a routed cert",
			"teleport_cluster", teleportCluster,
			"kube_cluster", requestedKubeCluster,
		)
		cert, err := issuer.IssueCert(ctx, teleportCluster, requestedKubeCluster, check)
		return cert, requestedKubeCluster, trace.Wrap(err)
	}

	// The cluster can still use the shared cert, so reissue it.
	cert, err := issuer.IssueCert(ctx, teleportCluster, "" /*kubeCluster*/, nil /*mfaCheck*/)
	return cert, "", trace.Wrap(err)
}

// AcquireConn holds the shared cluster connection until the returned release is called.
// Dialing it performs a relogin if the base session is expired.
func (issuer *kubeCertIssuer) AcquireConn(ctx context.Context) (func(), error) {
	_, release, err := issuer.conn.Acquire(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return release, nil
}

func (issuer *kubeCertIssuer) loadKubeKeyRings(teleportClusters []string) (map[string]*client.KeyRing, error) {
	kubeKeys := map[string]*client.KeyRing{}
	for _, teleportCluster := range teleportClusters {
		keyRing, err := issuer.keyStore.GetKeyRing(teleportCluster, client.WithKubeCerts{})
		if trace.IsNotFound(err) {
			// No keys stored for this cluster: its certs are issued fresh.
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeKeys[teleportCluster] = keyRing
	}
	return kubeKeys, nil
}

// issueCerts issues certs for the given clusters with at most one MFA ceremony.
// One issuance runs the ceremony and the rest replay its reusable response.
// Clusters without per-session MFA share one unrouted cert per Teleport cluster.
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
	run := issuer.newCertRun(mfaChecks)

	// Headless serializes every issuance on the ceremony lock, so the partition below cannot help it.
	if issuer.tc.AllowHeadless {
		return run.certs, trace.Wrap(run.IssuePerCluster(ctx, clusters))
	}

	mfaOn, mfaOff := run.PartitionByMFA(clusters)

	// MFA-gated issuances fan out concurrently.
	group := newKubeClusterGroup(cc, mfaOn, kubeCertIssueConcurrency())
	defer group.Close(ctx)
	if err := group.ForEach(ctx, run.IssueOne); err != nil {
		return nil, trace.Wrap(err)
	}

	if issuer.sharedCertUnsupported {
		return run.certs, trace.Wrap(run.IssuePerCluster(ctx, mfaOff))
	}

	// Prompt-free issuances share one unrouted cert per Teleport cluster.
	if err := run.IssueShared(ctx, mfaOff); err != nil {
		if !isUnroutedKubeCertRejected(err) {
			return nil, trace.Wrap(err)
		}
		// The auth server rejected the shared unrouted cert, so issue per cluster instead.
		logger.DebugContext(ctx, "Auth server rejected the shared unrouted cert, issuing per cluster", "error", err)
		issuer.sharedCertUnsupported = true
		return run.certs, trace.Wrap(run.IssuePerCluster(ctx, mfaOff))
	}
	return run.certs, nil
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
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !done {
				// A peer's ceremony captured a fresh response. Replay it.
				continue
			}
			return cert, nil
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
	if requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
		switch {
		case isMFAReuseRejected(err):
			// An auth server that predates the multi-requester rejected the ceremony.
			issuer.mfa.FallbackToLegacy(ctx, err)
			params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
			cert, err = issuer.requestCert(ctx, cc, params)
		case isMFAReuseRejectionSuspected(err):
			// A masked rejection from an old auth server and
			// a transient challenge-creation failure are indistinguishable, so fall back for this issuance only.
			// Every fresh ceremony probes the reusable requester again.
			// A transient failure cannot degrade the proxy permanently,
			// and an old auth server costs one failed request per ceremony.
			logger.DebugContext(ctx, "MFA challenge creation failed, retrying this issuance with a per-cluster ceremony", "error", err)
			params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
			cert, err = issuer.requestCert(ctx, cc, params)
		}
	}
	return cert, true, trace.Wrap(err)
}

// requestCert requests one cert from the cluster, with no single-flight lock.
func (issuer *kubeCertIssuer) requestCert(ctx context.Context, cc kubeCertClient, params client.ReissueParams) (*tls.Certificate, error) {
	unrouted := params.KubernetesCluster == ""
	if unrouted && params.RequesterName != proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
		return nil, trace.BadParameter("unrouted Kubernetes certificates can only be requested by %v, got %v",
			proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI, params.RequesterName)
	}

	result, err := cc.IssueUserCertsWithMFA(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Save the reusable MFA response produced by a fresh ceremony for the other issuances to replay.
	if result.ReusableMFAResponse != nil {
		issuer.mfa.Capture(result.ReusableMFAResponse)
	}

	// Save to the keystore if MFA was not required.
	// The unrouted cert stays in memory.
	if result.MFARequired == proto.MFARequired_MFA_REQUIRED_NO && !unrouted {
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

// isUnroutedKubeCertRejected reports whether an auth server refused an unrouted request,
// as every server predating the shared cert does.
func isUnroutedKubeCertRejected(err error) bool {
	// validateCertUsage rejects the request before it reaches any typed error.
	return trace.IsBadParameter(err) && strings.Contains(err.Error(), "missing KubernetesCluster field")
}

// isMFAReuseRejected reports whether an auth server unambiguously rejected the reusable MFA flow.
func isMFAReuseRejected(err error) bool {
	// Auth servers that validate challenge scopes reject the unknown kube scope with a typed error at challenge creation.
	if errors.Is(err, &mfa.ErrUnknownChallengeScope) {
		return true
	}
	// Servers that predate scope validation reject the response at validation without a typed error, recognized by message below.
	if trace.IsAccessDenied(err) || trace.IsBadParameter(err) {
		msg := err.Error()
		return strings.Contains(msg, "is not satisfied by the given") || // response scope unknown to the server, rejected at validation
			strings.Contains(msg, "reuse is not permitted") // server knows the scope but does not allow reuse for the requester
	}
	return false
}

// isMFAReuseRejectionSuspected reports whether the error may be a masked reuse rejection.
// Auth servers that predate challenge scope validation reject the unknown kube scope
// at challenge creation, masked behind their generic challenge-creation failure message.
func isMFAReuseRejectionSuspected(err error) bool {
	return trace.IsAccessDenied(err) && strings.Contains(err.Error(), "unable to create MFA challenges")
}
