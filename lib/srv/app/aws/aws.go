/*
Copyright 2021 Gravitational, Inc.

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

package appaws

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
)

const (
	// AmazonSigV4AuthorizationPrefix is AWS Authorization prefix indicating that the request
	// was signed by AWS Signature Version 4.
	// https://github.com/aws/aws-sdk-go/blob/main/aws/signer/v4/v4.go#L83
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html
	AmazonSigV4AuthorizationPrefix = "AWS4-HMAC-SHA256"

	// AmzDateTimeFormat is time format used in X-Amz-Date header.
	// https://github.com/aws/aws-sdk-go/blob/main/aws/signer/v4/v4.go#L84
	AmzDateTimeFormat = "20060102T150405Z"

	// AmzDateHeader header name containing timestamp when signature was generated.
	// https://docs.aws.amazon.com/general/latest/gr/sigv4-date-handling.html
	AmzDateHeader = "X-Amz-Date"

	authorizationHeader        = "Authorization"
	credentialAuthHeaderElem   = "Credential"
	signedHeaderAuthHeaderElem = "SignedHeaders"
	signatureAuthHeaderElem    = "Signature"
)

// SigV4 contains parsed content of the AWS Authorization header.
type SigV4 struct {
	// KeyIS is an AWS access-key-id
	KeyID string
	// Date value is specified using YYYYMMDD format.
	Date string
	// Region is an AWS Region.
	Region string
	// Service is an AWS Service.
	Service string
	// SignedHeaders is a  list of request headers that you used to compute Signature.
	SignedHeaders []string
	// The 256-bit Signature of the request.
	Signature string
}

// ParseSigV4 AWS SigV4 credentials string sections.
// AWS SigV4 header example:
// Authorization: AWS4-HMAC-SHA256
// Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,
// SignedHeaders=host;range;x-amz-date,
// Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
func ParseSigV4(header string) (*SigV4, error) {
	if header == "" {
		return nil, trace.BadParameter("empty header")
	}
	sectionParts := strings.Split(header, " ")

	m := make(map[string]string)
	for _, v := range sectionParts {
		kv := strings.Split(v, "=")
		if len(kv) != 2 {
			continue
		}
		m[kv[0]] = strings.TrimSuffix(kv[1], ",")
	}

	authParts := strings.Split(m[credentialAuthHeaderElem], "/")
	if len(authParts) != 5 {
		return nil, trace.BadParameter("invalid size of %q section", credentialAuthHeaderElem)
	}

	signature := m[signatureAuthHeaderElem]
	if signature == "" {
		return nil, trace.BadParameter("invalid signature")
	}
	var signedHeaders []string
	if v := m[signedHeaderAuthHeaderElem]; v != "" {
		signedHeaders = strings.Split(v, ";")
	}

	return &SigV4{
		KeyID:     authParts[0],
		Date:      authParts[1],
		Region:    authParts[2],
		Service:   authParts[3],
		Signature: signature,
		// Split semicolon-separated list of signed headers string.
		// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html
		// https://github.com/aws/aws-sdk-go/blob/main/aws/signer/v4/v4.go#L631
		SignedHeaders: signedHeaders,
	}, nil
}

// IsSignedByAWSSigV4 checks is the request was signed by AWS Signature Version 4 algorithm.
// https://docs.aws.amazon.com/general/latest/gr/signing_aws_api_requests.html
func IsSignedByAWSSigV4(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get(authorizationHeader), AmazonSigV4AuthorizationPrefix)
}

// GetAndReplaceReqBody returns the request and replace the drained body reader with io.NopCloser
// allowing for further body processing by http transport.
func GetAndReplaceReqBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return []byte{}, nil
	}
	// req.Body is closed during drainBody call.
	payload, err := drainBody(req.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Replace the drained body with io.NopCloser reader allowing for further request processing by HTTP transport.
	req.Body = io.NopCloser(bytes.NewReader(payload))
	return payload, nil
}

// drainBody drains the body,  close the reader and returns the read bytes.
func drainBody(b io.ReadCloser) ([]byte, error) {
	payload, err := io.ReadAll(b)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = b.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return payload, nil
}
