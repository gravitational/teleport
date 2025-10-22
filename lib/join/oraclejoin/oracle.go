// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oraclejoin

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/utils"
)

// CheckChallengeSolutionParams holds parameters necessary to check an Oracle
// join challenge solution.
type CheckChallengeSolutionParams struct {
	// Challenge is the challenge string that was issued to the client.
	Challenge string
	// Solution is the client's full solution to the challenge.
	Solution *messages.OracleChallengeSolution
	// ProvisionToken is the token being used for the request.
	ProvisionToken provision.Token
	// RootCACache caches Oracle root CAs per region.
	RootCACache *RootCACache
	// HTTPClient (optional) is an HTTP client that will be used to send
	// requests to the Oracle API.
	HTTPClient utils.HTTPDoClient
}

func (p *CheckChallengeSolutionParams) checkAndSetDefaults() error {
	switch {
	case p.Challenge == "":
		return trace.BadParameter("Challenge is required")
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case len(p.Solution.Cert) == 0:
		return trace.BadParameter("Cert is required")
	case len(p.Solution.Intermediate) == 0:
		return trace.BadParameter("Intermediate is required")
	case len(p.Solution.Signature) == 0:
		return trace.BadParameter("Signature is required")
	case len(p.Solution.SignedRootCAReq) == 0:
		return trace.BadParameter("SignedRootCAReq is required")
	case p.RootCACache == nil:
		return trace.BadParameter("RootCACache is required")
	}
	if p.HTTPClient == nil {
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		p.HTTPClient = httpClient
	}
	return nil
}

// CheckChallengeSolution checks an Oracle join challenge solution, and returns
// an error if the solution cannot be verified. It returns instance claims, and
// may return claims even when returning an error, which may be useful for
// identifying an agent that is failing to join.
func CheckChallengeSolution(ctx context.Context, params *CheckChallengeSolutionParams) (*Claims, error) {
	if err := params.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "checking request parameters")
	}

	identityCert, err := parseIdentityCert(params.Solution.Cert)
	if err != nil {
		return nil, trace.Wrap(err, "parsing identity certificate")
	}

	claims, err := parseClaims(identityCert)
	if err != nil {
		return nil, trace.Wrap(err, "parsing claims from identity certificate")
	}

	if err := verifyChallengeSignature(identityCert, []byte(params.Challenge), params.Solution.Signature); err != nil {
		return claims, trace.Wrap(err)
	}

	intermediateCAPool, err := makeIntermediateCAPool(params.Solution.Intermediate)
	if err != nil {
		return claims, trace.Wrap(err, "parsing intermediate CA certificates")
	}

	rootCAPool, err := makeRootCAPool(ctx, params, claims.InstanceID)
	if err != nil {
		return claims, trace.Wrap(err, "building Oracle root CA pool")
	}

	if _, err := identityCert.Verify(x509.VerifyOptions{
		Intermediates: intermediateCAPool,
		Roots:         rootCAPool,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return claims, trace.AccessDenied("instance certificate chain did not verify to Oracle root CAs")
	}

	if err := CheckOracleAllowRules(claims, params.ProvisionToken); err != nil {
		return claims, trace.Wrap(err)
	}

	return claims, nil
}

func parseIdentityCert(cert []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(cert)
	if block == nil {
		return nil, trace.BadParameter("no PEM block found")
	}
	if block.Type != "CERTIFICATE" {
		return nil, trace.BadParameter("expected PEM block of type CERTIFICATE, found %s", block.Type)
	}
	c, err := x509.ParseCertificate(block.Bytes)
	return c, trace.Wrap(err, "parsing certificate DER")
}

func parseClaims(cert *x509.Certificate) (*Claims, error) {
	var claims Claims
	for _, ou := range cert.Subject.OrganizationalUnit {
		chunks := strings.SplitN(ou, ":", 2)
		if len(chunks) < 2 {
			continue
		}
		switch chunks[0] {
		case "opc-instance":
			claims.InstanceID = chunks[1]
		case "opc-compartment":
			claims.CompartmentID = chunks[1]
		case "opc-tenant":
			claims.TenancyID = chunks[1]
		case "opc-certtype":
			if chunks[1] != "instance" {
				return nil, trace.BadParameter("opc-certtype is not instance, found: %s", chunks[1])
			}
		}
	}
	switch {
	case claims.InstanceID == "":
		return nil, trace.BadParameter("no InstanceID found")
	case claims.CompartmentID == "":
		return nil, trace.BadParameter("no CompartmentID found")
	case claims.TenancyID == "":
		return nil, trace.BadParameter("no TenancyID found")
	}
	return &claims, nil
}

func verifyChallengeSignature(identityCert *x509.Certificate, challenge, signature []byte) error {
	switch pub := identityCert.PublicKey.(type) {
	case *rsa.PublicKey:
		return trace.Wrap(verifyRSAChallengeSignature(pub, challenge, signature))
	default:
		return trace.AccessDenied("unsupported certificate key type %s", identityCert.PublicKeyAlgorithm.String())
	}
}

func verifyRSAChallengeSignature(pub *rsa.PublicKey, challenge, signature []byte) error {
	// pub.Size returns the key size in bytes, we require a 2048-bit key.
	if pub.Size() < 2048/8 {
		return trace.AccessDenied("instance key: RSA key is too small (minimum 2048 bits)")
	}
	if pub.Size() > 4096/8 {
		return trace.AccessDenied("instance key: RSA key is too large (maximum 4096 bits)")
	}
	digest := sha256.Sum256(challenge)
	if err := rsa.VerifyPSS(pub, crypto.SHA256, digest[:], signature, &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthAuto,
		Hash:       crypto.SHA256,
	}); err != nil {
		return trace.AccessDenied("RSA challenge signature did not verify")
	}
	return nil
}

