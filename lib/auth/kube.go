/*
Copyright 2018-2021 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
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
// signed certificate if successful.
func (s *Server) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	if !modules.GetModules().Features().Kubernetes {
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

	hostCA, err := s.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: req.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract identity from the CSR. Pass zero time for id.Expiry, it won't be
	// used here.
	id, err := tlsca.FromSubject(csr.Subject, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Enforce only k8s usage on generated cert, keep all other fields.
	id.Usage = []string{teleport.UsageKubeOnly}
	// Re-encode the identity to subject, with updated Usage.
	subject, err := id.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roleNames := id.Groups
	// This is a remote user, map roles to local roles first.
	if id.TeleportCluster != clusterName.GetClusterName() {
		ca, err := s.GetCertAuthority(types.CertAuthID{Type: types.UserCA, DomainName: id.TeleportCluster}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleNames, err = services.MapRoles(ca.CombinedMapping(), id.Groups)
		if err != nil {
			return nil, trace.AccessDenied("failed to map roles for remote user %q from cluster %q with remote roles %v", id.Username, id.TeleportCluster, id.Groups)
		}
		if len(roleNames) == 0 {
			return nil, trace.AccessDenied("no roles mapped for remote user %q from cluster %q with remote roles %v", id.Username, id.TeleportCluster, id.Groups)
		}
	}

	// Extract user roles from the identity (from the CSR Subject).
	roles, err := services.FetchRoles(roleNames, s, id.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Get the correct cert TTL based on roles.
	ttl := roles.AdjustSessionTTL(defaults.CertDuration)

	userCA, err := s.Trust.GetCertAuthority(types.CertAuthID{
		Type:       types.UserCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate TLS certificate
	tlsAuthority, err := tlsca.FromAuthority(userCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certRequest := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: csr.PublicKey,
		// Always trust the Subject sent by the proxy (minus the Usage field).
		// A user may have received temporary extra roles via workflow API, we
		// must preserve those. The storage backend doesn't record temporary
		// granted roles.
		Subject:  subject,
		NotAfter: s.clock.Now().UTC().Add(ttl),
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
