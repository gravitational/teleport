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
	log "github.com/sirupsen/logrus"
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

func getKubeCreds(kubeconfigPath string) (*kubeCreds, error) {
	var cfg *rest.Config
	// no kubeconfig is set, assume auth server is running in the cluster
	if kubeconfigPath == "" {
		caPEM, err := ioutil.ReadFile(teleport.KubeCAPath)
		if err != nil {
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

	targetAddr, err := parseKubeHost(cfg.Host)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse kubernetes host")
	}
	tlsConfig, err := rest.TLSConfigFor(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate TLS config from kubeconfig")
	}
	transportConfig, err := cfg.TransportConfig()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate transport config from kubeconfig")
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
		return "", trace.Wrap(err, "failed to parse kubernetes host")
	}
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		// add default HTTPS port
		return fmt.Sprintf("%v:443", u.Host), nil
	}
	return u.Host, nil
}

func (c *kubeCreds) wrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	return transport.HTTPWrappersForConfig(c.transportConfig, rt)
}
