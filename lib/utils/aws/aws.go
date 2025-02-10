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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/textproto"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws/migration"
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

	// AmzTargetHeader is a header containing the API target.
	// Format: target_version.operation
	// Example: DynamoDB_20120810.Scan
	AmzTargetHeader = "X-Amz-Target"
	// AmzJSON1_0 is an AWS Content-Type header that indicates the media type is JSON.
	AmzJSON1_0 = "application/x-amz-json-1.0"
	// AmzJSON1_1 is an AWS Content-Type header that indicates the media type is JSON.
	AmzJSON1_1 = "application/x-amz-json-1.1"

	// MaxRoleSessionNameLength is the maximum length of the role session name
	// used by the AssumeRole call.
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html
	MaxRoleSessionNameLength = 64

	iamServiceName = "iam"
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
// AWS SigV4 header example below adds newlines for readability only - the real
// header must be a single continuous string with commas (and optional spaces)
// between the Credential, SignedHeaders, and Signature:
// Authorization: AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,
// SignedHeaders=host;range;x-amz-date,
// Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
func ParseSigV4(header string) (*SigV4, error) {
	if header == "" {
		return nil, trace.BadParameter("empty AWS SigV4 header")
	}
	if !strings.HasPrefix(header, AmazonSigV4AuthorizationPrefix+" ") {
		return nil, trace.BadParameter("missing AWS SigV4 authorization algorithm")
	}
	header = strings.TrimPrefix(header, AmazonSigV4AuthorizationPrefix+" ")

	components := strings.Split(header, ",")
	if len(components) != 3 {
		return nil, trace.BadParameter("expected AWS SigV4 Authorization header with 3 comma-separated components but got %d", len(components))
	}

	m := make(map[string]string)
	for _, v := range components {
		kv := strings.Split(strings.Trim(v, " "), "=")
		if len(kv) != 2 {
			continue
		}
		m[kv[0]] = kv[1]
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

// VerifyAWSSignature verifies the request signature ensuring that the request originates from tsh aws command execution
// AWS CLI signs the request with random generated credentials that are passed to LocalProxy by
// the AWSCredentials LocalProxyConfig configuration.
func VerifyAWSSignature(req *http.Request, credProvider aws.CredentialsProvider) error {
	sigV4, err := ParseSigV4(req.Header.Get("Authorization"))
	if err != nil {
		return trace.BadParameter("%s", err)
	}

	// Verifies the request is signed by the expected access key ID.
	credValue, err := credProvider.Retrieve(req.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	if sigV4.KeyID != credValue.AccessKeyID {
		return trace.AccessDenied("AccessKeyID does not match")
	}

	// Skip signature verification if the incoming request includes the
	// "User-Agent" header when making the signature. AWS Go SDK explicitly
	// skips the "User-Agent" header so it will always produce a different
	// signature. Only AccessKeyID is verified above in this case.
	for _, signedHeader := range sigV4.SignedHeaders {
		if strings.EqualFold(signedHeader, "User-Agent") {
			return nil
		}
	}

	// Read the request body and replace the body ready with a new reader that will allow reading the body again
	// by HTTP Transport.
	payload, err := utils.GetAndReplaceRequestBody(req)
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
		return trace.BadParameter("%s", err)
	}

	signer := NewSignerV2(credProvider, sigV4.Service)
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

// NewSignerV2 is a temporary AWS SDK migration helper.
func NewSignerV2(provider aws.CredentialsProvider, signingServiceName string) *v4.Signer {
	return NewSigner(migration.NewCredentialsAdapter(provider), signingServiceName)
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

// filterHeaders removes request headers that are not in the headers list and returns the removed header keys.
func filterHeaders(r *http.Request, headers []string) []string {
	keep := make(map[string]struct{})
	for _, key := range headers {
		keep[textproto.CanonicalMIMEHeaderKey(key)] = struct{}{}
	}

	var removed []string
	out := make(http.Header)
	for key, vals := range r.Header {
		if _, ok := keep[textproto.CanonicalMIMEHeaderKey(key)]; ok {
			out[key] = vals
			continue
		}
		removed = append(removed, key)
	}
	r.Header = out
	return removed
}

// FilterAWSRoles returns role ARNs from the provided list that belong to the
// specified AWS account ID.
//
// If AWS account ID is empty, all valid AWS IAM roles are returned.
func FilterAWSRoles(arns []string, accountID string) (result Roles) {
	for _, roleARN := range arns {
		parsed, err := ParseRoleARN(roleARN)
		if err != nil {
			slog.WarnContext(context.Background(), "Skipping invalid AWS role ARN.", "error", err)
			continue
		}
		if accountID != "" && parsed.AccountID != accountID {
			continue
		}

		// In AWS convention, the display of the role is the last
		// /-delineated substring.
		//
		// Example ARNs:
		// arn:aws:iam::1234567890:role/EC2FullAccess      (display: EC2FullAccess)
		// arn:aws:iam::1234567890:role/path/to/customrole (display: customrole)
		parts := strings.Split(parsed.Resource, "/")
		result = append(result, Role{
			Name:      strings.Join(parts[1:], "/"),
			Display:   parts[len(parts)-1],
			ARN:       roleARN,
			AccountID: parsed.AccountID,
		})
	}
	return result
}

// Role describes an AWS IAM role for AWS console access.
type Role struct {
	// Name is the full role name with the entire path.
	Name string `json:"name"`
	// Display is the role display name.
	Display string `json:"display"`
	// ARN is the full role ARN.
	ARN string `json:"arn"`
	// AccountID is the AWS Account ID this role refers to.
	AccountID string `json:"accountId"`
}

// Roles is a slice of roles.
type Roles []Role

// Sort sorts the roles by their display names.
func (roles Roles) Sort() {
	sort.SliceStable(roles, func(x, y int) bool {
		return strings.ToLower(roles[x].Display) < strings.ToLower(roles[y].Display)
	})
}

// FindRoleByARN finds the role with the provided ARN.
func (roles Roles) FindRoleByARN(arn string) (Role, bool) {
	for _, role := range roles {
		if role.ARN == arn {
			return role, true
		}
	}
	return Role{}, false
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

// UnmarshalRequestBody reads and unmarshals a JSON request body into a protobuf Struct wrapper.
// If the request is not a recognized AWS JSON media type, or the body cannot be read, or the body
// is not valid JSON, then this function returns a nil value and an error.
// The protobuf Struct wrapper is useful for serializing JSON into a protobuf, because otherwise when the
// protobuf is marshaled it will re-marshall a JSON string field with escape characters or base64 encode
// a []byte field.
// Examples showing differences:
// - JSON string in proto: `{"Table": "some-table"}` --marshal to JSON--> `"{\"Table\": \"some-table\"}"`
// - bytes in proto: []byte --marshal to JSON--> `eyJUYWJsZSI6ICJzb21lLXRhYmxlIn0K` (base64 encoded)
// - *Struct in proto: *Struct --marshal to JSON--> `{"Table": "some-table"}` (unescaped JSON)
func UnmarshalRequestBody(req *http.Request) (*apievents.Struct, error) {
	contentType := req.Header.Get("Content-Type")
	if !isJSON(contentType) {
		return nil, trace.BadParameter("invalid JSON request Content-Type: %q", contentType)
	}
	jsonBody, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s := &apievents.Struct{}
	if err := s.UnmarshalJSON(jsonBody); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

// isJSON returns true if the Content-Type is recognized as standard JSON or any non-standard
// Amazon Content-Type header that indicates JSON media type.
func isJSON(contentType string) bool {
	switch contentType {
	case "application/json", AmzJSON1_0, AmzJSON1_1:
		return true
	default:
		return false
	}
}

// BuildRoleARN constructs a string AWS ARN from a username, region, and account ID.
// If username is an AWS ARN, this function checks that the ARN is an AWS IAM Role ARN
// in the correct partition and account.
func BuildRoleARN(username, region, accountID string) (string, error) {
	partition := apiawsutils.GetPartitionFromRegion(region)
	if arn.IsARN(username) {
		// sanity check the given username role ARN.
		parsed, err := ParseRoleARN(username)
		if err != nil {
			return "", trace.Wrap(err)
		}
		// don't check for empty accountID - callers do not always pass an account ID,
		// and it's only absolutely required if we need to build the role ARN below.
		if err := CheckARNPartitionAndAccount(parsed, partition, accountID); err != nil {
			return "", trace.Wrap(err)
		}
		return username, nil
	}
	resource := username
	if !IsPartialRoleARN(resource) {
		resource = fmt.Sprintf("role/%s", username)
	}
	roleARN := arn.ARN{
		Partition: partition,
		Service:   iamServiceName,
		AccountID: accountID,
		Resource:  resource,
	}
	if err := apiawsutils.CheckRoleARN(roleARN.String()); err != nil {
		return "", trace.Wrap(err)
	}
	return roleARN.String(), nil
}

// ValidateRoleARNAndExtractRoleName validates the role ARN and extracts the
// short role name from it.
func ValidateRoleARNAndExtractRoleName(roleARN, wantPartition, wantAccountID string) (string, error) {
	role, err := ParseRoleARN(roleARN)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := CheckARNPartitionAndAccount(role, wantPartition, wantAccountID); err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimPrefix(role.Resource, "role/"), nil
}

// ParseRoleARN parses an AWS ARN and checks that the ARN is
// for an IAM Role resource.
func ParseRoleARN(roleARN string) (*arn.ARN, error) {
	role, err := arn.Parse(roleARN)
	if err != nil {
		return nil, trace.BadParameter("invalid AWS ARN: %v", err)
	}
	if err := checkRoleARN(&role); err != nil {
		return nil, trace.Wrap(err)
	}
	return &role, nil
}

// checkRoleARN returns whether a parsed ARN is for an IAM Role resource.
// Example role ARN: arn:aws:iam::123456789012:role/some-role-name
func checkRoleARN(parsed *arn.ARN) error {
	parts := strings.Split(parsed.Resource, "/")
	if parts[0] != "role" || parsed.Service != iamServiceName {
		return trace.BadParameter("%q is not an AWS IAM role ARN", parsed)
	}
	if len(parts) < 2 || len(parts[len(parts)-1]) == 0 {
		return trace.BadParameter("%q is missing AWS IAM role name", parsed)
	}
	if err := apiawsutils.IsValidAccountID(parsed.AccountID); err != nil {
		return trace.BadParameter("%q invalid account ID: %v", parsed, err)
	}
	return nil
}

// CheckARNPartitionAndAccount checks an AWS ARN against an expected AWS partition and account ID.
// An empty expected AWS partition or account ID is not checked.
func CheckARNPartitionAndAccount(ARN *arn.ARN, wantPartition, wantAccountID string) error {
	if ARN.Partition != wantPartition && wantPartition != "" {
		return trace.BadParameter("expected AWS partition %q but got %q", wantPartition, ARN.Partition)
	}
	if ARN.AccountID != wantAccountID && wantAccountID != "" {
		return trace.BadParameter("expected AWS account ID %q but got %q", wantAccountID, ARN.AccountID)
	}
	return nil
}

// IsRoleARN returns true if the provided string is a AWS role ARN.
func IsRoleARN(roleARN string) bool {
	if _, err := ParseRoleARN(roleARN); err == nil {
		return true
	}

	return IsPartialRoleARN(roleARN)
}

// IsPartialRoleARN returns true if the provided role ARN only contains the
// resource name.
func IsPartialRoleARN(roleARN string) bool {
	return strings.HasPrefix(roleARN, "role/")
}

// IsUserARN returns true if the provided string is a AWS user ARN.
func IsUserARN(userARN string) bool {
	resourceName := userARN
	if parsed, err := arn.Parse(userARN); err == nil {
		resourceName = parsed.Resource
	}

	return strings.HasPrefix(resourceName, "user/")
}

// PolicyARN returns the ARN representation of an AWS IAM Policy.
func PolicyARN(partition, accountID, policy string) string {
	return iamResourceARN(partition, accountID, "policy", policy)
}

// RoleARN returns the ARN representation of an AWS IAM Role.
func RoleARN(partition, accountID, role string) string {
	return iamResourceARN(partition, accountID, "role", role)
}

func iamResourceARN(partition, accountID, resourceType, resourceName string) string {
	return arn.ARN{
		Partition: partition,
		Service:   "iam",
		AccountID: accountID,
		Resource:  fmt.Sprintf("%s/%s", resourceType, resourceName),
	}.String()
}

// MaybeHashRoleSessionName truncates the role session name and adds a hash
// when the original role session name is greater than AWS character limit
// (64).
func MaybeHashRoleSessionName(roleSessionName string) (ret string) {
	if len(roleSessionName) <= MaxRoleSessionNameLength {
		return roleSessionName
	}

	const hashLen = 16
	hash := sha1.New()
	hash.Write([]byte(roleSessionName))
	hex := hex.EncodeToString(hash.Sum(nil))[:hashLen]

	// "1" for the delimiter.
	keepPrefixIndex := MaxRoleSessionNameLength - len(hex) - 1

	// Sanity check. This should never happen since hash length and
	// MaxRoleSessionNameLength are both constant.
	if keepPrefixIndex < 0 {
		keepPrefixIndex = 0
	}

	ret = fmt.Sprintf("%s-%s", roleSessionName[:keepPrefixIndex], hex)
	slog.DebugContext(context.Background(), "AWS role session name is too long. Using a hash instead.", "hashed", ret, "original", roleSessionName)
	return ret
}
