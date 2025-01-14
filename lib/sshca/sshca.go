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

// Package sshca specifies interfaces for SSH certificate authorities
package sshca

import (
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	apidefaults "github.com/gravitational/teleport/api/defaults"
)

// Authority implements minimal key-management facility for generating OpenSSH
// compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	// GenerateHostCert takes the private key of the CA, public key of the new host,
	// along with metadata (host ID, node name, cluster name, roles, and ttl) and generates
	// a host certificate.
	GenerateHostCert(HostCertificateRequest) ([]byte, error)

	// GenerateUserCert generates user ssh certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(UserCertificateRequest) ([]byte, error)
}

// HostCertificateRequest is a request to generate a new ssh host certificate.
type HostCertificateRequest struct {
	// CASigner is the signer that will sign the public key of the host with the CA private key
	CASigner ssh.Signer
	// PublicHostKey is the public key of the host
	PublicHostKey []byte
	// HostID is used by Teleport to uniquely identify a node within a cluster (this is used to help infill
	// Identity.Princiapals and is not a standalone cert field).
	HostID string
	// NodeName is the DNS name of the node (this is used to help infill Identity.Princiapals and is not a
	// standalone cert field).
	NodeName string
	// TTL defines how long a certificate is valid for
	TTL time.Duration
	// Identity is the host identity to be encoded in the certificate.
	Identity Identity
}

func (r *HostCertificateRequest) Check() error {
	if r.CASigner == nil {
		return trace.BadParameter("ssh host certificate request missing ca signer")
	}
	if r.HostID == "" && len(r.Identity.Principals) == 0 {
		return trace.BadParameter("ssh host certificate request missing host ID and principals")
	}
	if r.Identity.ClusterName == "" {
		return trace.BadParameter("ssh host certificate request missing cluster name")
	}
	if r.Identity.ValidBefore != 0 {
		return trace.BadParameter("ValidBefore should not be set in host cert requests (derived from TTL)")
	}
	if r.Identity.ValidAfter != 0 {
		return trace.BadParameter("ValidAfter should not be set in host cert requests (derived from TTL)")
	}
	if err := r.Identity.SystemRole.Check(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UserCertificateRequest is a request to generate a new ssh user certificate.
type UserCertificateRequest struct {
	// CASigner is the signer that will sign the public key of the user with the CA private key
	CASigner ssh.Signer
	// PublicUserKey is the public key of the user in SSH authorized_keys format.
	PublicUserKey []byte
	// TTL defines how long a certificate is valid for (if specified, ValidAfter/ValidBefore within the
	// identity must not be set).
	TTL time.Duration
	// CertificateFormat is the format of the SSH certificate.
	CertificateFormat string
	// Identity is the user identity to be encoded in the certificate.
	Identity Identity
}

func (r *UserCertificateRequest) CheckAndSetDefaults() error {
	if r.CASigner == nil {
		return trace.BadParameter("ssh user certificate request missing ca signer")
	}
	if r.TTL < apidefaults.MinCertDuration {
		r.TTL = apidefaults.MinCertDuration
	}
	if len(r.Identity.Principals) == 0 {
		return trace.BadParameter("ssh user identity missing allowed logins")
	}
	if r.Identity.ValidBefore != 0 {
		return trace.BadParameter("ValidBefore should not be set in user cert requests (derived from TTL)")
	}
	if r.Identity.ValidAfter != 0 {
		return trace.BadParameter("ValidAfter should not be set in user cert requests (derived from TTL)")
	}

	return nil
}
