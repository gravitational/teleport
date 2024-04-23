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

package awsoidc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/defaults"
	awsutil "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

const (
	descriptionOIDCIdPRole = "Used by Teleport to provide access to AWS resources."
)

// IdPIAMConfigureRequest is a request to configure the required Policies to use the EC2 Instance Connect Endpoint feature.
type IdPIAMConfigureRequest struct {
	// Cluster is the Teleport Cluster.
	// Used for tagging the created Roles/IdP.
	Cluster string

	// AccountID is the AWS Account ID.
	// Optional. sts.GetCallerIdentity is used if not provided.
	AccountID string

	// IntegrationName is the Integration Name.
	// Used for tagging the created Roles/IdP.
	IntegrationName string

	// ProxyPublicAddress is the URL to use as provider URL.
	// This must be a valid URL (ie, url.Parse'able)
	// Eg, https://<tenant>.teleport.sh, https://proxy.example.org:443, https://teleport.ec2.aws:3080
	// Only one of ProxyPublicAddress or S3BucketLocation can be used.
	ProxyPublicAddress string

	// S3BucketLocation is the S3 URI which contains the bucket name and prefix for the issuer.
	// Format: s3://<bucket-name>/<prefix>
	// Eg, s3://my-bucket/idp-teleport
	// This is used in two places:
	// - create openid configuration and jwks objects
	// - set up the issuer
	// The bucket must be public and will be created if it doesn't exist.
	//
	// If empty, the ProxyPublicAddress is used as issuer and no s3 objects are created.
	S3BucketLocation string

	// S3JWKSContentsB64 must contain the public keys for the Issuer.
	// The contents must be Base64 encoded.
	// Eg. base64(`{"keys":[{"kty":"RSA","alg":"RS256","n":"<value of n>","e":"<value of e>","use":"sig","kid":""}]}`)
	S3JWKSContentsB64 string
	s3Bucket          string
	s3BucketPrefix    string
	jwksFileContents  []byte

	// issuer is the above value but only contains the host.
	// Eg, <tenant>.teleport.sh, proxy.example.org, my-bucket.s3.amazonaws.com/my-prefix
	issuer string
	// issuerURL is the full url for the issuer
	// Eg, https://<tenant>.teleport.sh, https://proxy.example.org, https://my-bucket.s3.amazonaws.com/my-prefix
	issuerURL string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	ownershipTags AWSTags
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *IdPIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.Cluster == "" {
		return trace.BadParameter("cluster is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if (r.ProxyPublicAddress == "" && r.S3BucketLocation == "") || (r.ProxyPublicAddress != "" && r.S3BucketLocation != "") {
		return trace.BadParameter("provide only one of --proxy-public-url or --s3-bucket-uri")
	}

	if r.ProxyPublicAddress != "" {
		issuerURL, err := url.Parse(r.ProxyPublicAddress)
		if err != nil {
			return trace.BadParameter("--proxy-public-url is not a valid url: %v", err)
		}
		r.issuer = issuerURL.Host
		if issuerURL.Port() == "443" {
			r.issuer = issuerURL.Hostname()
		}
		r.issuerURL = issuerURL.String()
	}

	if r.S3BucketLocation != "" {
		s3BucketURL, err := url.Parse(r.S3BucketLocation)
		if err != nil || s3BucketURL.Scheme != "s3" {
			return trace.BadParameter("--s3-bucket-uri must be valid s3 uri (eg s3://bucket/prefix)")
		}
		r.s3Bucket = s3BucketURL.Host
		r.s3BucketPrefix = strings.TrimPrefix(s3BucketURL.Path, "/")

		r.issuer = fmt.Sprintf("%s.s3.amazonaws.com/%s", r.s3Bucket, r.s3BucketPrefix)
		r.issuerURL = "https://" + r.issuer

		if len(r.S3JWKSContentsB64) == 0 {
			return trace.BadParameter("--s3-jwks-base64 is required.")
		}
		r.jwksFileContents, err = base64.StdEncoding.DecodeString(r.S3JWKSContentsB64)
		if err != nil {
			return trace.BadParameter("--s3-jwks-base64 is invalid: %v", err)
		}
	}

	r.ownershipTags = defaultResourceCreationTags(r.Cluster, r.IntegrationName)

	return nil
}

// IdPIAMConfigureClient describes the required methods to create the AWS OIDC IdP and a Role that trusts that identity provider.
// There is no guarantee that the client is thread safe.
type IdPIAMConfigureClient interface {
	// SetAWSRegion sets the aws region that must be used.
	// This is particularly relevant for API calls that must target a specific region's endpoint.
	// Eg calling S3 APIs for buckets that are in another region.
	SetAWSRegion(string)

	// RegionForCreateBucket is the AWS Region that should be used to create buckets.
	RegionForCreateBucket() string

	// HTTPHead performs an HTTP request for the URL using the HEAD verb.
	HTTPHead(ctx context.Context, url string) (resp *http.Response, err error)

	// GetCallerIdentity returns information about the caller identity.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)

	// CreateOpenIDConnectProvider creates an IAM OIDC IdP.
	CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput, optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error)

	// CreateRole creates a new IAM Role.
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)

	// GetRole retrieves information about the specified role, including the role's path,
	// GUID, ARN, and the role's trust policy that grants permission to assume the
	// role.
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)

	// UpdateAssumeRolePolicy updates the policy that grants an IAM entity permission to assume a role.
	// This is typically referred to as the "role trust policy".
	UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error)

	// CreateBucket creates an Amazon S3 bucket.
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)

	// PutObject adds an object to a bucket.
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

	// HeadBucket checks if a bucket exists and if you have permission to access it.
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)

	// DeletePublicAccessBlock removes the PublicAccessBlock configuration for an Amazon S3 bucket.
	DeletePublicAccessBlock(ctx context.Context, params *s3.DeletePublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.DeletePublicAccessBlockOutput, error)
}

