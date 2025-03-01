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

package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/utils"
)

// SigningCtx contains AWS SigV4 signing context parameters.
type SigningCtx struct {
	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// Credentials provides AWS credentials.
	Credentials aws.CredentialsProvider
	// SigningName is the AWS signing service name.
	SigningName string
	// SigningRegion is the AWS region to sign a request for.
	SigningRegion string
}

// Check checks signing context parameters.
func (sc *SigningCtx) Check() error {
	switch {
	case sc.Credentials == nil:
		return trace.BadParameter("missing AWS credentials")
	case sc.SigningName == "":
		return trace.BadParameter("missing AWS signing name")
	case sc.SigningRegion == "":
		return trace.BadParameter("missing AWS signing region")
	}
	return nil
}

// SignRequest creates a new HTTP request and rewrites the header from the original request and returns a new
// HTTP request signed by STS AWS API.
// Signing steps:
// 1) Decode Authorization Header. Authorization Header example:
//
//		Authorization: AWS4-HMAC-SHA256
//		Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,
//		SignedHeaders=host;range;x-amz-date,
//		Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
//
//	 2. Extract credential section from credential Authorization Header.
//	 3. Extract aws-region and aws-service from the credential section.
//	 4. Build AWS API endpoint based on extracted aws-region and aws-service fields.
//	    Not that for endpoint resolving the https://github.com/aws/aws-sdk-go/aws/endpoints/endpoints.go
//	    package is used and when Amazon releases a new API the dependency update is needed.
//	 5. Sign HTTP request.
func SignRequest(ctx context.Context, req *http.Request, signCtx *SigningCtx) (*http.Request, error) {
	if signCtx == nil {
		return nil, trace.BadParameter("missing signing context")
	}
	if err := signCtx.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	payload, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqCopy := req.Clone(ctx)
	reqCopy.Body = io.NopCloser(req.Body)

	// Only keep the headers signed in the original request for signing. This
	// not only avoids signing extra headers injected by Teleport along the
	// way, but also preserves the signing logic of the original AWS client.
	//
	// For example, Athena ODBC driver sends query requests with "Expect:
	// 100-continue" headers without being signed, otherwise the Athena service
	// would reject the requests.
	unsignedHeaders := removeUnsignedHeaders(reqCopy)
	signer := NewSignerV2(signCtx.Credentials, signCtx.SigningName)
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), signCtx.SigningName, signCtx.SigningRegion, signCtx.Clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// copy removed headers back to the request after signing it, but don't copy the old Authorization header.
	copyHeaders(reqCopy, req, utils.RemoveFromSlice(unsignedHeaders, "Authorization"))
	return reqCopy, nil
}

// removeUnsignedHeaders removes and returns header keys that are not included in SigV4 SignedHeaders.
// If the request is not already signed, then no headers are removed.
func removeUnsignedHeaders(reqCopy *http.Request) []string {
	// check if the request is already signed.
	authHeader := reqCopy.Header.Get("Authorization")
	sig, err := ParseSigV4(authHeader)
	if err != nil {
		return nil
	}
	return filterHeaders(reqCopy, sig.SignedHeaders)
}

// copyHeaders copies headers from src request to dst request, using a list of header keys to copy.
func copyHeaders(dst *http.Request, src *http.Request, keys []string) {
	for _, k := range keys {
		if vals, ok := src.Header[k]; ok {
			dst.Header[k] = vals
		}
	}
}
