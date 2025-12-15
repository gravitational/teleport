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

// Package oracle includes methods required for clients to join a Teleport
// cluster using the Oracle join method.
package oracle

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/utils"
)

// SolveChallenge solves an Oracle join challenge.
func SolveChallenge(ctx context.Context, httpClient utils.HTTPDoClient, challenge *messages.OracleChallenge) (*messages.OracleChallengeSolution, error) {
	signedRootCAReq, err := makeSignedRootCAReq(httpClient)
	if err != nil {
		return nil, trace.Wrap(err, "making signed root CA request")
	}

	cert, err := fetchIMDS(ctx, httpClient, "cert.pem")
	if err != nil {
		return nil, trace.Wrap(err, "fetching instance cert from IMDS")
	}

	intermediate, err := fetchIMDS(ctx, httpClient, "intermediate.pem")
	if err != nil {
		return nil, trace.Wrap(err, "fetching intermediate CA certs from IMDS")
	}

	keyPEM, err := fetchIMDS(ctx, httpClient, "key.pem")
	if err != nil {
		return nil, trace.Wrap(err, "fetching instance private key from IMDS")
	}

	key, err := keys.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, trace.Wrap(err, "parsing instance private key")
	}

	signature, err := SignChallenge(key.Signer, challenge.Challenge)
	if err != nil {
		return nil, trace.Wrap(err, "signing challenge")
	}

	return &messages.OracleChallengeSolution{
		Cert:            cert,
		Intermediate:    intermediate,
		Signature:       signature,
		SignedRootCAReq: signedRootCAReq,
	}, nil
}

// SignChallenge signs a challenge with a given instance private key.
func SignChallenge(key crypto.Signer, challenge string) ([]byte, error) {
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("only RSA keys are supported")
	}
	digest := sha256.Sum256([]byte(challenge))
	pssOpts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}
	signature, err := rsa.SignPSS(rand.Reader, rsaKey, crypto.SHA256, digest[:], pssOpts)
	return signature, trace.Wrap(err)
}

func makeSignedRootCAReq(client utils.HTTPDoClient) ([]byte, error) {
	provider, err := auth.InstancePrincipalConfigurationProviderWithCustomClient(
		func(dispatcher common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error) {
			return client, nil
		})
	if err != nil {
		return nil, trace.Wrap(err, "making oracle config provider")
	}

	localRegion, err := provider.Region()
	if err != nil {
		return nil, trace.Wrap(err, "finding local OCI region")
	}
	regionalAuthEndpoint := common.StringToRegion(localRegion).Endpoint("auth")
	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: "http",
			Host:   regionalAuthEndpoint,
			Path:   "/v1/instancePrincipalRootCACertificates",
		},
		Header: http.Header{
			"Date": []string{time.Now().UTC().Format(http.TimeFormat)},
		},
	}

	signer := common.DefaultRequestSigner(provider)
	if err := signer.Sign(req); err != nil {
		return nil, trace.Wrap(err, "signing request")
	}

	reqBytes, err := httputil.DumpRequestOut(req, false /*body*/)
	if err != nil {
		return nil, trace.Wrap(err, "dumping HTTP request to bytes")
	}

	return reqBytes, nil
}

// fetchIMDS fetches instance metadata from Oracle's IMDSv2, which is available
// on every OCI instance https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/gettingmetadata.htm
func fetchIMDS(ctx context.Context, client utils.HTTPDoClient, path string) ([]byte, error) {
	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: "http",
			Host:   "169.254.169.254",
			Path:   "/opc/v2/identity/" + path,
		},
		Header: http.Header{
			// This header is required on all IMDSv2 requests.
			"Authorization": []string{"Bearer Oracle"},
		},
	}
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "sending request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, teleport.MaxHTTPResponseSize))
	return body, trace.Wrap(err, "reading response body")
}
