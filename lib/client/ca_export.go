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
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
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

// ExportedAuthority represents an exported authority certificate, as returned
// by [ExportAllAuthorities] or [ExportAllAuthoritiesSecrets].
type ExportedAuthority struct {
	// Data is the output of the exported authority.
	// May be an SSH authorized key, an SSH known hosts entry, a DER or a PEM,
	// depending on the type of the exported authority.
	Data []byte
}

// ExportAllAuthorities exports public keys of all authorities of a particular
// type. The export format depends on the authority type, see below for
// details.
//
// An empty ExportAuthoritiesRequest.AuthType is interpreted as an export for
// host and user SSH keys.
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
//
// At least one authority is guaranteed on success.
func ExportAllAuthorities(ctx context.Context, client authclient.ClientI, req ExportAuthoritiesRequest) ([]*ExportedAuthority, error) {
	const exportSecrets = false
	return exportAllAuthorities(ctx, client, req, exportSecrets)
}

// ExportAllAuthoritiesSecrets exports private keys of all authorities of a
// particular type.
// See [ExportAllAuthorities] for more information.
//
// At least one authority is guaranteed on success.
func ExportAllAuthoritiesSecrets(ctx context.Context, client authclient.ClientI, req ExportAuthoritiesRequest) ([]*ExportedAuthority, error) {
	const exportSecrets = true
	return exportAllAuthorities(ctx, client, req, exportSecrets)
}

func exportAllAuthorities(
	ctx context.Context,
	client authclient.ClientI,
	req ExportAuthoritiesRequest,
	exportSecrets bool,
) ([]*ExportedAuthority, error) {
	authorities, err := exportAuth(ctx, client, req, exportSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Sanity check that we have at least one authority.
	// Not expected to happen in practice.
	if len(authorities) == 0 {
		return nil, trace.BadParameter("export returned zero authorities")
	}

	return authorities, nil
}

func exportAuth(ctx context.Context, client authclient.ClientI, req ExportAuthoritiesRequest, exportSecrets bool) ([]*ExportedAuthority, error) {
	var typesToExport []types.CertAuthType

	if exportSecrets {
		mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
		if err == nil {
			ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
		} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
			return nil, trace.Wrap(err)
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
			return nil, trace.Wrap(err)
		}
		typesToExport = []types.CertAuthType{authType}
	}
	localAuthName, err := client.GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []types.CertAuthority
	for _, at := range typesToExport {
		cas, err := client.GetCertAuthorities(ctx, at, exportSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
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
						return nil, trace.Wrap(err)
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
					return nil, trace.Wrap(err)
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
					return nil, trace.Wrap(err)
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
				return nil, trace.BadParameter("unknown user type: %q", ca.GetType())
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// write the export friendly string
			ret.WriteString(castr)
		}
	}

	return []*ExportedAuthority{
		{Data: []byte(ret.String())},
	}, nil
}

type exportTLSAuthorityRequest struct {
	AuthType          types.CertAuthType
	UnpackPEM         bool
	ExportPrivateKeys bool
}

