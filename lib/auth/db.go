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
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GenerateDatabaseCert generates client certificate used by a database
// service to authenticate with the database instance.
func (a *Server) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	if req.RequesterName == proto.DatabaseCertRequest_TCTL {
		// tctl/web cert request needs to generate a db server cert and trust
		// the db client CA.
		return a.generateDatabaseServerCert(ctx, req)
	}
	// db service needs to generate a db client cert and trust the db server CA.
	return a.generateDatabaseClientCert(ctx, req)
}

// generateDatabaseServerCert generates database server certificate used by a
// database to authenticate itself to a database service.
func (a *Server) generateDatabaseServerCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// databases should be configured to trust the DatabaseClientCA when
	// clients connect so return DatabaseClientCA in the response.
	dbClientCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseClientCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbServerCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := a.generateDatabaseCert(ctx, req, dbServerCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.DatabaseCertResponse{
		Cert:    cert,
		CACerts: services.GetTLSCerts(dbClientCA),
	}, nil
}

// generateDatabaseClientCert generates client certificate used by a database
// service to authenticate with the database instance.
func (a *Server) generateDatabaseClientCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbClientCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseClientCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := a.generateDatabaseCert(ctx, req, dbClientCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// db clients should trust the Database Server CA when establishing
	// connection to a database, so return that CA's certs in the response.
	//
	// The only exception is the SQL Server with PKINIT integration, where the
	// `kinit` command line needs our client CA to trust the user certificates
	// we pass.
	returnedCAType := types.DatabaseCA
	if req.CertificateExtensions == proto.DatabaseCertRequest_WINDOWS_SMARTCARD {
		returnedCAType = types.DatabaseClientCA
	}

	returnedCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       returnedCAType,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.DatabaseCertResponse{
		Cert:    cert,
		CACerts: services.GetTLSCerts(returnedCA),
	}, nil
}

func (a *Server) generateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest, ca types.CertAuthority) ([]byte, error) {
	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCert, signer, err := getCAandSigner(ctx, a.GetKeyStore(), ca, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certReq := tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: csr.PublicKey,
		Subject:   csr.Subject,
		NotAfter:  a.clock.Now().UTC().Add(req.TTL.Get()),
	}
	if req.CertificateExtensions == proto.DatabaseCertRequest_WINDOWS_SMARTCARD {
		// Pass through ExtKeyUsage (which we need for Smartcard Logon usage)
		// and SubjectAltName (which we need for otherName SAN, not supported
		// out of the box in crypto/x509) extensions only.
		certReq.ExtraExtensions = filterExtensions(a.CloseContext(), a.logger, csr.Extensions, oidExtKeyUsage, oidSubjectAltName, oidADUserMapping)
		certReq.KeyUsage = x509.KeyUsageDigitalSignature
		// CRL is required for Windows smartcard certs.
		certReq.CRLDistributionPoints = []string{req.CRLEndpoint}
	} else {
		// Include provided server names as SANs in the certificate, CommonName
		// has been deprecated since Go 1.15:
		//   https://golang.org/doc/go1.15#commonname
		certReq.DNSNames = getServerNames(req)

		// The windows smartcard cert req already does the same in
		// lib/winpki/windows.go, along with another ExtKeyUsage for
		// smartcard logon that we don't want to override above.
		switch ca.GetType() {
		case types.DatabaseCA:
			// Override ExtKeyUsage to ExtKeyUsageServerAuth.
			certReq.ExtraExtensions = append(certReq.ExtraExtensions, extKeyUsageServerAuthExtension)
		case types.DatabaseClientCA:
			// Override ExtKeyUsage to ExtKeyUsageClientAuth.
			certReq.ExtraExtensions = append(certReq.ExtraExtensions, extKeyUsageClientAuthExtension)
		}
	}
	cert, err := tlsCA.GenerateCertificate(certReq)
	return cert, trace.Wrap(err)
}

// getCAandSigner returns correct signer and CA that should be used when generating database certificate.
// This function covers the database CA rotation scenario when on rotation init phase additional/new TLS
// key should be used to sign the database CA. Otherwise, the trust chain will break after the old CA is
// removed - standby phase.
func getCAandSigner(ctx context.Context, keyStore *keystore.Manager, databaseCA types.CertAuthority, req *proto.DatabaseCertRequest,
) ([]byte, crypto.Signer, error) {
	if req.RequesterName == proto.DatabaseCertRequest_TCTL &&
		databaseCA.GetType() == types.DatabaseCA &&
		databaseCA.GetRotation().Phase == types.RotationPhaseInit {
		return keyStore.GetAdditionalTrustedTLSCertAndSigner(ctx, databaseCA)
	}

	return keyStore.GetTLSCertAndSigner(ctx, databaseCA)
}