func makeIntermediateCAPool(intermediateCAPEM []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(intermediateCAPEM) {
		return nil, trace.BadParameter("failed to parse any intermediate CAs")
	}
	return pool, nil
}

func makeRootCAPool(ctx context.Context, params *CheckChallengeSolutionParams, instanceID string) (*x509.CertPool, error) {
	rootCAReq, err := parseRootCAReq(params.Solution.SignedRootCAReq)
	if err != nil {
		return nil, trace.Wrap(err, "parsing signed root CA request")
	}

	region, err := validateRootCAReq(rootCAReq, instanceID)
	if err != nil {
		return nil, trace.Wrap(err, "validating root CA request")
	}

	rootCAPool, err := params.RootCACache.get(ctx, region, func() (*x509.CertPool, time.Time, error) {
		return getRootCAPool(ctx, params.HTTPClient, rootCAReq)
	})
	return rootCAPool, trace.Wrap(err)
}

func parseRootCAReq(req []byte) (*http.Request, error) {
	httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
	if err != nil {
		return nil, trace.Wrap(err, "parsing HTTP request")
	}

	// Unset RequestURI and set URL (necessary quirk of sending a request parsed by
	// http.ReadRequest)
	httpReq.RequestURI = ""
	httpReq.URL.Scheme = "https"
	httpReq.URL.Host = httpReq.Host
	// If the HTTP client sees this header set to gzip it will expect the
	// caller to handle gzip itself instead of decoding automatically.
	delete(httpReq.Header, "Accept-Encoding")
	return httpReq, nil
}

func validateRootCAReq(req *http.Request, instanceID string) (string, error) {
	const rootCAPath = "/v1/instancePrincipalRootCACertificates"
	if req.URL.Path != rootCAPath {
		return "", trace.BadParameter("path must be %s, got %s", rootCAPath, req.URL.Path)
	}
	expectedRegion, err := regionFromInstanceID(instanceID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	expectedHostname := expectedRegion.Endpoint("auth")
	if req.URL.Hostname() != expectedHostname {
		return "", trace.BadParameter("hostname must be %s, got %s", expectedHostname, req.URL.Hostname())
	}
	return string(expectedRegion), nil
}

// regionFromInstanceID returns an oci region for a given instance ID. It will
// return an error if the region name is not recognized by the SDK. This is a
// critical check since this region will be used to validate the endpoint that
// will be used to fetch the regional root CAs. If a bogus region were allowed
// then teleport could be tricked into making a request to
// auth.<bogus-region-name>.oraclecloud.com with potential tricks in
// <bogus-region-name>.
func regionFromInstanceID(instanceID string) (common.Region, error) {
	// Expected InstanceID format ocid1.instance.<realm>.[region][.future-use].<id>
	chunks := strings.SplitN(instanceID, ".", 5)
	if len(chunks) < 5 {
		return "", trace.BadParameter("instance ID does not have expected format, got %s", instanceID)
	}
	regionShortName := chunks[3]
	region := common.StringToRegion(regionShortName)
	if _, err := region.RealmID(); err != nil {
		// StringToRegion always returns something, RealmID will return an
		// error if it's not a real region.
		return "", trace.AccessDenied("unsupported region %s", regionShortName)
	}
	return region, nil
}

func getRootCAPool(ctx context.Context, httpClient utils.HTTPDoClient, rootCAReq *http.Request) (*x509.CertPool, time.Time, error) {
	resp, err := executeRootCAReq(ctx, httpClient, rootCAReq)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err, "executing Oracle root CA request")
	}
	rootCAPool, err := parseRootCAPool(resp)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err, "parsing Oracle root CA pool")
	}
	expires, err := time.Parse(time.RFC3339, resp.RefreshIn)
	if err != nil {
		expires = time.Now().Add(5 * time.Minute)
	}
	return rootCAPool, expires, nil
}

func executeRootCAReq(ctx context.Context, client utils.HTTPDoClient, req *http.Request) (*rootCAResp, error) {
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "sending request to Oracle API")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, teleport.MaxHTTPResponseSize))
	if err != nil {
		return nil, trace.Wrap(err, "reading response body")
	}

	var parsedResp rootCAResp
	if err := json.Unmarshal(body, &parsedResp); err != nil {
		return nil, trace.Wrap(err, "parsing response")
	}
	return &parsedResp, nil
}

// rootCAResp models the JSON structure returned by Oracle's
// /v1/instancePrincipalRootCACertificates API.
type rootCAResp struct {
	// Certificates is expected to be a list of base64-encoded ASN.1 DER X509
	// certificates.
	Certificates []string `json:"certificates"`
	// RefreshIn is an RFC3339 timestamp indicating when the CA should be re-fetched.
	RefreshIn string `json:"refreshIn"`
}

func parseRootCAPool(resp *rootCAResp) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, certBase64 := range resp.Certificates {
		bytes, err := base64.StdEncoding.DecodeString(certBase64)
		if err != nil {
			return nil, trace.Wrap(err, "decoding cert base64")
		}
		cert, err := x509.ParseCertificate(bytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing x509 certificate")
		}
		pool.AddCert(cert)
	}
	return pool, nil
}
