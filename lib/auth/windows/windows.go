// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windows

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// CertTTL is the TTL for Teleport-issued Windows Certificates.
	// Certificates are requested on each connection attempt, so the TTL is
	// deliberately set to a small value to give enough time to establish a
	// single desktop session.
	CertTTL = 5 * time.Minute
)

type certRequest struct {
	csrPEM      []byte
	crlEndpoint string
	keyDER      []byte
}

func getCertRequest(username, domain string, clusterName string, ldapConfig LDAPConfig, activeDirectorySID []byte) (*certRequest, error) {
	// Important: rdpclient currently only supports 2048-bit RSA keys.
	// If you switch the key type here, update handle_general_authentication in
	// rdp/rdpclient/src/piv.rs accordingly.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Also important: rdpclient expects the private key to be in PKCS1 format.
	keyDER := x509.MarshalPKCS1PrivateKey(rsaKey)

	// Generate the Windows-compatible certificate, see
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	// for requirements.
	san, err := SubjectAltNameExtension(username, domain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: username},
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
	// asn1.Unmarshal
	realvalstr := "S-1-5-21-1329593140-2634913955-1900852804-500"
	realval, _ := asn1.Marshal(realvalstr)
	fmt.Printf("realval = %v\n", realval)
	realval2 := []byte{0x30, 0x3f, 0xa0, 0x3d, 0x06, 0x0a, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x19, 0x02, 0x01, 0xa0, 0x2f, 0x04, 45, 83, 45, 49, 45, 53, 45, 50, 49, 45, 49, 51, 50, 57, 53, 57, 51, 49, 52, 48, 45, 50, 54, 51, 52, 57, 49, 51, 57, 53, 53, 45, 49, 57, 48, 48, 56, 53, 50, 56, 48, 52, 45, 53, 48, 48}
	val := "S-1-5-21-1381186052-4247692386-135928078-1225"
	value, _ := asn1.Marshal(val)
	// 0x30 0x3f ; SEQUENCE (0x3f Bytes)
	value2 := []byte{0x30, 0x3f, 0xa0, 0x3d, 0x06, 0x0a, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x82, 0x37, 0x19, 0x02, 0x01, 0xa0, 0x2f, 0x04, 0x2d, 0x53, 0x2d, 0x31, 0x2d, 0x35, 0x2d, 0x32, 0x31, 0x2d, 0x31, 0x33, 0x38, 0x31, 0x31, 0x38, 0x36, 0x30, 0x35, 0x32, 0x2d, 0x34, 0x32, 0x34, 0x37, 0x36, 0x39, 0x32, 0x33, 0x38, 0x36, 0x2d, 0x31, 0x33, 0x35, 0x39, 0x32, 0x38, 0x30, 0x37, 0x38, 0x2d, 0x31, 0x32, 0x32, 0x35}
	fmt.Printf("value = %v\n", value)
	asutf, _ := asn1.Marshal(struct {
		Value string `asn1:"utf8"`
	}{
		Value: val,
	})
	asia5, _ := asn1.Marshal(struct {
		Value string `asn1:"ia5"`
	}{
		Value: val,
	})
	asprintable, _ := asn1.Marshal(struct {
		Value string `asn1:"printable"`
	}{
		Value: val,
	})
	fmt.Printf("value as utf = %v\n", asutf)
	fmt.Printf("value as asia5 = %v\n", asia5)
	fmt.Printf("value as asprintable = %v\n", asprintable)
	fmt.Printf("value2 = %v\n", value2)
	asn1oid, _ := asn1.Marshal(ActiveDirectorySidOID)
	fmt.Printf("asn1OID = %x\n", asn1oid)
	oidstruct, _ := asn1.Marshal(parentoidstruct{
		Oidstruct: oidstruct{
			OID: ActiveDirectorySidOID1,
			Value: valstruct{
				Value: []byte(val),
			},
		}})
	fmt.Printf("oidstruct = %x\n", oidstruct)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if activeDirectorySID != nil {
		fmt.Printf("appending activeDirectorySID\n") //todo(isaiah): delete
		csr.ExtraExtensions = append(csr.ExtraExtensions, pkix.Extension{
			Id:    ActiveDirectorySidOID,
			Value: realval2,
		})
	}

	fmt.Printf("len(csr.ExtraExtensions) = %v\n", len(csr.ExtraExtensions))

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, rsaKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	// Note: this CRL DN may or may not be the same DN published in updateCRL.
	//
	// There can be multiple AD domains connected to Teleport. Each
	// windows_desktop_service is connected to a single AD domain and publishes
	// CRLs in it. Each service can also handle RDP connections for a different
	// domain, with the assumption that some other windows_desktop_service
	// published a CRL there.
	crlDN := crlDN(clusterName, ldapConfig)
	return &certRequest{csrPEM: csrPEM, crlEndpoint: fmt.Sprintf("ldap:///%s?certificateRevocationList?base?objectClass=cRLDistributionPoint", crlDN), keyDER: keyDER}, nil
}

