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

package client

import (
	"context"
	"encoding/pem"
	"errors"
	"strings"
	"time"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
)

// ExportAuthoritiesRequest has the required fields to create an export authorities request.
//
// An empty AuthType exports all types.
//
// When exporting private keys, you can set ExportAuthorityFingerprint to filter the authority.
// Fingerprint must be the SHA256 of the Authority's public key.
//
// You can export using the old 1.0 format where host and user
// certificate authorities were exported in the known_hosts format.
// To do so, set UseCompatVersion to true.
type ExportAuthoritiesRequest struct {
	AuthType                   string
	ExportAuthorityFingerprint string
	UseCompatVersion           bool
}

// ExportAuthorities returns the list of authorities in OpenSSH compatible formats as a string.
// If the ExportAuthoritiesRequest.AuthType is present only prints keys for CAs of this type,
// otherwise returns host and user SSH keys.
//
// Exporting using "tls*", "database", "windows" AuthType:
// Returns the certificate authority public key to be used by systems that rely on TLS.
// The format can be PEM or DER depending on the target.
//
// Exporting using "user" AuthType:
// Returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
// > cert-authority AAA... type=user&clustername=cluster-a
// URL encoding is used to pass the CA type and cluster name into the comment field.
//
// Exporting using "host" AuthType:
// Returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/known_hosts. The format adheres to the man sshd (8)
// known_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
// > @cert-authority *.cluster-a ssh-rsa AAA... type=host
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func ExportAuthorities(ctx context.Context, client auth.ClientI, req ExportAuthoritiesRequest) (string, error) {
	return exportAuth(ctx, client, req, false /* exportSecrets */)
}

// ExportAuthoritiesSecrets exports the Authority Certificate secrets (private keys).
// See ExportAuthorities for more information.
func ExportAuthoritiesSecrets(ctx context.Context, client auth.ClientI, req ExportAuthoritiesRequest) (string, error) {
	return exportAuth(ctx, client, req, true /* exportSecrets */)
}

