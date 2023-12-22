/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
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
func (a *Server) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	ctx := context.TODO()
	if err := enforceLicense(types.KindKubernetesCluster); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Certificate for remote cluster is a user certificate
	// with special provisions.
	log.Debugf("Generating certificate to access remote Kubernetes clusters.")

	hostCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
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
		ca, err := a.GetCertAuthority(ctx, types.CertAuthID{Type: types.UserCA, DomainName: id.TeleportCluster}, false)
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
	roles, err := services.FetchRoles(roleNames, a, id.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Get the correct cert TTL based on roles.
	ttl := roles.AdjustSessionTTL(apidefaults.CertDuration)

	userCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate TLS certificate
	cert, signer, err := a.GetKeyStore().GetTLSCertAndSigner(ctx, userCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certRequest := tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: csr.PublicKey,
		// Always trust the Subject sent by the proxy (minus the Usage field).
		// A user may have received temporary extra roles via workflow API, we
		// must preserve those. The storage backend doesn't record temporary
		// granted roles.
		Subject:  subject,
		NotAfter: a.clock.Now().UTC().Add(ttl),
	}
	tlsCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &KubeCSRResponse{
		Cert:            tlsCert,
		CertAuthorities: services.GetTLSCerts(hostCA),
	}, nil
}
