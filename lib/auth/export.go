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

package auth

import (
	"context"
	"encoding/pem"
	"strings"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
)

// ExportAuthoritiesRequest has the required fields to create an export authorities request.
// An empty AuthType exports all types.
type ExportAuthoritiesRequest struct {
	AuthType                   string
	Client                     ClientI
	ExportAuthorityFingerprint string
	ExportPrivateKeys          bool
	CompatVersion              string
}

// ExportAuthorities returns the list of authorities in OpenSSH compatible formats
// If the ExportAuthoritiesRequest.AuthType is present only prints keys for CAs of this type,
// otherwise returns all keys concatenated
func ExportAuthorities(ctx context.Context, req ExportAuthoritiesRequest) (string, error) {
	ret := strings.Builder{}

	var typesToExport []types.CertAuthType

	// this means to export TLS authority
	switch req.AuthType {
	// "tls" is supported for backwards compatibility.
	// "tls-host" and "tls-user" were added later to allow export of the user
	// TLS CA.
	case "tls", "tls-host":
		return exportTLSAuthority(ctx, req.Client, types.HostCA, false, req.ExportPrivateKeys)
	case "tls-user":
		return exportTLSAuthority(ctx, req.Client, types.UserCA, false, req.ExportPrivateKeys)
	case "db":
		return exportTLSAuthority(ctx, req.Client, types.DatabaseCA, false, req.ExportPrivateKeys)
	case "tls-user-der", "windows":
		return exportTLSAuthority(ctx, req.Client, types.UserCA, true, req.ExportPrivateKeys)
	}

	// if no --type flag is given, export HostCA and UserCA.
	if req.AuthType == "" {
		typesToExport = []types.CertAuthType{types.HostCA, types.UserCA}
	} else {
		authType := types.CertAuthType(req.AuthType)
		if err := authType.Check(); err != nil {
			return "", trace.Wrap(err)
		}
		typesToExport = []types.CertAuthType{authType}
	}
	localAuthName, err := req.Client.GetDomainName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []types.CertAuthority
	for _, at := range typesToExport {
		cas, err := req.Client.GetCertAuthorities(ctx, at, req.ExportPrivateKeys)
		if err != nil {
			return "", trace.Wrap(err)
		}
		for _, ca := range cas {
			if ca.GetClusterName() == localAuthName {
				authorities = append(authorities, ca)
			}
		}
	}

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
				ret.WriteString(string(key.PrivateKey) + "\n")
			}
		} else {
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
					castr, err := hostCAFormat(ca, key.PublicKey, req.Client)
					if err != nil {
						return "", trace.Wrap(err)
					}

					ret.WriteString(castr + "\n")
					continue
				}

				// export certificate authority in user or host ca format
				var castr string
				switch ca.GetType() {
				case types.UserCA:
					castr, err = userCAFormat(ca, key.PublicKey)
				case types.HostCA:
					castr, err = hostCAFormat(ca, key.PublicKey, req.Client)
				default:
					return "", trace.BadParameter("unknown user type: %q", ca.GetType())
				}
				if err != nil {
					return "", trace.Wrap(err)
				}

				// write the export friendly string
				ret.WriteString(castr + "\n")
			}
		}
	}

	return ret.String(), nil
}

func exportTLSAuthority(ctx context.Context, client ClientI, typ types.CertAuthType, unpackPEM bool, exportPrivateKeys bool) (string, error) {
	ret := strings.Builder{}

	clusterName, err := client.GetDomainName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	certAuthority, err := client.GetCertAuthority(
		ctx,
		types.CertAuthID{Type: typ, DomainName: clusterName},
		exportPrivateKeys,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(certAuthority.GetActiveKeys().TLS) != 1 {
		return "", trace.BadParameter("expected one TLS key pair, got %v", len(certAuthority.GetActiveKeys().TLS))
	}
	keyPair := certAuthority.GetActiveKeys().TLS[0]

	marhsalKeyPair := func(data []byte) error {
		if !unpackPEM {
			ret.WriteString(string(data) + "\n")
			return nil
		}

		b, _ := pem.Decode(data)
		if b == nil {
			return trace.BadParameter("no PEM data in CA data: %q", data)
		}
		ret.WriteString(string(b.Bytes) + "\n")

		return nil
	}

	if exportPrivateKeys {
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
//    cert-authority AAA... type=user&clustername=cluster-a
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
//    @cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca types.CertAuthority, keyBytes []byte, client ClientI) (string, error) {
	roles, err := services.FetchRoles(ca.GetRoles(), client, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.GetLoginsForTTL(apidefaults.MinCertDuration + time.Second)
	return sshutils.MarshalAuthorizedHostsFormat(ca.GetClusterName(), keyBytes, allowedLogins)
}
