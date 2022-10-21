/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"context"
	"encoding/pem"
	"strings"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
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
// To do so, set CompatVersion to "1.0".
// No other CompatVersion value is accepted.
type ExportAuthoritiesRequest struct {
	AuthType                   string
	ExportAuthorityFingerprint string
	ExportPrivateKeys          bool
	CompatVersion              string
}

// ExportAuthorities returns the list of authorities in OpenSSH compatible formats as a string.
// If the ExportAuthoritiesRequest.AuthType is present only prints keys for CAs of this type,
// otherwise returns all keys concatenated.
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
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
// > @cert-authority *.cluster-a ssh-rsa AAA... type=host
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func ExportAuthorities(ctx context.Context, client auth.ClientI, req ExportAuthoritiesRequest) (string, error) {
	var typesToExport []types.CertAuthType

	// this means to export TLS authority
	switch req.AuthType {
	// "tls" is supported for backwards compatibility.
	// "tls-host" and "tls-user" were added later to allow export of the user
	// TLS CA.
	case "tls", "tls-host":
		req := exportTLSAuthorityRequest{
			AuthType:          types.HostCA,
			UnpackPEM:         false,
			ExportPrivateKeys: req.ExportPrivateKeys,
		}
		return exportTLSAuthority(ctx, client, req)
	case "tls-user":
		req := exportTLSAuthorityRequest{
			AuthType:          types.UserCA,
			UnpackPEM:         false,
			ExportPrivateKeys: req.ExportPrivateKeys,
		}
		return exportTLSAuthority(ctx, client, req)
	case "db":
		req := exportTLSAuthorityRequest{
			AuthType:          types.DatabaseCA,
			UnpackPEM:         false,
			ExportPrivateKeys: req.ExportPrivateKeys,
		}
		return exportTLSAuthority(ctx, client, req)
	case "tls-user-der", "windows":
		req := exportTLSAuthorityRequest{
			AuthType:          types.UserCA,
			UnpackPEM:         true,
			ExportPrivateKeys: req.ExportPrivateKeys,
		}
		return exportTLSAuthority(ctx, client, req)
	}

	// If no AuthType is given, export both HostCA and UserCA.
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
		cas, err := client.GetCertAuthorities(ctx, at, req.ExportPrivateKeys)
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
		if req.ExportPrivateKeys {
			for _, key := range ca.GetActiveKeys().SSH {
				fingerprint, err := sshutils.PrivateKeyFingerprint(key.PrivateKey)
				if err != nil {
					return "", trace.Wrap(err)
				}
				if req.ExportAuthorityFingerprint != "" && fingerprint != req.ExportAuthorityFingerprint {
					continue
				}
				ret.WriteString(string(key.PrivateKey))
				ret.WriteString("\n")
			}
			continue
		}

		for _, key := range ca.GetTrustedSSHKeyPairs() {
			fingerprint, err := sshutils.AuthorizedKeyFingerprint(key.PublicKey)
			if err != nil {
				return "", trace.Wrap(err)
			}
			if req.ExportAuthorityFingerprint != "" && fingerprint != req.ExportAuthorityFingerprint {
				continue
			}

			// export certificates in the old 1.0 format where host and user
			// certificate authorities were exported in the known_hosts format.
			if req.CompatVersion == "1.0" {
				castr, err := hostCAFormat(ca, key.PublicKey, client)
				if err != nil {
					return "", trace.Wrap(err)
				}

				ret.WriteString(castr)
				ret.WriteString("\n")
				continue
			}

			// export certificate authority in user or host ca format
			var castr string
			switch ca.GetType() {
			case types.UserCA:
				castr, err = userCAFormat(ca, key.PublicKey)
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
			ret.WriteString("\n")
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

	if len(certAuthority.GetActiveKeys().TLS) != 1 {
		return "", trace.BadParameter("expected one TLS key pair, got %v", len(certAuthority.GetActiveKeys().TLS))
	}
	keyPair := certAuthority.GetActiveKeys().TLS[0]

	ret := strings.Builder{}
	marhsalKeyPair := func(data []byte) error {
		if !req.UnpackPEM {
			ret.WriteString(string(data))
			ret.WriteString("\n")
			return nil
		}

		b, _ := pem.Decode(data)
		if b == nil {
			return trace.BadParameter("no PEM data in CA data: %q", data)
		}
		ret.WriteString(string(b.Bytes))
		ret.WriteString("\n")

		return nil
	}

	if req.ExportPrivateKeys {
		if err := marhsalKeyPair(keyPair.Key); err != nil {
			return "", trace.Wrap(err)
		}
	}

	if err := marhsalKeyPair(keyPair.Cert); err != nil {
		return "", trace.Wrap(err)
	}

	return ret.String(), nil
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
func userCAFormat(ca types.CertAuthority, keyBytes []byte) (string, error) {
	return sshutils.MarshalAuthorizedKeysFormat(ca.GetClusterName(), keyBytes)
}

// hostCAFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//	@cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca types.CertAuthority, keyBytes []byte, client auth.ClientI) (string, error) {
	roles, err := services.FetchRoles(ca.GetRoles(), client, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.GetLoginsForTTL(apidefaults.MinCertDuration + time.Second)
	return sshutils.MarshalAuthorizedHostsFormat(ca.GetClusterName(), keyBytes, allowedLogins)
}