// getServerNames returns deduplicated list of server names from signing request.
func getServerNames(req *proto.DatabaseCertRequest) []string {
	serverNames := req.ServerNames
	if req.ServerName != "" { // Include legacy ServerName field for compatibility.
		serverNames = append(serverNames, req.ServerName)
	}
	return utils.Deduplicate(serverNames)
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (a *Server) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.DB).Enabled {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for database access, please contact the cluster administrator")
	}

	a.logger.DebugContext(ctx, "Signing database CSR for cluster", "cluster", req.ClusterName)

	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	// Extract the identity from the CSR.
	id, err := tlsca.FromSubject(csr.Subject, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure that the CSR originated from the local cluster user.
	if clusterName.GetClusterName() != id.TeleportCluster {
		return nil, trace.AccessDenied("can't sign database CSR for identity %v", id)
	}

	// Update "accepted usage" field to indicate that the certificate can
	// only be used for database proxy server and re-encode the identity.
	id.Usage = []string{teleport.UsageDatabaseOnly}
	subject, err := id.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract user roles from the identity.
	roles, err := services.FetchRoles(id.Groups, a, id.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the correct cert TTL based on roles.
	ttl := roles.AdjustSessionTTL(apidefaults.CertDuration)

	// Generate the TLS certificate.
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, signer, err := a.GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, err := tlsAuthority.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: csr.PublicKey,
		Subject:   subject,
		NotAfter:  a.clock.Now().UTC().Add(ttl),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.DatabaseCSRResponse{
		Cert:    tlsCert,
		CACerts: services.GetTLSCerts(hostCA),
	}, nil
}

// GenerateSnowflakeJWT generates JWT in the format required by Snowflake.
func (a *Server) GenerateSnowflakeJWT(ctx context.Context, req *proto.SnowflakeJWTRequest) (*proto.SnowflakeJWTResponse, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.DB).Enabled {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for database access, please contact the cluster administrator")
	}

	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseClientCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// DatabaseClientCA doesn't exist, fallback to DatabaseCA.
		ca, err = a.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.DatabaseCA,
			DomainName: clusterName.GetClusterName(),
		}, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if len(ca.GetActiveKeys().TLS) == 0 {
		return nil, trace.Errorf("incorrect database CA; missing TLS key")
	}

	tlsCert := ca.GetActiveKeys().TLS[0].Cert

	block, _ := pem.Decode(tlsCert)
	if block == nil {
		return nil, trace.BadParameter("failed to parse TLS certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subject, issuer := getSnowflakeJWTParams(ctx, req.AccountName, req.UserName, pubKey)

	_, signer, err := a.GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), a.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := privateKey.SignSnowflake(jwt.SignParams{
		Username: subject,
		Expires:  time.Now().Add(86400 * time.Second), // the same validity as the JWT generated by snowsql
	}, issuer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.SnowflakeJWTResponse{
		Token: token,
	}, nil
}

func getSnowflakeJWTParams(ctx context.Context, accountName, userName string, publicKey []byte) (string, string) {
	// Use only the first part of the account name to generate JWT
	// Based on:
	// https://github.com/snowflakedb/snowflake-connector-python/blob/f2f7e6f35a162484328399c8a50a5015825a5573/src/snowflake/connector/auth_keypair.py#L83
	accNameSeparator := "."
	if strings.Contains(accountName, ".global") {
		accNameSeparator = "-"
	}

	accnToken, _, _ := strings.Cut(accountName, accNameSeparator)
	accnTokenCap := strings.ToUpper(accnToken)
	userNameCap := strings.ToUpper(userName)
	logger.DebugContext(ctx, "Signing database JWT token",
		"account_name", accnTokenCap,
		"user_name", userNameCap,
	)

	subject := fmt.Sprintf("%s.%s", accnTokenCap, userNameCap)

	keyFp := sha256.Sum256(publicKey)
	keyFpStr := base64.StdEncoding.EncodeToString(keyFp[:])

	// Generate issuer name in the Snowflake required format.
	issuer := fmt.Sprintf("%s.%s.SHA256:%s", accnTokenCap, userNameCap, keyFpStr)

	return subject, issuer
}

func filterExtensions(ctx context.Context, logger *slog.Logger, extensions []pkix.Extension, oids ...asn1.ObjectIdentifier) []pkix.Extension {
	filtered := make([]pkix.Extension, 0, len(oids))
	for _, e := range extensions {
		matched := false
		for _, id := range oids {
			if e.Id.Equal(id) {
				matched = true
			}
		}
		if matched {
			filtered = append(filtered, e)
		} else {
			logger.WarnContext(ctx, "filtering out unexpected certificate extension; this may indicate Teleport bug", "oid", e.Id.String(), "value", e.Value, "critical", e.Critical)
		}
	}
	return filtered
}

// TODO(gavin): move OIDs from here and in lib/winpki to lib/tlsca package.
var (
	oidExtKeyUsage    = asn1.ObjectIdentifier{2, 5, 29, 37}
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidADUserMapping  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 25, 2}

	oidExtKeyUsageServerAuth       = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1}
	oidExtKeyUsageClientAuth       = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}
	extKeyUsageServerAuthExtension = pkix.Extension{
		Id: oidExtKeyUsage,
		Value: func() []byte {
			val, err := asn1.Marshal([]asn1.ObjectIdentifier{oidExtKeyUsageServerAuth})
			if err != nil {
				panic(err)
			}
			return val
		}(),
	}
	extKeyUsageClientAuthExtension = pkix.Extension{
		Id: oidExtKeyUsage,
		Value: func() []byte {
			val, err := asn1.Marshal([]asn1.ObjectIdentifier{oidExtKeyUsageClientAuth})
			if err != nil {
				panic(err)
			}
			return val
		}(),
	}
)
