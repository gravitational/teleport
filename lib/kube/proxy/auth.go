// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"

	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	authzapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	// Load kubeconfig auth plugins for gcp and azure.
	// Without this, users can't provide a kubeconfig using those.
	//
	// Note: we don't want to load _all_ plugins. This is a balance between
	// support for popular hosting providers and minimizing attack surface.
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// kubeCreds contain authentication-related fields from kubeconfig.
//
// TODO(awly): make this an interface, one implementation for local k8s cluster
// and another for a remote teleport cluster.
type kubeCreds struct {
	// tlsConfig contains (m)TLS configuration.
	tlsConfig *tls.Config
	// transportConfig contains HTTPS-related configuration.
	// Note: use wrapTransport method if working with http.RoundTrippers.
	transportConfig *transport.Config
	// targetAddr is a kubernetes API address.
	targetAddr string
	kubeClient *kubernetes.Clientset
}

// ImpersonationPermissionsChecker describes a function that can be used to check
// for the required impersonation permissions on a Kubernetes cluster. Return nil
// to indicate success.
type ImpersonationPermissionsChecker func(ctx context.Context, clusterName string,
	sarClient authztypes.SelfSubjectAccessReviewInterface) error

// getKubeCreds fetches the kubernetes API credentials.
//
// There are 2 possible sources of credentials:
// - pod service account credentials: files in hardcoded paths when running
//   inside of a k8s pod; this is used when kubeClusterName is set
// - kubeconfig: a file with a set of k8s endpoints and credentials mapped to
//   them this is used when kubeconfigPath is set
//
// serviceType changes the loading behavior:
// - LegacyProxyService:
//   - if loading from kubeconfig, only "current-context" is returned; the
//     returned map key matches tpClusterName
//   - if no credentials are loaded, no error is returned
//   - permission self-test failures are only logged
// - ProxyService:
//   - no credentials are loaded and no error is returned
// - KubeService:
//   - if loading from kubeconfig, all contexts are returned
//   - if no credentials are loaded, returns an error
//   - permission self-test failures cause an error to be returned
func getKubeCreds(ctx context.Context, log logrus.FieldLogger, tpClusterName, kubeClusterName, kubeconfigPath string, serviceType KubeServiceType, checkImpersonation ImpersonationPermissionsChecker) (map[string]*kubeCreds, error) {
	log.
		WithField("kubeconfigPath", kubeconfigPath).
		WithField("kubeClusterName", kubeClusterName).
		WithField("serviceType", serviceType).
		Debug("Reading Kubernetes creds.")

	// Proxy service should never have creds, forwards to kube service
	if serviceType == ProxyService {
		return map[string]*kubeCreds{}, nil
	}

	// Load kubeconfig or local pod credentials.
	loadAll := serviceType == KubeService
	cfg, err := kubeutils.GetKubeConfig(kubeconfigPath, loadAll, kubeClusterName)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if trace.IsNotFound(err) || len(cfg.Contexts) == 0 {
		switch serviceType {
		case KubeService:
			return nil, trace.BadParameter("no Kubernetes credentials found; Kubernetes_service requires either a valid kubeconfig_file or to run inside of a Kubernetes pod")
		case LegacyProxyService:
			log.Debugf("Could not load Kubernetes credentials. This proxy will still handle Kubernetes requests for trusted teleport clusters or Kubernetes nodes in this teleport cluster")
		}
		return map[string]*kubeCreds{}, nil
	}

	if serviceType == LegacyProxyService {
		// Hack for legacy proxy service - register a k8s cluster named after
		// the teleport cluster name to route legacy requests.
		//
		// Also, remove all other contexts. Multiple kubeconfig entries are
		// only supported for kubernetes_service.
		if currentContext, ok := cfg.Contexts[cfg.CurrentContext]; ok {
			cfg.Contexts = map[string]*rest.Config{
				tpClusterName: currentContext,
			}
		} else {
			return nil, trace.BadParameter("no Kubernetes current-context found; Kubernetes proxy service requires either a valid kubeconfig_file with a current-context or to run inside of a Kubernetes pod")
		}
	}

	res := make(map[string]*kubeCreds, len(cfg.Contexts))
	// Convert kubeconfig contexts into kubeCreds.
	for cluster, clientCfg := range cfg.Contexts {
		clusterCreds, err := extractKubeCreds(ctx, cluster, clientCfg, serviceType, kubeconfigPath, log, checkImpersonation)
		if err != nil {
			log.WithError(err).Warnf("failed to load credentials for cluster %q.", cluster)
			continue
		}
		res[cluster] = clusterCreds
	}
	return res, nil
}