type defaultIdPIAMConfigureClient struct {
	httpClient *http.Client

	*iam.Client
	awsConfig aws.Config
	stsClient *sts.Client
	s3Client  *s3.Client
}

// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
func (d *defaultIdPIAMConfigureClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return d.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

// CreateBucket creates an Amazon S3 bucket.
func (d *defaultIdPIAMConfigureClient) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	return d.s3Client.CreateBucket(ctx, params, optFns...)
}

// PutObject adds an object to a bucket.
func (d *defaultIdPIAMConfigureClient) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return d.s3Client.PutObject(ctx, params, optFns...)
}

// HeadBucket adds an object to a bucket.
func (d *defaultIdPIAMConfigureClient) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return d.s3Client.HeadBucket(ctx, params, optFns...)
}

// PutBucketPolicy applies an Amazon S3 bucket policy to an Amazon S3 bucket.
func (d *defaultIdPIAMConfigureClient) PutBucketPolicy(ctx context.Context, params *s3.PutBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.PutBucketPolicyOutput, error) {
	return d.s3Client.PutBucketPolicy(ctx, params, optFns...)
}

// DeletePublicAccessBlock removes the PublicAccessBlock configuration for an Amazon S3 bucket.
func (d *defaultIdPIAMConfigureClient) DeletePublicAccessBlock(ctx context.Context, params *s3.DeletePublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.DeletePublicAccessBlockOutput, error) {
	return d.s3Client.DeletePublicAccessBlock(ctx, params, optFns...)
}

// GetBucketPolicy returns the policy of a specified bucket
func (d *defaultIdPIAMConfigureClient) GetBucketPolicy(ctx context.Context, params *s3.GetBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	return d.s3Client.GetBucketPolicy(ctx, params, optFns...)
}

// RegionForCreateBucket returns the region where the bucket should be created.
func (d *defaultIdPIAMConfigureClient) RegionForCreateBucket() string {
	return d.awsConfig.Region
}

// SetAWSRegion sets the aws region for next api calls.
func (d *defaultIdPIAMConfigureClient) SetAWSRegion(awsRegion string) {
	if d.awsConfig.Region == awsRegion {
		return
	}

	d.awsConfig.Region = awsRegion

	// S3 Client is the only client that depends on the region.
	d.s3Client = s3.NewFromConfig(d.awsConfig)
}

// HTTPHead performs an HTTP request for the URL using the HEAD verb.
func (d *defaultIdPIAMConfigureClient) HTTPHead(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return d.httpClient.Do(req)
}

