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

package joinclient

import (
	"bytes"
	"context"
	"crypto/x509"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/cloud/imds/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/join/azurejoin"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/utils"
)

func azureJoin(ctx context.Context, stream messages.ClientStream, joinParams JoinParams, clientParams messages.ClientParams) (messages.Response, error) {
	// The Azure join method involves the following messages:
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server AzureInit
	// client<-server AzureChallenge
	// client->server AzureChallengeSolution
	// client<-server Result
	//
	// At this point the ServerInit messages has already been received, what's
	// left is to send the AzureInit message, handle the challenge-response, and
	// receive and return the final result.
	if err := stream.Send(&messages.AzureInit{
		ClientParams: clientParams,
	}); err != nil {
		return nil, trace.Wrap(err, "sending AzureInit")
	}

	challenge, err := messages.RecvResponse[*messages.AzureChallenge](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving AzureChallenge")
	}

	imds := joinParams.AzureParams.IMDSClient
	if imds == nil {
		imds = azure.NewInstanceMetadataClient()
	}
	if !imds.IsAvailable(ctx) {
		return nil, trace.AccessDenied("could not reach instance metadata. Is Teleport running on an Azure VM?")
	}
	ad, err := imds.GetAttestedData(ctx, challenge.Challenge)
	if err != nil {
		return nil, trace.Wrap(err, "getting attested data document")
	}
	intermediate, err := getIntermediateChain(ctx, joinParams.AzureParams.IssuerHTTPClient, ad)
	if err != nil {
		return nil, trace.Wrap(err, "getting intermediate CA for attested data")
	}
	accessToken, err := imds.GetAccessToken(ctx, joinParams.AzureParams.ClientID)
	if err != nil {
		return nil, trace.Wrap(err, "getting access token")
	}

	if err := stream.Send(&messages.AzureChallengeSolution{
		AttestedData: ad,
		Intermediate: intermediate,
		AccessToken:  accessToken,
	}); err != nil {
		return nil, trace.Wrap(err, "sending AzureChallengeSolution")
	}

	result, err := stream.Recv()
	return result, trace.Wrap(err, "receiving join result")
}

func getIntermediateChain(ctx context.Context, httpClient utils.HTTPDoClient, ad []byte) ([]byte, error) {
	_, p7, err := azurejoin.ParseAttestedData(ad)
	if err != nil {
		return nil, trace.Wrap(err, "parsing attested data document")
	}
	if len(p7.Certificates) == 0 {
		return nil, trace.Errorf("attested data signature has no certificates")
	}
	leafCert := p7.Certificates[0]
	if len(leafCert.IssuingCertificateURL) == 0 {
		return nil, trace.Errorf("attested data leaf certificate has no issuing certificate URL")
	}

	if httpClient == nil {
		httpClient, err = defaults.HTTPClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// mostly arbitrary, meant as a sanity check against infinite loops
	const maxDepth = 10
	// track which certificates we've seen to detect cycles
	seen := make(map[string]struct{})
	cert := leafCert
	var chainDER []byte
	for range maxDepth {
		if len(cert.IssuingCertificateURL) == 0 {
			break
		}

		url := cert.IssuingCertificateURL[0]
		if _, ok := seen[url]; ok {
			return nil, trace.Errorf("found cycle in intermediate chain")
		}
		seen[url] = struct{}{}
		issuer, err := fetchIssuerCert(ctx, httpClient, url)
		if err != nil {
			// failing to fetch may not guarantee an invalid state, so we log, break out
			// of the loop, and return the intermediates we've collected so far
			slog.WarnContext(ctx, "failed to fetch an issuing certificate while joining with azure", "error", err, "url", url)
			break
		}

		// we don't want to include the root in the chain, so we stop if we
		// find it
		if bytes.Equal(issuer.RawSubject, issuer.RawIssuer) {
			break
		}

		chainDER = append(chainDER, issuer.Raw...)
		cert = issuer
	}

	if len(chainDER) == 0 {
		return nil, trace.Errorf("attested data certificate has no intermediate chain")
	}
	return chainDER, nil
}

func fetchIssuerCert(ctx context.Context, httpClient utils.HTTPDoClient, issuerURL string) (*x509.Certificate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuerURL, nil /*body*/)
	if err != nil {
		return nil, trace.Wrap(err, "building HTTP request")
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "fetching intermediate certificate")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, trace.Errorf("failed to fetch intermediate cert, got HTTP status code %d", res.StatusCode)
	}

	body, err := utils.ReadAtMost(res.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err, "reading HTTP response body")
	}

	intermediates, err := x509.ParseCertificates(body)
	if err != nil {
		return nil, trace.Wrap(err, "parsing intermediate certificate")
	}

	if len(intermediates) != 1 {
		return nil, trace.Errorf("expected 1 intermediate, found %d", len(intermediates))
	}
	return intermediates[0], nil
}
