/*
Copyright 2021 Gravitational, Inc.

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
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GenerateDatabaseCert generates client certificate used by a database
// service to authenticate with the database instance.
func (s *Server) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databaseCA, err := s.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		if trace.IsNotFound(err) {
			// Database CA doesn't exist. Fallback to Host CA.
			// https://github.com/gravitational/teleport/issues/5029
			databaseCA, err = s.GetCertAuthority(ctx, types.CertAuthID{
				Type:       types.HostCA,
				DomainName: clusterName.GetClusterName(),
			}, true)
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	caCert, signer, err := getCAandSigner(ctx, s.GetKeyStore(), databaseCA, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certReq := tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: csr.PublicKey,
		Subject:   csr.Subject,
		NotAfter:  s.clock.Now().UTC().Add(req.TTL.Get()),
	}
	if req.CertificateExtensions == proto.DatabaseCertRequest_WINDOWS_SMARTCARD {
		// Pass through ExtKeyUsage (which we need for Smartcard Logon usage)
		// and SubjectAltName (which we need for otherName SAN, not supported
		// out of the box in crypto/x509) extensions only.
		certReq.ExtraExtensions = filterExtensions(csr.Extensions, oidExtKeyUsage, oidSubjectAltName)
		certReq.KeyUsage = x509.KeyUsageDigitalSignature
		// CRL is required for Windows smartcard certs.
		certReq.CRLDistributionPoints = []string{req.CRLEndpoint}
	} else {
		// Include provided server names as SANs in the certificate, CommonName
		// has been deprecated since Go 1.15:
		//   https://golang.org/doc/go1.15#commonname
		certReq.DNSNames = getServerNames(req)
	}
	cert, err := tlsCA.GenerateCertificate(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.DatabaseCertResponse{
		Cert:    cert,
		CACerts: services.GetTLSCerts(databaseCA),
	}, nil
}

// getCAandSigner returns correct signer and CA that should be used when generating database certificate.
// This function covers the database CA rotation scenario when on rotation init phase additional/new TLS
// key should be used to sign the database CA. Otherwise, the trust chain will break after the old CA is
// removed - standby phase.
func getCAandSigner(ctx context.Context, keyStore *keystore.Manager, databaseCA types.CertAuthority, req *proto.DatabaseCertRequest,
) ([]byte, crypto.Signer, error) {
	if req.RequesterName == proto.DatabaseCertRequest_TCTL &&
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
func (s *Server) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	if !modules.GetModules().Features().DB {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for database access, please contact the cluster administrator")
	}

	log.Debugf("Signing database CSR for cluster %v.", req.ClusterName)

	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostCA, err := s.GetCertAuthority(ctx, types.CertAuthID{
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
	roles, err := services.FetchRoles(id.Groups, s, id.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the correct cert TTL based on roles.
	ttl := roles.AdjustSessionTTL(apidefaults.CertDuration)

	// Generate the TLS certificate.
	ca, err := s.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, signer, err := s.GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, err := tlsAuthority.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: csr.PublicKey,
		Subject:   subject,
		NotAfter:  s.clock.Now().UTC().Add(ttl),
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
func (s *Server) GenerateSnowflakeJWT(ctx context.Context, req *proto.SnowflakeJWTRequest) (*proto.SnowflakeJWTResponse, error) {
	if !modules.GetModules().Features().DB {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for database access, please contact the cluster administrator")
	}

	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := s.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
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

	subject, issuer := getSnowflakeJWTParams(req.AccountName, req.UserName, pubKey)

	_, signer, err := s.GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), s.clock)
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

func getSnowflakeJWTParams(accountName, userName string, publicKey []byte) (string, string) {
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
	log.Debugf("Signing database JWT token for %s %s", accnTokenCap, userNameCap)

	subject := fmt.Sprintf("%s.%s", accnTokenCap, userNameCap)

	keyFp := sha256.Sum256(publicKey)
	keyFpStr := base64.StdEncoding.EncodeToString(keyFp[:])

	// Generate issuer name in the Snowflake required format.
	issuer := fmt.Sprintf("%s.%s.SHA256:%s", accnTokenCap, userNameCap, keyFpStr)

	return subject, issuer
}

func filterExtensions(extensions []pkix.Extension, oids ...asn1.ObjectIdentifier) []pkix.Extension {
	filtered := make([]pkix.Extension, 0, len(oids))
	for _, e := range extensions {
		for _, id := range oids {
			if e.Id.Equal(id) {
				filtered = append(filtered, e)
			}
		}
	}
	return filtered
}

var (
	oidExtKeyUsage    = asn1.ObjectIdentifier{2, 5, 29, 37}
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
)
