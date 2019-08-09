/*
Copyright 2018-2019 Gravitational, Inc.

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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
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

// KubeCSRResponse is a response to kubernetes CSR request
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

	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Certificate for remote cluster is a user certificate
	// with special provisions.
	log.Debugf("Generating certificate to access remote Kubernetes clusters.")

	hostCA, err := s.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: req.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := s.GetUser(req.Username, false)
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
		DomainName: clusterName.GetClusterName(),
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
