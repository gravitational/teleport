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
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"slices"
	"strconv"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/subca"
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
	if err := validateWindowsDesktopCSR(csr); err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caID := types.CertAuthID{
		Type:       types.WindowsCA,
		DomainName: clusterName.GetClusterName(),
	}
	if !req.SupportsWindowsCA {
		var callerID string
		if ig, err := authz.UserFromContext(ctx); err == nil {
			callerID = ig.GetIdentity().Username
		}
		a.logger.WarnContext(ctx,
			""+
				"Windows Desktop Service caller does not support the WindowsCA. "+
				"Issuing certificates using the UserCA.",
			"caller_id", callerID,
		)
		caID.Type = types.UserCA
	}
	ca, err := a.GetCertAuthority(ctx, caID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caCert, signer, err := a.GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Calculate CA override.
	subCAResolver, err := a.loadCAOverrideResolverForCA(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	overrideResult, err := subCAResolver.CalculateOverride(subca.Certificate{PEM: caCert})
	if err != nil {
		return nil, trace.Wrap(err, "calculate CA override")
	}
	caCert = overrideResult.CACertificate.PEM

	tlsCA, err := tlsca.FromCertAndSigner(caCert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Our CSR may contains non-standard subject OIDs that
	// we need to preserve, but de-serializing the CSR puts those fields
	// in csr.Subject.Names which is ignored when re-serializing
	// (See documentation on Names and ExtraNames fields of [pkix.Name])
	// Copy the Names -> ExtraNames but be warned - The values in `ExtraNames`
	// take precedence over standard Subject values (CommonName, Country, Organization, etc.)
	// in the pkix.Name struct. If you modified any of these values after parsing the CSR,
	// those modifications will be LOST upon cert generation.
	// If you need to modify the original CSR's subject in this function, then we must
	// take care to prune standard OIDs for these fields from csr.Subject.Names first.
	if len(csr.Subject.Names) > 0 {
		csr.Subject.ExtraNames = csr.Subject.Names
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
		cdp, err := winpki.CRLDistributionPoint(req.CRLDomain, caID.Type, tlsCA, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certReq.CRLDistributionPoints = []string{cdp}
	} else if req.CRLEndpoint != "" {
		// Legacy clients will specify CRL endpoint instead of CRL domain.
		// DELETE IN v20 (zmb3): v19 clients will no longer be setting CRLEndpoint
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
	if err := validateGeneratedWindowsDesktopCertificate(cert); err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.WindowsDesktopCertResponse{
		Cert:       cert,
		CaOverride: overrideResult.ToClientOverrideDetailsProto(),
	}, nil
}

// validateWindowsDesktopCSR validates all caller-controlled certificate fields
// before they are copied into a certificate. Windows desktop certificates before
// v18.7.0 are signed by the User CA, so they must not contain fields that Teleport
// can interpret as user identity or authorization.
func validateWindowsDesktopCSR(csr *x509.CertificateRequest) error {
	if csr == nil {
		return trace.BadParameter("missing certificate signing request")
	}
	if err := csr.CheckSignature(); err != nil {
		return trace.Wrap(err, "invalid CSR signature")
	}

	// The entire subject is copied into the certificate, so it cannot contain
	// an attribute that Teleport decodes as identity or authorization:
	// roles, scope pins, traits, etc.
	//
	// A certificate signed by the User CA carrying one of these could be used
	// as a Teleport user certificate.
	for _, name := range slices.Concat(csr.Subject.Names, csr.Subject.ExtraNames) {
		if tlsca.IsIdentityOID(name.Type) {
			return trace.BadParameter("Windows desktop certificate subject must not contain Teleport identity attribute %q", name.Type)
		}
	}

	seen := make(map[string]struct{}, len(csr.Extensions))
	var hasEnhancedKeyUsage, hasSubjectAltName bool
	for _, extension := range csr.Extensions {
		oid := extension.Id.String()
		if _, ok := seen[oid]; ok {
			return trace.BadParameter("duplicate Windows desktop certificate extension %q", oid)
		}
		seen[oid] = struct{}{}
		if extension.Critical {
			return trace.BadParameter("Windows desktop certificate extension %q must not be critical", oid)
		}

		switch {
		case extension.Id.Equal(tlsca.LicenseOID), extension.Id.Equal(tlsca.DesktopsLimitExceededOID):
			return trace.BadParameter("Windows desktop certificate extension %q is controlled by the Auth Service", oid)
		case extension.Id.Equal(winpki.EnhancedKeyUsageExtensionOID):
			if err := validateWindowsEnhancedKeyUsage(extension.Value); err != nil {
				return trace.Wrap(err)
			}
			hasEnhancedKeyUsage = true
		case extension.Id.Equal(winpki.SubjectAltNameExtensionOID):
			if err := validateWindowsSubjectAltName(extension.Value); err != nil {
				return trace.Wrap(err)
			}
			hasSubjectAltName = true
		case extension.Id.Equal(winpki.ADUserMappingExtensionOID):
			if err := validateWindowsADUserMapping(extension.Value); err != nil {
				return trace.Wrap(err)
			}
		case extension.Id.Equal(tlsca.CreateWindowsUserOID):
			// OK, allowed
		case extension.Id.Equal(tlsca.ADStatusOID):
			if !bytes.Equal(extension.Value, []byte("AD")) {
				return trace.BadParameter("invalid Windows desktop AD status extension")
			}
		default:
			return trace.BadParameter("unsupported Windows desktop certificate extension %q", oid)
		}
	}

	if !hasEnhancedKeyUsage {
		return trace.BadParameter("Windows desktop certificate request is missing enhanced key usage")
	}
	if !hasSubjectAltName {
		return trace.BadParameter("Windows desktop certificate request is missing subject alternative name")
	}
	return nil
}

func validateWindowsEnhancedKeyUsage(value []byte) error {
	var usages []asn1.ObjectIdentifier
	rest, err := asn1.Unmarshal(value, &usages)
	if err != nil || len(rest) != 0 {
		return trace.BadParameter("invalid Windows desktop enhanced key usage extension")
	}
	if len(usages) != 2 ||
		!slices.ContainsFunc(usages, winpki.ClientAuthenticationOID.Equal) ||
		!slices.ContainsFunc(usages, winpki.SmartcardLogonOID.Equal) {
		return trace.BadParameter("Windows desktop certificate must request exactly client authentication and smart-card logon usages")
	}
	return nil
}

func validateWindowsSubjectAltName(value []byte) error {
	var san winpki.SubjectAltName[winpki.UPN]
	rest, err := asn1.Unmarshal(value, &san)
	if err != nil || len(rest) != 0 ||
		!san.OtherName.OID.Equal(winpki.UPNOtherNameOID) ||
		san.OtherName.Value.Value == "" {
		return trace.BadParameter("invalid Windows desktop UPN subject alternative name")
	}
	return nil
}

func validateWindowsADUserMapping(value []byte) error {
	var mapping winpki.SubjectAltName[winpki.ADSid]
	rest, err := asn1.Unmarshal(value, &mapping)
	if err != nil || len(rest) != 0 ||
		!mapping.OtherName.OID.Equal(winpki.ADUserMappingInternalOID) ||
		len(mapping.OtherName.Value.Value) == 0 {
		return trace.BadParameter("invalid Windows desktop AD user mapping extension")
	}
	return nil
}

// validateGeneratedWindowsDesktopCertificate asserts that the signed desktop
// certificate contains the desktop limit extension.
//
// Authentication middleware relies on this extension to prevent smart-card
// certificates signed by the User CA from being used as Teleport client credentials.
func validateGeneratedWindowsDesktopCertificate(certPEM []byte) error {
	cert, err := tlsca.ParseCertificatePEM(certPEM)
	if err != nil {
		return trace.Wrap(err, "parsing generated Windows desktop certificate")
	}
	count := 0
	for _, extension := range cert.Extensions {
		if extension.Id.Equal(tlsca.DesktopsLimitExceededOID) {
			count++
		}
	}
	if count != 1 {
		return trace.Errorf("generated Windows desktop certificate contains %d desktop extensions, expected exactly 1", count)
	}
	return nil
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
		types.CertAuthID{Type: types.WindowsCA, DomainName: clusterName},
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