func exportTLSAuthority(ctx context.Context, client authclient.ClientI, req exportTLSAuthorityRequest) ([]*ExportedAuthority, error) {
	clusterName, err := client.GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certAuthority, err := client.GetCertAuthority(
		ctx,
		types.CertAuthID{Type: req.AuthType, DomainName: clusterName},
		req.ExportPrivateKeys,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	activeKeys := certAuthority.GetActiveKeys().TLS
	// TODO(codingllama): Export AdditionalTrustedKeys as well?

	authorities := make([]*ExportedAuthority, len(activeKeys))
	for i, activeKey := range activeKeys {
		bytesToExport := activeKey.Cert
		if req.ExportPrivateKeys {
			bytesToExport = activeKey.Key
		}

		if req.UnpackPEM {
			block, _ := pem.Decode(bytesToExport)
			if block == nil {
				return nil, trace.BadParameter("invalid PEM data")
			}
			bytesToExport = block.Bytes
		}

		authorities[i] = &ExportedAuthority{
			Data: bytesToExport,
		}
	}

	return authorities, nil
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
func hostCAFormat(ca types.CertAuthority, keyBytes []byte, client authclient.ClientI) (string, error) {
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

// IsIntegrationAuthorityType returns true if provided type is an integration CA
// type.
func IsIntegrationAuthorityType(authType string) bool {
	return authType == types.IntegrationSubKindGitHub
}

// ExportIntegrationAuthoritiesRequest has the required fields to create an
// export authorities request for integrations.
type ExportIntegrationAuthoritiesRequest struct {
	// AuthType is the type of CA to be exported. See
	// ExportIntegrationAuthorities for details.
	AuthType string
	// MatchFingerprint filters authorities using provided fingerprint if
	// specified. Fingerprint must be the SHA256 of the Authority's public key.
	MatchFingerprint string
	// Integration is the name of the integration resource.
	Integration string
}

// ExportIntegrationAuthorities exports the public keys of all authorities
// associated with an integration.
//
// Integrations that require certificate authorities have their CAs saved as
// plugin credentials per integration. This ensures compatibility with services
// like GitHub which mandate the use of unique CAs cross all integrations.
// In addition, unlike cluster-level CAs, integration CAs are not used between
// Teleport clients/agents/clusters. Integration CAs should only be used by an
// agent to authenticate the service associated with the integration.
//
// Exporting integration CAs requires READ access to the integration. Currently,
// "github" is the only supported AuthType.
//
// "github" AuthType returns the public key of each SSH certificate authority in
// a single line. Each line starts with key type like "ssh-rsa AA..." and can be
// copied to the text box when configuring new CA for a GitHub organization.
// Once a CA is added to the GitHub organization, GitHub only displays the
// SHA256 fingerprint of the key and the date it was added. The MatchFingerprint
// option can be used to verify whether a fingerprint corresponds to that
// particular integration.
func ExportIntegrationAuthorities(ctx context.Context, client authclient.ClientI, req ExportIntegrationAuthoritiesRequest) ([]*ExportedAuthority, error) {
	if req.Integration == "" {
		return nil, trace.BadParameter("integration name is required when exporting integration authorities")
	}

	switch req.AuthType {
	case types.IntegrationSubKindGitHub:
		keySet, err := fetchIntegrationCAKeySet(ctx, client, req.Integration)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ret, err := exportGitHubCAs(keySet, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []*ExportedAuthority{
			{Data: []byte(ret)},
		}, nil

	default:
		return nil, trace.BadParameter("unknown integration CA type %q", req.AuthType)
	}
}

func fetchIntegrationCAKeySet(ctx context.Context, client authclient.ClientI, integration string) (*types.CAKeySet, error) {
	resp, err := client.IntegrationsClient().ExportIntegrationCertAuthorities(ctx, &integrationpb.ExportIntegrationCertAuthoritiesRequest{
		Integration: integration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.CertAuthorities, nil
}

func exportGitHubCAs(keySet *types.CAKeySet, req ExportIntegrationAuthoritiesRequest) (string, error) {
	ret := strings.Builder{}
	for _, key := range keySet.SSH {
		if req.MatchFingerprint != "" {
			if fingerprint, err := sshutils.AuthorizedKeyFingerprint(key.PublicKey); err != nil {
				return "", trace.Wrap(err)
			} else if !sshutils.EqualFingerprints(req.MatchFingerprint, fingerprint) {
				continue
			}
		}

		// GitHub only needs the keys like "ssh-rsa xxx" so print them without
		// cert-authority for easier copy-and-paste.
		ret.WriteString(fmt.Sprintf("%s integration=%s\n", strings.TrimSpace(string(key.PublicKey)), req.Integration))
	}
	if req.MatchFingerprint != "" && ret.Len() == 0 {
		return "", trace.NotFound("no authorities found matching the provided fingerprint")
	}
	return ret.String(), nil
}
