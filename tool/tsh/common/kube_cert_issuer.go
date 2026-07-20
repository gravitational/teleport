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
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

func kubeCertIssueConcurrency() int {
	const (
		// defaultKubeCertIssueConcurrency bounds concurrent per-cluster cert
		// issuances so a large fan-out does not overwhelm the auth server.
		defaultKubeCertIssueConcurrency = 10
		// kubeCertIssueConcurrencyEnvVar overrides the concurrency bound.
		// It is a tuning and benchmarking knob, not a supported interface.
		kubeCertIssueConcurrencyEnvVar = "TELEPORT_KUBE_CERT_ISSUE_CONCURRENCY"
	)
	if v := os.Getenv(kubeCertIssueConcurrencyEnvVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultKubeCertIssueConcurrency
}

// kubeCertClient is the subset of [client.ClusterClient] the issuer issues through.
type kubeCertClient interface {
	IssueUserCertsWithMFA(ctx context.Context, params client.ReissueParams) (*client.IssueUserCertsWithMFAResult, error)
	ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error)
}

// kubeCertIssuer issues per-cluster Kubernetes certificates.
// Under per-session MFA it runs one reusable USER_SESSION ceremony
// and replays the response across issuances,
// so the user is prompted once per issuance wave instead of once per cluster.
type kubeCertIssuer struct {
	tc *client.TeleportClient
	// saveKeyRing persists certs not gated by MFA to the key store.
	saveKeyRing func(keyRing *client.KeyRing) error

	mu sync.Mutex
	// requester starts as TSH_KUBE_LOCAL_PROXY_MULTI and permanently drops to
	// the legacy TSH_KUBE_LOCAL_PROXY, with one non-reusable ceremony per cluster,
	// once an auth server rejects reuse.
	requester           proto.UserCertsRequest_Requester
	reusableMFAResponse *proto.MFAAuthenticateResponse
}

func newKubeCertIssuer(tc *client.TeleportClient) *kubeCertIssuer {
	return &kubeCertIssuer{
		tc:        tc,
		requester: proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
		saveKeyRing: func(keyRing *client.KeyRing) error {
			return trace.Wrap(tc.LocalAgent().AddKubeKeyRing(keyRing))
		},
	}
}

