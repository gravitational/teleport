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

package winpki

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

type certRequest struct {
	csrPEM []byte
	keyDER []byte

	// Optionally specifies the AD domain where CRLs are published.
	// If omitted then the cert will not specify a CRL distribution point.
	cdpDomain string
}

func createUsersExtension(groups []string) (pkix.Extension, error) {
	value, err := json.Marshal(struct {
		CreateUser bool     `json:"createUser"`
		Groups     []string `json:"groups"`
	}{true, groups})
	if err != nil {
		return pkix.Extension{}, trace.Wrap(err)
	}
	return pkix.Extension{
		Id:    tlsca.CreateWindowsUserOID,
		Value: value,
	}, nil
}

func getCertRequest(req *GenerateCredentialsRequest) (*certRequest, error) {
	// Important: rdpclient currently only supports 2048-bit RSA keys.
	// If you switch the key type here, update handle_general_authentication in
	// rdp/rdpclient/src/piv.rs accordingly.
	rsaKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Also important: rdpclient expects the private key to be in PKCS1 format.
	keyDER := x509.MarshalPKCS1PrivateKey(rsaKey.(*rsa.PrivateKey))

	// Generate the Windows-compatible certificate, see
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	// for requirements.
	san, err := SubjectAltNameExtension(req.Username, req.Domain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: req.Username},
		// We have to pass SAN and ExtKeyUsage as raw extensions because
		// crypto/x509 doesn't support what we need:
		// - x509.ExtKeyUsage doesn't have the Smartcard Logon variant
		// - x509.CertificateRequest doesn't have OtherName SAN fields (which
		//   is a type of SAN distinct from DNSNames, EmailAddresses, IPAddresses
		//   and URIs)
		ExtraExtensions: []pkix.Extension{
			EnhancedKeyUsageExtension,
			san,
		},
	}

	if req.CreateUser {
		createUser, err := createUsersExtension(req.Groups)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		csr.ExtraExtensions = append(csr.ExtraExtensions, createUser)
	}

	if req.AD {
		csr.ExtraExtensions = append(csr.ExtraExtensions, pkix.Extension{
			Id:    tlsca.ADStatusOID,
			Value: []byte("AD"),
		})
	}

	if req.ActiveDirectorySID != "" {
		adUserMapping, err := asn1.Marshal(SubjectAltName[adSid]{
			otherName[adSid]{
				OID: ADUserMappingInternalOID,
				Value: adSid{
					Value: []byte(req.ActiveDirectorySID),
				},
			}})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		csr.ExtraExtensions = append(csr.ExtraExtensions, pkix.Extension{
			Id:    ADUserMappingExtensionOID,
			Value: adUserMapping,
		})
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, rsaKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	cr := &certRequest{
		csrPEM: csrPEM,
		keyDER: keyDER,
	}

	if !req.OmitCDP {
		cr.cdpDomain = cmp.Or(req.PKIDomain, req.Domain)
	}

	return cr, nil
}

// AuthInterface is a subset of auth.ClientI
type AuthInterface interface {
	// GenerateDatabaseCert generates a database certificate for windows SQL Server
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)
	// GenerateWindowsDesktopCert generates a windows remote desktop certificate
	GenerateWindowsDesktopCert(context.Context, *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error)
	// GetClusterName returns a types.ClusterName interface
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// GenerateCredentialsRequest are the request parameters for
// generating a windows cert/key pair
type GenerateCredentialsRequest struct {
	// Username is the Windows username
	Username string
	// Domain is the Active Directory domain of the user.
	Domain string
	// PKIDomain is the Active Directory domain where CRLs are published.
	// (Optional, defaults to the same domain as the user.)
	PKIDomain string
	// TTL is the ttl for the certificate
	TTL time.Duration
	// ClusterName is the local cluster name
	ClusterName string
	// ActiveDirectorySID is the SID of the Windows user
	// specified by Username. If specified (!= ""), it is
	// encoded in the certificate per https://go.microsoft.com/fwlink/?linkid=2189925.
	ActiveDirectorySID string
	// CAType is the certificate authority type used to generate the certificate.
	// This is used to proper generate the CRL LDAP path.
	CAType types.CertAuthType
	// CreateUser specifies if Windows user should be created if missing
	CreateUser bool
	// Groups are groups that user should be member of
	Groups []string

	// OmitCDP can be used to prevent Teleport from issuing certs with a
	// CRL Distribution Point (CDP). CDPs are required in user certificates
	// for RDP, but they can be omitted for certs that are used for LDAP binds.
	OmitCDP bool

	// AD is true if we're connecting to a domain-joined desktop.
	AD bool
}

// GenerateWindowsDesktopCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
func GenerateWindowsDesktopCredentials(ctx context.Context, auth AuthInterface, req *GenerateCredentialsRequest) (certDER, keyDER []byte, err error) {
	certReq, err := getCertRequest(req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	genResp, err := auth.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{
		CSR:       certReq.csrPEM,
		CRLDomain: certReq.cdpDomain,
		TTL:       proto.Duration(req.TTL),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certBlock, _ := pem.Decode(genResp.Cert)
	if certBlock == nil {
		return nil, nil, trace.BadParameter("failed to decode certificate")
	}
	certDER = certBlock.Bytes
	keyDER = certReq.keyDER
	return certDER, keyDER, nil
}

// generateDatabaseCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
func generateDatabaseCredentials(ctx context.Context, auth AuthInterface, req *GenerateCredentialsRequest) (certDER, keyDER []byte, caCerts [][]byte, err error) {
	certReq, err := getCertRequest(req)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	genResp, err := auth.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
		CSR: certReq.csrPEM,
		// LDAP URI pointing at the CRL created with updateCRL.
		//
		// The full format is:
		// ldap://domain_controller_addr/distinguished_name_and_parameters.
		//
		// Using ldap:///distinguished_name_and_parameters (with empty
		// domain_controller_addr) will cause Windows to fetch the CRL from any
		// of its current domain controllers.
		CRLDomain:             certReq.cdpDomain,
		TTL:                   proto.Duration(req.TTL),
		CertificateExtensions: proto.DatabaseCertRequest_WINDOWS_SMARTCARD,
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	certBlock, _ := pem.Decode(genResp.Cert)
	if certBlock == nil {
		return nil, nil, nil, trace.BadParameter("failed to decode certificate")
	}
	certDER = certBlock.Bytes
	keyDER = certReq.keyDER
	return certDER, keyDER, genResp.CACerts, nil
}

// DatabaseCredentials returns certificate and private key bytes encoded in PEM format for use with `kinit`.
func DatabaseCredentials(ctx context.Context, auth AuthInterface, req *GenerateCredentialsRequest) (certPEM, keyPEM []byte, caCerts [][]byte, err error) {
	certDER, keyDER, caCerts, err := generateDatabaseCredentials(ctx, auth, req)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})

	return
}

// The following vars contain the various object identifiers required for smartcard
// login certificates.
//
// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
var (
	// EnhancedKeyUsageExtensionOID is the object identifier for a
	// certificate's enhanced key usage extension
	EnhancedKeyUsageExtensionOID = asn1.ObjectIdentifier{2, 5, 29, 37}

	// SubjectAltNameExtensionOID is the object identifier for a
	// certificate's subject alternative name extension
	SubjectAltNameExtensionOID = asn1.ObjectIdentifier{2, 5, 29, 17}

	// ClientAuthenticationOID is the object idnetifier that is used to
	// include client SSL authentication in a certificate's enhanced
	// key usage
	ClientAuthenticationOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}

	// SmartcardLogonOID is the object identifier that is used to include
	// smartcard login in a certificate's enhanced key usage
	SmartcardLogonOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 2}

	// UPNOtherNameOID is the object identifier that is used to include
	// the user principal name in a certificate's subject alternative name
	UPNOtherNameOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 3}

	// ADUserMappingExtensionOID is the Active Directory SID extension for mapping certificates
	// to their user's Active Directory SID. This value goes in the Id field of the pkix.Extension.
	// See https://go.microsoft.com/fwlink/?linkid=2189925.
	ADUserMappingExtensionOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 25, 2}
	// ADUserMappingInternalOID is the OID that's sent as part of the Other Name section
	// of the Active Directory SID extension. There's limited documentation on this extension,
	// this value was determined empirically based on how AD CA's Enterprise CA issues these
	// certificates post the May 10, 2022 Windows update.
	ADUserMappingInternalOID = append(ADUserMappingExtensionOID, 1)
)

// EnhancedKeyUsageExtension is a set of required extended key fields specific for Microsoft certificates
var EnhancedKeyUsageExtension = pkix.Extension{
	Id: EnhancedKeyUsageExtensionOID,
	Value: func() []byte {
		val, err := asn1.Marshal([]asn1.ObjectIdentifier{
			ClientAuthenticationOID,
			SmartcardLogonOID,
		})
		if err != nil {
			panic(err)
		}
		return val
	}(),
}

// SubjectAltNameExtension fills in the SAN for a Windows certificate
func SubjectAltNameExtension(user, domain string) (pkix.Extension, error) {
	// Setting otherName SAN according to
	// https://samfira.com/2020/05/16/golang-x-509-certificates-and-othername/
	//
	// otherName SAN is needed to pass the UPN of the user, per
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	ext := pkix.Extension{Id: SubjectAltNameExtensionOID}
	var err error
	ext.Value, err = asn1.Marshal(
		SubjectAltName[upn]{
			OtherName: otherName[upn]{
				OID: UPNOtherNameOID,
				Value: upn{
					Value: fmt.Sprintf("%s@%s", user, domain), // TODO(zmb3): sanitize username to avoid domain spoofing
				},
			},
		},
	)
	if err != nil {
		return ext, trace.Wrap(err)
	}
	return ext, nil
}

// Types for ASN.1 SAN serialization.
type (
	// SubjectAltName is a struct that can be marshaled as ASN.1
	// into the SAN field in an x.509 certificate.
	//
	// See RFC 3280: https://www.ietf.org/rfc/rfc3280.txt
	//
	// T is the ASN.1 encodeable struct corresponding to an otherName
	// item of the GeneralNames sequence.
	SubjectAltName[T any] struct {
		OtherName otherName[T] `asn1:"tag:0"`
	}

	otherName[T any] struct {
		OID   asn1.ObjectIdentifier
		Value T `asn1:"tag:0"`
	}

	upn struct {
		Value string `asn1:"utf8"`
	}

	adSid struct {
		// Value is the bytes representation of the user's SID string,
		// e.g. []byte("S-1-5-21-1329593140-2634913955-1900852804-500")
		Value []byte // Gets encoded as an asn1 octet string
	}
)
