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

package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/textproto"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
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

	// AmzDateHeader is header name containing timestamp when signature was generated.
	// https://docs.aws.amazon.com/general/latest/gr/sigv4-date-handling.html
	AmzDateHeader = "X-Amz-Date"

	AuthorizationHeader        = "Authorization"
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
	// Signature is the 256-bit Signature of the request.
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
		return nil, trace.BadParameter("empty AWS SigV4 header")
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
	return strings.HasPrefix(r.Header.Get(AuthorizationHeader), AmazonSigV4AuthorizationPrefix)
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

// drainBody drains the body, close the reader and returns the read bytes.
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

// VerifyAWSSignature verifies the request signature ensuring that the request originates from tsh aws command execution
// AWS CLI signs the request with random generated credentials that are passed to LocalProxy by
// the AWSCredentials LocalProxyConfig configuration.
func VerifyAWSSignature(req *http.Request, credentials *credentials.Credentials) error {
	sigV4, err := ParseSigV4(req.Header.Get("Authorization"))
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	// Read the request body and replace the body ready with a new reader that will allow reading the body again
	// by HTTP Transport.
	payload, err := GetAndReplaceReqBody(req)
	if err != nil {
		return trace.Wrap(err)
	}

	reqCopy := req.Clone(context.Background())

	// Remove all the headers that are not present in awsCred.SignedHeaders.
	filterHeaders(reqCopy, sigV4.SignedHeaders)

	// Get the date that was used to create the signature of the original request
	// originated from AWS CLI and reuse it as a timestamp during request signing call.
	t, err := time.Parse(AmzDateTimeFormat, reqCopy.Header.Get(AmzDateHeader))
	if err != nil {
		return trace.BadParameter(err.Error())
	}

	signer := NewSigner(credentials, sigV4.Service)
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), sigV4.Service, sigV4.Region, t)
	if err != nil {
		return trace.Wrap(err)
	}

	localSigV4, err := ParseSigV4(reqCopy.Header.Get("Authorization"))
	if err != nil {
		return trace.Wrap(err)
	}

	// Compare the origin request AWS SigV4 signature with the signature calculated in LocalProxy based on
	// AWSCredentials taken from LocalProxyConfig.
	if sigV4.Signature != localSigV4.Signature {
		return trace.AccessDenied("signature verification failed")
	}
	return nil
}

// NewSigner creates a new V4 signer.
func NewSigner(credentials *credentials.Credentials, signingServiceName string) *v4.Signer {
	options := func(s *v4.Signer) {
		// s3 and s3control requests are signed with URL unescaped (found by
		// searching "DisableURIPathEscaping" in "aws-sdk-go/service"). Both
		// services use "s3" as signing name. See description of
		// "DisableURIPathEscaping" for more details.
		if signingServiceName == "s3" {
			s.DisableURIPathEscaping = true
		}
	}
	return v4.NewSigner(credentials, options)
}

// filterHeaders removes request headers that are not in the headers list.
func filterHeaders(r *http.Request, headers []string) {
	out := make(http.Header)
	for _, v := range headers {
		ck := textproto.CanonicalMIMEHeaderKey(v)
		val, ok := r.Header[ck]
		if ok {
			out[ck] = val
		}
	}
	r.Header = out
}

// FilterAWSRoles returns role ARNs from the provided list that belong to the
// specified AWS account ID.
//
// If AWS account ID is empty, all roles are returned.
func FilterAWSRoles(arns []string, accountID string) (result Roles) {
	for _, roleARN := range arns {
		parsed, err := arn.Parse(roleARN)
		if err != nil || (accountID != "" && parsed.AccountID != accountID) {
			continue
		}

		// In AWS convention, the display of the role is the last
		// /-delineated substring.
		//
		// Example ARNs:
		// arn:aws:iam::1234567890:role/EC2FullAccess      (display: EC2FullAccess)
		// arn:aws:iam::1234567890:role/path/to/customrole (display: customrole)
		parts := strings.Split(parsed.Resource, "/")
		numParts := len(parts)
		if numParts < 2 || parts[0] != "role" {
			continue
		}
		result = append(result, AWSRole{
			Name:    strings.Join(parts[1:], "/"),
			Display: parts[numParts-1],
			ARN:     roleARN,
		})
	}
	return result
}

// AWSRole describes an AWS IAM role for AWS console access.
type AWSRole struct {
	// Name is the full role name with the entire path.
	Name string `json:"name"`
	// Display is the role display name.
	Display string `json:"display"`
	// ARN is the full role ARN.
	ARN string `json:"arn"`
}

// Roles is a slice of roles.
type Roles []AWSRole

// Sort sorts the roles by their display names.
func (roles Roles) Sort() {
	sort.SliceStable(roles, func(x, y int) bool {
		return strings.ToLower(roles[x].Display) < strings.ToLower(roles[y].Display)
	})
}

// FindRoleByARN finds the role with the provided ARN.
func (roles Roles) FindRoleByARN(arn string) (AWSRole, bool) {
	for _, role := range roles {
		if role.ARN == arn {
			return role, true
		}
	}
	return AWSRole{}, false
}

// FindRolesByName finds all roles matching the provided name.
func (roles Roles) FindRolesByName(name string) (result Roles) {
	for _, role := range roles {
		// Match either full name or the display name.
		if role.Display == name || role.Name == name {
			result = append(result, role)
		}
	}
	return
}
