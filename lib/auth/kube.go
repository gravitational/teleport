/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/authority"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
)

// KubeCSR is a kubernetes CSR request
type KubeCSR struct {
	// Username of user's certificate
	Username string `json:"username"`
	// ClusterName is a name of the target cluster to generate certificate for
	ClusterName string `json:"cluster_name"`
	// CSR is a kubernetes CSR
	CSR []byte `json:"csr"`
}

// CheckAndSetDefaults checks and sets defaults
func (a *KubeCSR) CheckAndSetDefaults() error {
	if len(a.CSR) == 0 {
		return trace.BadParameter("missing parameter 'csr'")
	}
	return nil
}

// KubeCSRREsponse is a response to kubernetes CSR request
type KubeCSRResponse struct {
	// Cert is a signed certificate PEM block
	Cert []byte `json:"cert"`
	// CertAuthorities is a list of PEM block with trusted cert authorities
	CertAuthorities [][]byte `json:"cert_authorities"`
	// TargetAddr is an optional target address
	// of the kubernetes API server that can be set
	// in the kubeconfig
	TargetAddr string `json:"target_addr"`
}

type kubeCreds struct {
	// clt is a working kubernetes client
	clt *kubernetes.Clientset
	// caPEM is a PEM encoded certificate authority
	// of the kubernetes API server
	caPEM []byte
	// targetAddr is a target address of the
	// kubernetes cluster read from config
	targetAddr string
}

// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
// signed certificate if sucessful.
func (s *AuthServer) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	if !modules.GetModules().SupportsKubernetes() {
		return nil, trace.AccessDenied(
			"this teleport cluster does not support kubernetes, please contact system administrator for support")
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.ClusterName == s.clusterName.GetClusterName() {
		log.Debugf("Generating certificate for local Kubernetes cluster.")

		kubeCreds, err := s.getKubeClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cert, err := authority.ProcessCSR(kubeCreds.clt, req.CSR)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &KubeCSRResponse{
			Cert:            cert,
			CertAuthorities: [][]byte{kubeCreds.caPEM},
			TargetAddr:      kubeCreds.targetAddr,
		}, nil
	}

	// Certificate for remote cluster is a user certificate
	// with special provisions.
	log.Debugf("Generating certificate for remote Kubernetes cluster.")

	hostCA, err := s.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: req.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := s.GetUser(req.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := services.FetchRoles(user.GetRoles(), s, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ttl := roles.AdjustSessionTTL(defaults.CertDuration)

	// extract and encode the kubernetes groups of the authenticated
	// user in the newly issued certificate
	kubernetesGroups, err := roles.CheckKubeGroups(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userCA, err := s.Trust.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: s.clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate TLS certificate
	tlsAuthority, err := userCA.TLSCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity := tlsca.Identity{
		Username: user.GetName(),
		Groups:   roles.RoleNames(),
		// Generate a certificate restricted for
		// use against a kubernetes endpoint, and not the API server endpoint
		// otherwise proxies can generate certs for any user.
		Usage:            []string{teleport.UsageKubeOnly},
		KubernetesGroups: kubernetesGroups,
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: csr.PublicKey,
		Subject:   identity.Subject(),
		NotAfter:  s.clock.Now().UTC().Add(ttl),
	}
	tlsCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	re := &KubeCSRResponse{Cert: tlsCert}
	for _, keyPair := range hostCA.GetTLSKeyPairs() {
		re.CertAuthorities = append(re.CertAuthorities, keyPair.Cert)
	}
	return re, nil
}

func (s *AuthServer) getKubeClient() (*kubeCreds, error) {
	// no kubeconfig is set, assume auth server is running in the cluster
	if s.kubeconfigPath == "" {
		caPEM, err := ioutil.ReadFile(teleport.KubeCAPath)
		if err != nil {
			return nil, trace.BadParameter(`auth server assumed that it is 
running in a kubernetes cluster, but %v mounted in pods could not be read: %v, 
set kubeconfig_path if auth server is running outside of the cluster`, teleport.KubeCAPath, err)
		}

		clt, cfg, err := kubeutils.GetKubeClient(os.Getenv(teleport.EnvKubeConfig))
		if err != nil {
			return nil, trace.BadParameter(`auth server assumed that it is 
running in a kubernetes cluster, but could not init in-cluster kubernetes client: %v`, err)
		}

		targetAddr, err := parseKubeHost(cfg.Host)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse kubernetes host")
		}

		return &kubeCreds{
			clt:        clt,
			caPEM:      caPEM,
			targetAddr: targetAddr,
		}, nil
	}

	log.Debugf("Reading configuration from kubeconfig file %v.", s.kubeconfigPath)

	clt, cfg, err := kubeutils.GetKubeClient(s.kubeconfigPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetAddr, err := parseKubeHost(cfg.Host)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse kubernetes host")
	}

	var caPEM []byte
	if len(cfg.CAData) == 0 {
		if cfg.CAFile == "" {
			return nil, trace.BadParameter("can't find trusted certificates in %v", s.kubeconfigPath)
		}
		caPEM, err = ioutil.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, trace.BadParameter("failed to read trusted certificates from %v: %v", cfg.CAFile, err)
		}
	} else {
		caPEM = cfg.CAData
	}

	return &kubeCreds{
		clt:        clt,
		caPEM:      caPEM,
		targetAddr: targetAddr,
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
