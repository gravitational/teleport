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
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strconv"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/winpki"
)

// GenerateWindowsDesktopCert generates client certificate for Windows RDP
// authentication.
func (a *Server) GenerateWindowsDesktopCert(ctx context.Context, req *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.Desktop).Enabled {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for desktop access, please contact the cluster administrator")
	}

	limitExceeded, err := a.desktopsLimitExceeded(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCert, signer, err := a.GetKeyStore().GetTLSCertAndSigner(ctx, userCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// See https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	// for cert requirements for Windows authn.
	certReq := tlsca.CertificateRequest{
		Clock:           a.clock,
		PublicKey:       csr.PublicKey,
		Subject:         csr.Subject,
		NotAfter:        a.clock.Now().UTC().Add(req.TTL.Get()),
		ExtraExtensions: csr.Extensions,
		KeyUsage:        x509.KeyUsageDigitalSignature,
	}

	// CRL Distribution Points (CDP) are required for Windows smartcard certs
	// for users wanting to RDP. They are not required for the service account
	// cert that Teleport itself uses to authenticate for LDAP.
	//
	// The CDP is computed here by the auth server issuing the cert and not provided
	// by the client because the CDP is based on the identity of the issuer, which is
	// necessary in order to support clusters with multiple issuing certs (HSMs).
	if req.CRLDomain != "" {
		cdp := winpki.CRLDistributionPoint(
			req.CRLDomain,
			types.UserCA,
			tlsCA,
		)
		certReq.CRLDistributionPoints = []string{cdp}
	} else if req.CRLEndpoint != "" {
		// legacy clients will specify CRL endpoint instead of CRL domain
		// DELETE IN v20 (zmb3)
		certReq.CRLDistributionPoints = []string{req.CRLEndpoint}
		a.logger.DebugContext(ctx, "Generating Windows desktop cert with legacy CDP")
	}

	certReq.ExtraExtensions = append(certReq.ExtraExtensions, pkix.Extension{
		Id:    tlsca.LicenseOID,
		Value: []byte(modules.GetModules().BuildType()),
	}, pkix.Extension{
		Id:    tlsca.DesktopsLimitExceededOID,
		Value: []byte(strconv.FormatBool(limitExceeded)),
	})
	cert, err := tlsCA.GenerateCertificate(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.WindowsDesktopCertResponse{
		Cert: cert,
	}, nil
}

// desktopAccessConfigureScript is the script that will run on the windows
// machine and configure Active Directory
//
//go:embed windows-configure-ad.ps1
var desktopAccessScriptConfigure string
var DesktopAccessScriptConfigure = template.Must(template.New("desktop-access-configure-ad").Parse(desktopAccessScriptConfigure))

func (a *Server) GetDesktopBootstrapScript(ctx context.Context) (*proto.DesktopBootstrapScriptResponse, error) {
	clusterName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certAuthority, err := a.GetCertAuthority(
		ctx,
		types.CertAuthID{Type: types.UserCA, DomainName: clusterName},
		false,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(certAuthority.GetActiveKeys().TLS) != 1 {
		return nil, trace.BadParameter("expected one TLS key pair, got %v", len(certAuthority.GetActiveKeys().TLS))
	}

	keyPair := certAuthority.GetActiveKeys().TLS[0]
	block, _ := pem.Decode(keyPair.Cert)
	if block == nil {
		return nil, trace.BadParameter("no PEM data in CA data")
	}

	var buf bytes.Buffer
	err = DesktopAccessScriptConfigure.Execute(&buf, map[string]string{
		"caCertPEM":    string(keyPair.Cert),
		"caCertSHA1":   fmt.Sprintf("%X", sha1.Sum(block.Bytes)),
		"caCertBase64": base64.StdEncoding.EncodeToString(utils.CreateCertificateBLOB(block.Bytes)),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.DesktopBootstrapScriptResponse{
		Script: buf.String(),
	}, nil
}
