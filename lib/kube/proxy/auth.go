package proxy

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/gravitational/teleport"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	authzapi "k8s.io/api/authorization/v1"
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
type kubeCreds struct {
	// tlsConfig contains (m)TLS configuration.
	tlsConfig *tls.Config
	// transportConfig contains HTTPS-related configuration.
	// Note: use wrapTransport method if working with http.RoundTrippers.
	transportConfig *transport.Config
	// targetAddr is a kubernetes API address.
	targetAddr string
}

func getKubeCreds(log logrus.FieldLogger, kubeconfigPath string) (*kubeCreds, error) {
	var cfg *rest.Config
	// no kubeconfig is set, assume auth server is running in the cluster
	if kubeconfigPath == "" {
		caPEM, err := ioutil.ReadFile(teleport.KubeCAPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Debugf("kubeconfig_file was not provided in the config and %q doesn't exist; this proxy will still be able to forward requests to trusted leaf Teleport clusters, but not to a Kubernetes cluster directly", teleport.KubeCAPath)
				return nil, nil
			}
			return nil, trace.BadParameter(`auth server assumed that it is
running in a kubernetes cluster, but %v mounted in pods could not be read: %v,
set kubeconfig_file if auth server is running outside of the cluster`, teleport.KubeCAPath, err)
		}

		cfg, err = kubeutils.GetKubeConfig(os.Getenv(teleport.EnvKubeConfig))
		if err != nil {
			return nil, trace.BadParameter(`auth server assumed that it is
running in a kubernetes cluster, but could not init in-cluster kubernetes client: %v`, err)
		}
		cfg.CAData = caPEM
	} else {
		log.Debugf("Reading configuration from kubeconfig file %v.", kubeconfigPath)

		var err error
		cfg, err = kubeutils.GetKubeConfig(kubeconfigPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	log.Debug("Checking kubernetes impersonation permissions granted to proxy.")
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate kubernetes client from kubeconfig: %v", err)
	}
	if err := checkImpersonationPermissions(client.AuthorizationV1().SelfSubjectAccessReviews()); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Proxy has all necessary kubernetes impersonation permissions.")

	targetAddr, err := parseKubeHost(cfg.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := rest.TLSConfigFor(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate TLS config from kubeconfig: %v", err)
	}
	transportConfig, err := cfg.TransportConfig()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate transport config from kubeconfig: %v", err)
	}

	return &kubeCreds{
		tlsConfig:       tlsConfig,
		transportConfig: transportConfig,
		targetAddr:      targetAddr,
	}, nil
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

func checkImpersonationPermissions(sarClient authztypes.SelfSubjectAccessReviewInterface) error {
	for _, resource := range []string{"users", "groups", "serviceaccounts"} {
		resp, err := sarClient.Create(&authzapi.SelfSubjectAccessReview{
			Spec: authzapi.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authzapi.ResourceAttributes{
					Verb:     "impersonate",
					Resource: resource,
				},
			},
		})
		if err != nil {
			return trace.Wrap(err, "failed to verify impersonation permissions for kubernetes: %v; this may be due to missing the SelfSubjectAccessReview permission on the ClusterRole used by the proxy; please make sure that proxy has all the necessary permissions: https://gravitational.com/teleport/docs/kubernetes_ssh/#impersonation", err)
		}
		if !resp.Status.Allowed {
			return trace.AccessDenied("proxy can't impersonate kubernetes %s at the cluster level; please make sure that proxy has all the necessary permissions: https://gravitational.com/teleport/docs/kubernetes_ssh/#impersonation", resource)
		}
	}
	return nil
}