func exportAuth(ctx context.Context, client auth.ClientI, req ExportAuthoritiesRequest, exportSecrets bool) (string, error) {
	var typesToExport []types.CertAuthType

	if exportSecrets {
		mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
		if err == nil {
			ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
		} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
			return "", trace.Wrap(err)
		}
	}

	// this means to export TLS authority
	switch req.AuthType {
	// "tls" is supported for backwards compatibility.
	// "tls-host" and "tls-user" were added later to allow export of the user
	// TLS CA.
	case "tls", "tls-host":
		req := exportTLSAuthorityRequest{
			AuthType:          types.HostCA,
			UnpackPEM:         false,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "tls-user":
		req := exportTLSAuthorityRequest{
			AuthType:          types.UserCA,
			UnpackPEM:         false,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "tls-spiffe":
		req := exportTLSAuthorityRequest{
			AuthType:          types.SPIFFECA,
			UnpackPEM:         false,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "db":
		req := exportTLSAuthorityRequest{
			AuthType:          types.DatabaseCA,
			UnpackPEM:         false,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "db-der":
		req := exportTLSAuthorityRequest{
			AuthType:          types.DatabaseCA,
			UnpackPEM:         true,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "db-client":
		req := exportTLSAuthorityRequest{
			AuthType:          types.DatabaseClientCA,
			UnpackPEM:         false,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "db-client-der":
		req := exportTLSAuthorityRequest{
			AuthType:          types.DatabaseClientCA,
			UnpackPEM:         true,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "tls-user-der", "windows":
		req := exportTLSAuthorityRequest{
			AuthType:          types.UserCA,
			UnpackPEM:         true,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	case "saml-idp":
		req := exportTLSAuthorityRequest{
			AuthType:          types.SAMLIDPCA,
			UnpackPEM:         true,
			ExportPrivateKeys: exportSecrets,
		}
		return exportTLSAuthority(ctx, client, req)
	}

	// If none of the above auth-types was requested, means we are dealing with SSH HostCA or SSH UserCA.
	// Either for adding SSH known hosts (~/.ssh/known_hosts) or authorized keys (`~/.ssh/authorized_keys`).
	// Both are exported if AuthType is empty.
	if req.AuthType == "" {
		typesToExport = []types.CertAuthType{types.HostCA, types.UserCA}
	} else {
		authType := types.CertAuthType(req.AuthType)
		if err := authType.Check(); err != nil {
			return "", trace.Wrap(err)
		}
		typesToExport = []types.CertAuthType{authType}
	}
	localAuthName, err := client.GetDomainName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []types.CertAuthority
	for _, at := range typesToExport {
		cas, err := client.GetCertAuthorities(ctx, at, exportSecrets)
		if err != nil {
			return "", trace.Wrap(err)
		}
		for _, ca := range cas {
			if ca.GetClusterName() == localAuthName {
				authorities = append(authorities, ca)
			}
		}
	}

	ret := strings.Builder{}
	for _, ca := range authorities {
		if exportSecrets {
			for _, key := range ca.GetActiveKeys().SSH {
				if req.ExportAuthorityFingerprint != "" {
					fingerprint, err := sshutils.PrivateKeyFingerprint(key.PrivateKey)
					if err != nil {
						return "", trace.Wrap(err)
					}

					if fingerprint != req.ExportAuthorityFingerprint {
						continue
					}
				}

				ret.Write(key.PrivateKey)
				ret.WriteString("\n")
			}
			continue
		}

		for _, key := range ca.GetTrustedSSHKeyPairs() {
			if req.ExportAuthorityFingerprint != "" {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(key.PublicKey)
				if err != nil {
					return "", trace.Wrap(err)
				}

				if fingerprint != req.ExportAuthorityFingerprint {
					continue
				}
			}

			// export certificates in the old 1.0 format where host and user
			// certificate authorities were exported in the known_hosts format.
			if req.UseCompatVersion {
				castr, err := hostCAFormat(ca, key.PublicKey, client)
				if err != nil {
					return "", trace.Wrap(err)
				}

				ret.WriteString(castr)
				continue
			}

			// export certificate authority in user or host ca format
			var castr string
			switch ca.GetType() {
			case types.UserCA, types.OpenSSHCA:
				castr, err = userOrOpenSSHCAFormat(ca, key.PublicKey)
			case types.HostCA:
				castr, err = hostCAFormat(ca, key.PublicKey, client)
			default:
				return "", trace.BadParameter("unknown user type: %q", ca.GetType())
			}
			if err != nil {
				return "", trace.Wrap(err)
			}

			// write the export friendly string
			ret.WriteString(castr)
		}
	}

	return ret.String(), nil
}

type exportTLSAuthorityRequest struct {
	AuthType          types.CertAuthType
	UnpackPEM         bool
	ExportPrivateKeys bool
}

func exportTLSAuthority(ctx context.Context, client auth.ClientI, req exportTLSAuthorityRequest) (string, error) {
	clusterName, err := client.GetDomainName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	certAuthority, err := client.GetCertAuthority(
		ctx,
		types.CertAuthID{Type: req.AuthType, DomainName: clusterName},
		req.ExportPrivateKeys,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if l := len(certAuthority.GetActiveKeys().TLS); l != 1 {
		return "", trace.BadParameter("expected one TLS key pair, got %v", l)
	}
	keyPair := certAuthority.GetActiveKeys().TLS[0]

	bytesToExport := keyPair.Cert
	if req.ExportPrivateKeys {
		bytesToExport = keyPair.Key
	}

	if !req.UnpackPEM {
		return string(bytesToExport), nil
	}

	b, _ := pem.Decode(bytesToExport)
	if b == nil {
		return "", trace.BadParameter("invalid PEM data")
	}

	return string(b.Bytes), nil
}

// userCAFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//	cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func userOrOpenSSHCAFormat(ca types.CertAuthority, keyBytes []byte) (string, error) {
	return sshutils.MarshalAuthorizedKeysFormat(ca.GetClusterName(), keyBytes)
}

// hostCAFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/known_hosts. The format adheres to the man sshd (8)
// known_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//	@cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca types.CertAuthority, keyBytes []byte, client auth.ClientI) (string, error) {
	roles, err := services.FetchRoles(ca.GetRoles(), client, nil /* traits */)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.GetLoginsForTTL(apidefaults.MinCertDuration + time.Second)
	return sshutils.MarshalKnownHost(sshutils.KnownHost{
		Hostname:      ca.GetClusterName(),
		AuthorizedKey: keyBytes,
		Comment: map[string][]string{
			"logins": allowedLogins,
		},
	})
}