func extractKubeCreds(ctx context.Context, cluster string, clientCfg *rest.Config, serviceType KubeServiceType, kubeconfigPath string, log logrus.FieldLogger, checkPermissions ImpersonationPermissionsChecker) (*kubeCreds, error) {
	log = log.WithField("cluster", cluster)

	log.Debug("Checking Kubernetes impersonation permissions.")
	client, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate Kubernetes client for cluster %q", cluster)
	}

	// For each loaded cluster, check impersonation permissions. This
	// check only logs when permissions are not configured, but does not fail startup.
	if err := checkPermissions(ctx, cluster, client.AuthorizationV1().SelfSubjectAccessReviews()); err != nil {
		log.WithError(err).Warning("Failed to test the necessary Kubernetes permissions. The target Kubernetes cluster may be down or have misconfigured RBAC. This teleport instance will still handle Kubernetes requests towards this Kubernetes cluster.")
		if serviceType != KubeService && kubeconfigPath != "" {
			// We used to recommend users to set a dummy kubeconfig on root
			// proxies to get kubernetes support working for leaf clusters:
			// https://community.goteleport.com/t/enabling-teleport-to-act-as-a-kubernetes-proxy-for-trusted-leaf-clusters/418
			//
			// Since this is no longer necessary, recommend them to clean up
			// via logs.
			log.Info("If this is a proxy and you provided a dummy kubeconfig_file, you can remove it from teleport.yaml to get rid of this warning")
		}
	} else {
		log.Debug("Have all necessary Kubernetes impersonation permissions.")
	}

	targetAddr, err := parseKubeHost(clientCfg.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := rest.TLSConfigFor(clientCfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate TLS config from kubeconfig: %v", err)
	}
	if tlsConfig == nil {
		cc := rest.AnonymousClientConfig(clientCfg)
		if len(cc.CAData) != 0 {
			cc.CAData = []byte("REDACTED")
		}
		return nil, trace.BadParameter("failed to generate TLS config from kubeConfig. clientConfig: %s", cc.String())
	}
	transportConfig, err := clientCfg.TransportConfig()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate transport config from kubeconfig: %v", err)
	}

	log.Debug("Initialized Kubernetes credentials")
	return &kubeCreds{
		tlsConfig:       tlsConfig,
		transportConfig: transportConfig,
		targetAddr:      targetAddr,
		kubeClient:      client,
	}, nil
}

// parseKubeHost parses and formats kubernetes hostname
// to host:port format, if no port it set,
// it assumes default HTTPS port
func parseKubeHost(host string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", trace.Wrap(err, "failed to parse Kubernetes host: %v", err)
	}
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		// add default HTTPS port
		return fmt.Sprintf("%v:443", u.Host), nil
	}
	return u.Host, nil
}

func (c *kubeCreds) wrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	if c == nil {
		return rt, nil
	}
	return transport.HTTPWrappersForConfig(c.transportConfig, rt)
}

func checkImpersonationPermissions(ctx context.Context, cluster string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
	for _, resource := range []string{"users", "groups", "serviceaccounts"} {
		resp, err := sarClient.Create(ctx, &authzapi.SelfSubjectAccessReview{
			Spec: authzapi.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authzapi.ResourceAttributes{
					Verb:     "impersonate",
					Resource: resource,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return trace.Wrap(err, "failed to verify impersonation permissions for Kubernetes: %v; this may be due to missing the SelfSubjectAccessReview permission on the ClusterRole used by the proxy; please make sure that proxy has all the necessary permissions: https://goteleport.com/teleport/docs/kubernetes-ssh/#impersonation", err)
		}
		if !resp.Status.Allowed {
			return trace.AccessDenied("proxy can't impersonate Kubernetes %s at the cluster level; please make sure that proxy has all the necessary permissions: https://goteleport.com/teleport/docs/kubernetes-ssh/#impersonation", resource)
		}
	}
	return nil
}