// NewIdPIAMConfigureClient creates a new IdPIAMConfigureClient.
// The client is not thread safe.
func NewIdPIAMConfigureClient(ctx context.Context) (IdPIAMConfigureClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Region == "" {
		return nil, trace.BadParameter("failed to resolve local AWS region from environment, please set the AWS_REGION environment variable")
	}

	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultIdPIAMConfigureClient{
		httpClient: httpClient,
		awsConfig:  cfg,
		Client:     iam.NewFromConfig(cfg),
		stsClient:  sts.NewFromConfig(cfg),
		s3Client:   s3.NewFromConfig(cfg),
	}, nil
}

// ConfigureIdPIAM creates a new IAM OIDC IdP in AWS.
//
// The Provider URL is Teleport's Public Address or the S3 bucket.
// It also creates a new Role configured to trust the recently created IdP.
// If the role already exists, it will create another trust relationship for the IdP (if it doesn't exist).
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:CreateOpenIDConnectProvider
//   - iam:CreateRole
//   - iam:GetRole
//   - iam:UpdateAssumeRolePolicy
//
// If it's using the S3 bucket flow, the following are required as well:
//   - s3:CreateBucket
//   - s3:PutBucketPublicAccessBlock (used for s3:DeletePublicAccessBlock)
//   - s3:ListBuckets (used for s3:HeadBucket)
//   - s3:PutObject
func ConfigureIdPIAM(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if req.AccountID == "" {
		callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		req.AccountID = aws.ToString(callerIdentity.Account)
	}

	logrus.Infof("Creating IAM OpenID Connect Provider: url=%q.", req.issuerURL)
	if err := ensureOIDCIdPIAM(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}

	logrus.Infof("Creating IAM Role %q.", req.IntegrationRole)
	if err := upsertIdPIAMRole(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}

	// Configuration stops here if there's no S3 bucket.
	// It will use the teleport's public address as IdP issuer.
	if req.s3Bucket == "" {
		return nil
	}
	log := logrus.WithFields(logrus.Fields{
		"bucket":        req.s3Bucket,
		"bucket-prefix": req.s3BucketPrefix,
	})

	log.Infof("Creating bucket in region %q", clt.RegionForCreateBucket())
	if err := ensureBucketIdPIAM(ctx, clt, req, log); err != nil {
		return trace.Wrap(err)
	}

	log.Info(`Removing "Block all public access".`)
	_, err := clt.DeletePublicAccessBlock(ctx, &s3.DeletePublicAccessBlockInput{
		Bucket:              &req.s3Bucket,
		ExpectedBucketOwner: &req.AccountID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	log.Info("Uploading 'openid-configuration' and 'jwks' files.")
	if err := uploadOpenIDPublicFiles(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func ensureOIDCIdPIAM(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	var err error
	// For S3 bucket setups the thumbprint is ignored, but the API still requires a parseable one.
	// https://github.com/aws-actions/configure-aws-credentials/issues/357#issuecomment-1626357333
	// We pass this dummy one for those scenarios.
	thumbprint := "afafafafafafafafafafafafafafafafafafafaf"

	// For set ups that use the ProxyPublicAddress, we still calculate the thumbprint.
	if req.ProxyPublicAddress != "" {
		thumbprint, err = ThumbprintIdP(ctx, req.ProxyPublicAddress)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	_, err = clt.CreateOpenIDConnectProvider(ctx, &iam.CreateOpenIDConnectProviderInput{
		ThumbprintList: []string{thumbprint},
		Url:            &req.issuerURL,
		ClientIDList:   []string{types.IntegrationAWSOIDCAudience},
		Tags:           req.ownershipTags.ToIAMTags(),
	})
	if err != nil {
		awsErr := awslib.ConvertIAMv2Error(err)
		if trace.IsAlreadyExists(awsErr) {
			return nil
		}

		return trace.Wrap(err)
	}

	return nil
}

func ensureBucketIdPIAM(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest, log *logrus.Entry) error {
	// According to https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetBucketLocation.html
	// s3:GetBucketLocation is not recommended, and should be replaced by s3:HeadBucket according to AWS docs.
	// The issue with using s3:HeadBucket is that it returns an error if the SDK client's region is not the same as the bucket.
	// Doing a HEAD HTTP request seems to be the best option
	resp, err := clt.HTTPHead(ctx, fmt.Sprintf("https://s3.amazonaws.com/%s", req.s3Bucket))
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	// Even if the bucket is private, the "x-amz-bucket-region" Header will be there.
	bucketRegion := resp.Header.Get("x-amz-bucket-region")
	if bucketRegion != "" {
		if bucketRegion == "EU" {
			bucketRegion = "eu-west-1"
		}

		clt.SetAWSRegion(bucketRegion)
	}

	headBucketResp, err := clt.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket:              &req.s3Bucket,
		ExpectedBucketOwner: &req.AccountID,
	})
	if err == nil {
		log.Infof("Bucket already exists in %q", aws.ToString(headBucketResp.BucketRegion))
		return nil
	}
	awsErr := awslib.ConvertIAMv2Error(err)
	if trace.IsNotFound(awsErr) {
		_, err := clt.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket:                    &req.s3Bucket,
			CreateBucketConfiguration: awsutil.CreateBucketConfiguration(clt.RegionForCreateBucket()),
			ObjectOwnership:           s3types.ObjectOwnershipBucketOwnerPreferred,
		})
		return trace.Wrap(err)
	}

	return trace.Wrap(awsErr)
}

func uploadOpenIDPublicFiles(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	openidConfigPath := path.Join(req.s3BucketPrefix, ".well-known/openid-configuration")
	jwksBucketPath := path.Join(req.s3BucketPrefix, ".well-known/jwks")
	jwksPublicURI, err := url.JoinPath(req.issuerURL, ".well-known/jwks")
	if err != nil {
		return trace.Wrap(err)
	}

	openIDConfigJSON, err := json.Marshal(oidc.OpenIDConfigurationForIssuer(req.issuer, jwksPublicURI))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = clt.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &req.s3Bucket,
		Key:    &openidConfigPath,
		Body:   bytes.NewReader(openIDConfigJSON),
		ACL:    s3types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &req.s3Bucket,
		Key:    &jwksBucketPath,
		Body:   bytes.NewReader(req.jwksFileContents),
		ACL:    s3types.ObjectCannedACLPublicRead,
	})
	return trace.Wrap(err)
}