// issueCerts issues certs for the given clusters with at most one MFA ceremony:
// MFA requirements are prefetched concurrently,
// the first MFA-gated cluster is issued serially to run the single reusable ceremony,
// and the remaining MFA-gated clusters are issued concurrently replaying its response.
// Clusters without per-session MFA are issued serially, as their certs are
// saved to the key store, which does not tolerate concurrent use.
func (i *kubeCertIssuer) issueCerts(ctx context.Context, cc kubeCertClient, clusters kubeconfig.LocalProxyClusters) (alpnproxy.KubeClientCerts, error) {
	mfaChecks, err := i.fetchMFARequired(ctx, cc, clusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
		cert, err := i.issueCert(ctx, cc, cluster.TeleportCluster, cluster.KubeCluster, mfaChecks[localProxyClusterKey(cluster)])
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(ctx, "Client cert issued for cluster", "cluster", cluster)
		certsMu.Lock()
		defer certsMu.Unlock()
		certs.Add(cluster.TeleportCluster, cluster.KubeCluster, *cert)
		return nil
	}

	if len(mfaOn) > 0 {
		// The first MFA-gated issuance runs serially: it performs the one
		// MFA ceremony (or detects an older auth server and switches to the
		// per-cluster fallback). It runs before everything else so the user
		// is prompted right at command start.
		if err := issueAndAdd(ctx, mfaOn[0]); err != nil {
			return nil, trace.Wrap(err)
		}
		if i.fallbackActive() {
			// Older auth server: every MFA-gated issuance prompts, so keep
			// them serial to prompt one at a time, as before.
			for _, cluster := range mfaOn[1:] {
				if err := issueAndAdd(ctx, cluster); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		} else {
			// MFA-gated issuances replay the reusable response and never
			// write the key store, so they are safe to run concurrently.
			g, gctx := errgroup.WithContext(ctx)
			g.SetLimit(kubeCertIssueConcurrency())
			for _, cluster := range mfaOn[1:] {
				g.Go(func() error {
					return trace.Wrap(issueAndAdd(gctx, cluster))
				})
			}
			if err := g.Wait(); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	// Clusters without per-session MFA issue serially: these issuances save
	// certs to the key store, and the key store is not safe for concurrent
	// use (issuance reads key rings from disk that a concurrent save may be
	// rewriting).
	for _, cluster := range mfaOff {
		if err := issueAndAdd(ctx, cluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return certs, nil
}

// issueCert issues one cert, replaying the reusable MFA response
// when one is held and capturing the response produced by a fresh ceremony.
func (i *kubeCertIssuer) issueCert(ctx context.Context, cc kubeCertClient, teleportCluster, kubeCluster string, mfaCheck *proto.IsMFARequiredResponse) (*tls.Certificate, error) {
	params := client.ReissueParams{
		RouteToCluster:    teleportCluster,
		KubernetesCluster: kubeCluster,
		TTL:               i.tc.KeyTTL,
		MFACheck:          mfaCheck,
	}

	// Headless keeps its dedicated requester and non-reusable ceremony.
	// "proxy kube" sets AllowHeadless only when running with --headless, so it means the user is in headless mode.
	if i.tc.AllowHeadless {
		params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS
		return i.issue(ctx, cc, params)
	}

	i.mu.Lock()
	requester := i.requester
	reusable := i.reusableMFAResponse
	i.mu.Unlock()

	params.RequesterName = requester
	params.ReusableMFAResponse = reusable
	cert, err := i.issue(ctx, cc, params)
	if requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI && isMFAReuseRejected(err) {
		logger.DebugContext(ctx, "Auth server does not allow reusable MFA for the kube local proxy, falling back to per-cluster MFA ceremonies", "error", err)
		i.mu.Lock()
		i.requester = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
		i.reusableMFAResponse = nil
		i.mu.Unlock()
		params.RequesterName = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
		params.ReusableMFAResponse = nil
		return i.issue(ctx, cc, params)
	}
	return cert, trace.Wrap(err)
}

func (i *kubeCertIssuer) issue(ctx context.Context, cc kubeCertClient, params client.ReissueParams) (*tls.Certificate, error) {
	result, err := cc.IssueUserCertsWithMFA(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Save the reusable MFA response produced by a fresh ceremony.
	if result.ReusableMFAResponse != nil {
		i.mu.Lock()
		if i.requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
			i.reusableMFAResponse = result.ReusableMFAResponse
		}
		i.mu.Unlock()
	}

	// Save it if MFA was not required.
	if result.MFARequired == proto.MFARequired_MFA_REQUIRED_NO {
		if err := i.saveKeyRing(result.KeyRing); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	cert, err := result.KeyRing.KubeTLSCert(params.KubernetesCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set leaf so we don't have to parse it on each request.
	leaf, err := utils.TLSCertLeaf(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert.Leaf = leaf

	return &cert, nil
}

// fetchMFARequired checks the per-session MFA requirement for every cluster concurrently,
// so issuance can be partitioned into prompt-free and MFA-gated work up front.
func (i *kubeCertIssuer) fetchMFARequired(ctx context.Context, cc kubeCertClient, clusters kubeconfig.LocalProxyClusters) (map[string]*proto.IsMFARequiredResponse, error) {
	checks := make(map[string]*proto.IsMFARequiredResponse, len(clusters))
	var checksMu sync.Mutex

	byTeleportCluster := make(map[string]kubeconfig.LocalProxyClusters)
	for _, cluster := range clusters {
		byTeleportCluster[cluster.TeleportCluster] = append(byTeleportCluster[cluster.TeleportCluster], cluster)
	}

	for teleportCluster, group := range byTeleportCluster {
		authClient, err := cc.ConnectToCluster(ctx, teleportCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer func() {
			if err := authClient.Close(); err != nil {
				logger.WarnContext(ctx, "Failed to close auth client", "error", err)
			}
		}()
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(kubeCertIssueConcurrency())
		for _, cluster := range group {
			g.Go(func() error {
				resp, err := authClient.IsMFARequired(gctx, &proto.IsMFARequiredRequest{
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
		}
		if err := g.Wait(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return checks, nil
}

func (i *kubeCertIssuer) fallbackActive() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
}

func localProxyClusterKey(cluster kubeconfig.LocalProxyCluster) string {
	return cluster.TeleportCluster + "/" + cluster.KubeCluster
}

func isMFAReuseRejected(err error) bool {
	return trace.IsAccessDenied(err) && strings.Contains(err.Error(), "reuse is not permitted")
}
