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

var skipSelfPermissionCheck bool

// TestOnlySkipSelfPermissionCheck sets whether or not to skip checking k8s
// impersonation permissions granted to this instance.
//
// Used in CI integration tests, where we intentionally scope down permissions
// from what a normal prod instance should have.
func TestOnlySkipSelfPermissionCheck(skip bool) {
	skipSelfPermissionCheck = skip
}

// getKubeCreds fetches the kubernetes API credentials.
//
// There are 2 possible sources of credentials:
// - pod service account credentials: files in hardcoded paths when running
//   inside of a k8s pod; this is used when kubeClusterName is set
// - kubeconfig: a file with a set of k8s endpoints and credentials mapped to
//   them this is used when kubeconfigPath is set
//
// newKubeService changes the loading behavior:
// - false:
//   - if loading from kubeconfig, only "current-context" is returned; the
//     returned map key matches tpClusterName
//   - if no credentials are loaded, no error is returned
//   - permission self-test failures are only logged
// - true:
//   - if loading from kubeconfig, all contexts are returned
//   - if no credentials are loaded, returns an error
//   - permission self-test failures cause an error to be returned
func getKubeCreds(ctx context.Context, log logrus.FieldLogger, tpClusterName, kubeClusterName, kubeconfigPath string, newKubeService bool) (map[string]*kubeCreds, error) {
	log.
		WithField("kubeconfigPath", kubeconfigPath).
		WithField("kubeClusterName", kubeClusterName).
		WithField("newKubeService", newKubeService).
		Debug("Reading kubernetes creds.")

	// Load kubeconfig or local pod credentials.
	cfg, err := kubeutils.GetKubeConfig(kubeconfigPath, newKubeService, kubeClusterName)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) || len(cfg.Contexts) == 0 {
		if newKubeService {
			return nil, trace.BadParameter("no kubernetes credentials found; kubernetes_service requires either a valid kubeconfig_path or to run inside of a kubernetes pod")
		}
		log.Debugf("Could not load kubernetes credentials. This proxy will still handle kubernetes requests for trusted teleport clusters or kubernetes nodes in this teleport cluster")
		return map[string]*kubeCreds{}, nil
	}
	if !newKubeService {
		// Hack for proxy_service - register a k8s cluster named after the
		// teleport cluster name to route legacy requests.
		//
		// Also, remove all other contexts. Multiple kubeconfig entries are
		// only supported for kubernetes_service.
		cfg.Contexts = map[string]*rest.Config{
			tpClusterName: cfg.Contexts[cfg.CurrentContext],
		}
	}

	res := make(map[string]*kubeCreds, len(cfg.Contexts))
	// Convert kubeconfig contexts into kubeCreds.
	for cluster, clientCfg := range cfg.Contexts {
		log := log.WithField("cluster", cluster)
		log.Debug("Checking kubernetes impersonation permissions.")
		client, err := kubernetes.NewForConfig(clientCfg)
		if err != nil {
			return nil, trace.Wrap(err, "failed to generate kubernetes client for cluster %q", cluster)
		}
		// For each loaded cluster, check impersonation permissions. This
		// failure is only critical for newKubeService.
		if err := checkImpersonationPermissions(ctx, client.AuthorizationV1().SelfSubjectAccessReviews()); err != nil {
			// kubernetes_service must have valid RBAC permissions, otherwise
			// it's pointless.
			// proxy_service can run without them (e.g. a root proxy).
			if newKubeService {
				return nil, trace.Wrap(err)
			}
			log.WithError(err).Warning("Failed to test the necessary kubernetes permissions. This teleport instance will still handle kubernetes requests towards other kubernetes clusters")
			// We used to recommend users to set a dummy kubeconfig on root
			// proxies to get kubernetes support working for leaf clusters:
			// https://community.gravitational.com/t/enabling-teleport-to-act-as-a-kubernetes-proxy-for-trusted-leaf-clusters/418
			//
			// Since this is no longer necessary, recommend them to clean up
			// via logs.
			if kubeconfigPath != "" {
				log.Info("If this is a proxy and you provided a dummy kubeconfig_path, you can remove it from teleport.yaml to get rid of this warning")
			}
		} else {
			log.Debug("Have all necessary kubernetes impersonation permissions.")
		}

		targetAddr, err := parseKubeHost(clientCfg.Host)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConfig, err := rest.TLSConfigFor(clientCfg)
		if err != nil {
			return nil, trace.Wrap(err, "failed to generate TLS config from kubeconfig: %v", err)
		}
		transportConfig, err := clientCfg.TransportConfig()
		if err != nil {
			return nil, trace.Wrap(err, "failed to generate transport config from kubeconfig: %v", err)
		}

		log.Debug("Initialized kubernetes credentials")
		res[cluster] = &kubeCreds{
			tlsConfig:       tlsConfig,
			transportConfig: transportConfig,
			targetAddr:      targetAddr,
			kubeClient:      client,
		}
	}
	return res, nil
}

// parseKubeHost parses and formats kubernetes hostname
// to host:port format, if no port it set,
// it assumes default HTTPS port
func parseKubeHost(host string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", trace.Wrap(err, "failed to parse kubernetes host: %v", err)
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

func checkImpersonationPermissions(ctx context.Context, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
	if skipSelfPermissionCheck {
		return nil
	}

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
			return trace.Wrap(err, "failed to verify impersonation permissions for kubernetes: %v; this may be due to missing the SelfSubjectAccessReview permission on the ClusterRole used by the proxy; please make sure that proxy has all the necessary permissions: https://gravitational.com/teleport/docs/kubernetes_ssh/#impersonation", err)
		}
		if !resp.Status.Allowed {
			return trace.AccessDenied("proxy can't impersonate kubernetes %s at the cluster level; please make sure that proxy has all the necessary permissions: https://gravitational.com/teleport/docs/kubernetes_ssh/#impersonation", resource)
		}
	}
	return nil
}