func createIdPIAMRole(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	integrationRoleAssumeRoleDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForAWSOIDCRoleTrustRelationship(req.AccountID, req.issuer, []string{types.IntegrationAWSOIDCAudience}),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 &req.IntegrationRole,
		Description:              aws.String(descriptionOIDCIdPRole),
		AssumeRolePolicyDocument: &integrationRoleAssumeRoleDocument,
		Tags:                     req.ownershipTags.ToIAMTags(),
	})
	return trace.Wrap(err)
}

func upsertIdPIAMRole(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	getRoleOut, err := clt.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &req.IntegrationRole,
	})
	if err != nil {
		convertedErr := awslib.ConvertIAMv2Error(err)
		if !trace.IsNotFound(convertedErr) {
			return trace.Wrap(convertedErr)
		}

		return trace.Wrap(createIdPIAMRole(ctx, clt, req))
	}

	if !req.ownershipTags.MatchesIAMTags(getRoleOut.Role.Tags) {
		return trace.BadParameter("IAM Role %q already exists but is not managed by Teleport. "+
			"Add the following tags to allow Teleport to manage this Role: %s", req.IntegrationRole, req.ownershipTags)
	}

	trustRelationshipDoc, err := awslib.ParsePolicyDocument(aws.ToString(getRoleOut.Role.AssumeRolePolicyDocument))
	if err != nil {
		return trace.Wrap(err)
	}

	trustRelationshipForIdP := awslib.StatementForAWSOIDCRoleTrustRelationship(req.AccountID, req.issuer, []string{types.IntegrationAWSOIDCAudience})
	for _, existingStatement := range trustRelationshipDoc.Statements {
		if existingStatement.EqualStatement(trustRelationshipForIdP) {
			return nil
		}
	}

	trustRelationshipDoc.Statements = append(trustRelationshipDoc.Statements, trustRelationshipForIdP)
	trustRelationshipDocString, err := trustRelationshipDoc.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &trustRelationshipDocString,
	})
	return trace.Wrap(err)
}