type parentoidstruct struct {
	Oidstruct oidstruct `asn1:"tag:0"`
}

type oidstruct struct {
	OID asn1.ObjectIdentifier
	// Value string `asn1:"utf8"`
	Value valstruct `asn1:"tag:0"`
}

type valstruct struct {
	Value []byte
}

// AuthInterface is a subset of auth.ClientI
type AuthInterface interface {
	// GenerateDatabaseCert generates a database certificate for windows SQL Server
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)
	// GenerateWindowsDesktopCert generates a windows remote desktop certificate
	GenerateWindowsDesktopCert(context.Context, *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error)
	// GetCertAuthority returns a types.CertAuthority interface
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)
	// GetClusterName returns a types.ClusterName interface
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// GenerateCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
// TODO(isaiah): convert the parameters into a struct
func GenerateCredentials(ctx context.Context, username, domain string, ttl time.Duration, clusterName string, ldapConfig LDAPConfig, authClient AuthInterface, activeDirectorySID []byte) (certDER, keyDER []byte, err error) {
	certReq, err := getCertRequest(username, domain, clusterName, ldapConfig, activeDirectorySID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	genResp, err := authClient.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{
		CSR: certReq.csrPEM,
		// LDAP URI pointing at the CRL created with updateCRL.
		//
		// The full format is:
		// ldap://domain_controller_addr/distinguished_name_and_parameters.
		//
		// Using ldap:///distinguished_name_and_parameters (with empty
		// domain_controller_addr) will cause Windows to fetch the CRL from any
		// of its current domain controllers.
		CRLEndpoint: certReq.crlEndpoint,
		TTL:         proto.Duration(ttl),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certBlock, _ := pem.Decode(genResp.Cert)
	certDER = certBlock.Bytes
	keyDER = certReq.keyDER
	return certDER, keyDER, nil
}

// GenerateDatabaseCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
func GenerateDatabaseCredentials(ctx context.Context, username, domain string, ttl time.Duration, clusterName string, ldapConfig LDAPConfig, authClient AuthInterface) (certDER, keyDER []byte, err error) {
	certReq, err := getCertRequest(username, domain, clusterName, ldapConfig, nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	genResp, err := authClient.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
		CSR: certReq.csrPEM,
		// LDAP URI pointing at the CRL created with updateCRL.
		//
		// The full format is:
		// ldap://domain_controller_addr/distinguished_name_and_parameters.
		//
		// Using ldap:///distinguished_name_and_parameters (with empty
		// domain_controller_addr) will cause Windows to fetch the CRL from any
		// of its current domain controllers.
		CRLEndpoint:           certReq.crlEndpoint,
		TTL:                   proto.Duration(ttl),
		CertificateExtensions: proto.DatabaseCertRequest_WINDOWS_SMARTCARD,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certBlock, _ := pem.Decode(genResp.Cert)
	certDER = certBlock.Bytes
	keyDER = certReq.keyDER
	return certDER, keyDER, nil
}

// CertKeyPEM returns certificate and private key bytes encoded in PEM format for use with `kinit`
func CertKeyPEM(ctx context.Context, username, domain string, ttl time.Duration, clusterName string, ldapConfig LDAPConfig, authClient AuthInterface) (certPEM, keyPEM []byte, err error) {
	certDER, keyDER, err := GenerateDatabaseCredentials(ctx, username, domain, ttl, clusterName, ldapConfig, authClient)
	if err != nil {
		return nil, nil, trace.Wrap(err)
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

	// ActiveDirectorySidOID is the Active Directory SID extension for mapping certificates
	// to their user's Active Directory SID. See https://go.microsoft.com/fwlink/?linkid=2189925
	ActiveDirectorySidOID  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 25, 2}
	ActiveDirectorySidOID1 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 25, 2, 1}
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
	// othernName SAN is needed to pass the UPN of the user, per
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	ext := pkix.Extension{Id: SubjectAltNameExtensionOID}
	var err error
	ext.Value, err = asn1.Marshal(
		SubjectAltName{
			OtherName: otherName{
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

// SubjectAltName is a struct for marshaling the SAN field in a certificate
type SubjectAltName struct {
	OtherName otherName `asn1:"tag:0"`
}

type otherName struct {
	OID   asn1.ObjectIdentifier
	Value upn `asn1:"tag:0"`
}

type upn struct {
	Value string `asn1:"utf8"`
}
