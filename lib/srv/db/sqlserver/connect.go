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

package sqlserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
)

// Connector defines an interface for connecting to a SQL Server so it can be
// swapped out in tests.
type Connector interface {
	Connect(context.Context, *common.Session, *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error)
}

type connector struct {
	Auth        common.Auth
	ClusterName string
	Domain      string
}

// Connect connects to the target SQL Server with Kerberos authentication.
func (c *connector) Connect(ctx context.Context, sessionCtx *common.Session, loginPacket *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error) {
	host, port, err := net.SplitHostPort(sessionCtx.Database.GetURI())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	portI, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsConfig, err := c.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Pass all login options from the client to the server.
	options := msdsn.LoginOptions{
		OptionFlags1: loginPacket.OptionFlags1(),
		OptionFlags2: loginPacket.OptionFlags2(),
		TypeFlags:    loginPacket.TypeFlags(),
	}

	auth, err := c.getAuth(sessionCtx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	connector := mssql.NewConnectorConfig(msdsn.Config{
		Host:         host,
		Port:         portI,
		Database:     sessionCtx.DatabaseName,
		LoginOptions: options,
		Encryption:   msdsn.EncryptionRequired,
		TLSConfig:    tlsConfig,
		PacketSize:   loginPacket.PacketSize(),
	}, auth)

	conn, err := connector.Connect(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	mssqlConn, ok := conn.(*mssql.Conn)
	if !ok {
		return nil, nil, trace.BadParameter("expected *mssql.Conn, got: %T", conn)
	}

	// Return all login flags returned by the server so that they can be passed
	// back to the client.
	return mssqlConn.GetUnderlyingConn(), mssqlConn.GetLoginFlags(), nil
}

func (s *connector) generateCredentials(ctx context.Context, username, domain string, ttl time.Duration) (certDER, keyDER []byte, err error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	san, err := subjectAltNameExtension(username, domain)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	csr := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: username},
		ExtraExtensions: []pkix.Extension{
			enhancedKeyUsageExtension,
			san,
		},
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, rsaKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	crlDN := s.crlDN()
	genResp, err := s.Auth.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{
		CSR:         csrPEM,
		CRLEndpoint: fmt.Sprintf("ldap:///%s?certificateRevocationList?base?objectClass=cRLDistributionPoint", crlDN),
		TTL:         proto.Duration(ttl),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return genResp.Cert, keyPEM, nil
}

var enhancedKeyUsageExtension = pkix.Extension{
	Id: enhancedKeyUsageExtensionOID,
	Value: func() []byte {
		val, err := asn1.Marshal([]asn1.ObjectIdentifier{
			clientAuthenticationOID,
			smartcardLogonOID,
		})
		if err != nil {
			panic(err)
		}
		return val
	}(),
}

var (
	// enhancedKeyUsageExtensionOID is the object identifier for a
	// certificate's enhanced key usage extension
	enhancedKeyUsageExtensionOID = asn1.ObjectIdentifier{2, 5, 29, 37}

	// subjectAltNameExtensionOID is the object identifier for a
	// certificate's subject alternative name extension
	subjectAltNameExtensionOID = asn1.ObjectIdentifier{2, 5, 29, 17}

	// clientAuthenticationOID is the object idnetifier that is used to
	// include client SSL authentication in a certificate's enhanced
	// key usage
	clientAuthenticationOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}

	// smartcardLogonOID is the object identifier that is used to include
	// smartcard login in a certificate's enhanced key usage
	smartcardLogonOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 2}

	// upnOtherNameOID is the object identifier that is used to include
	// the user principal name in a certificate's subject alternative name
	upnOtherNameOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 3}
)

func subjectAltNameExtension(user, domain string) (pkix.Extension, error) {
	// Setting otherName SAN according to
	// https://samfira.com/2020/05/16/golang-x-509-certificates-and-othername/
	//
	// othernName SAN is needed to pass the UPN of the user, per
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	ext := pkix.Extension{Id: subjectAltNameExtensionOID}
	var err error
	ext.Value, err = asn1.Marshal(
		subjectAltName{
			OtherName: otherName{
				OID: upnOtherNameOID,
				Value: upn{
					Value: fmt.Sprintf("%s@%s", user, domain),
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

type subjectAltName struct {
	OtherName otherName `asn1:"tag:0"`
}

type otherName struct {
	OID   asn1.ObjectIdentifier
	Value upn `asn1:"tag:0"`
}

type upn struct {
	Value string `asn1:"utf8"`
}

// crlDN generates the LDAP distinguished name (DN) where this Windows Service
// will publish its certificate revocation list
func (s *connector) crlDN() string {
	return "CN=" + s.ClusterName + "," + s.crlContainerDN()
}

// crlContainerDN generates the LDAP distinguished name (DN) of the container
// where the certificate revocation list is published
func (s *connector) crlContainerDN() string {
	return "CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration," + s.Domain
}
