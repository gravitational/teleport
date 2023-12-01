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
	"crypto/x509"
	"crypto/x509/pkix"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GenerateWindowsDesktopCert generates client certificate for Windows RDP
// authentication.
func (a *Server) GenerateWindowsDesktopCert(ctx context.Context, req *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	if !modules.GetModules().Features().Desktop {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for desktop access, please contact the cluster administrator")
	}
	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName()
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
		// CRL is required for Windows smartcard certs.
		CRLDistributionPoints: []string{req.CRLEndpoint},
	}

	limitExceeded, err := a.desktopsLimitExceeded(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
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
