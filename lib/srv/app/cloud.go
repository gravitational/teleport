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

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/ssocreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// Cloud provides cloud provider access related methods such as generating
// sign in URLs for management consoles.
type Cloud interface {
	// GetAWSSigninURL generates AWS management console federation sign-in URL.
	GetAWSSigninURL(context.Context, AWSSigninRequest) (*AWSSigninResponse, error)
}

// AWSSigninRequest is a request to generate AWS console signin URL.
type AWSSigninRequest struct {
	// Identity is the identity of the user requesting signin URL.
	Identity *tlsca.Identity
	// TargetURL is the target URL within the console.
	TargetURL string
	// Issuer is the application public URL.
	Issuer string
	// ExternalID is the AWS external ID.
	ExternalID string
	// Integration is the Integration name to use to generate credentials.
	// If empty, it will use ambient credentials
	Integration string
}

// CheckAndSetDefaults validates the request.
func (r *AWSSigninRequest) CheckAndSetDefaults() error {
	if r.Identity == nil {
		return trace.BadParameter("missing Identity")
	}
	_, err := awsutils.ParseRoleARN(r.Identity.RouteToApp.AWSRoleARN)
	if err != nil {
		return trace.Wrap(err)
	}
	if r.TargetURL == "" {
		return trace.BadParameter("missing TargetURL")
	}
	if r.Issuer == "" {
		return trace.BadParameter("missing Issuer")
	}
	return nil
}

// AWSSigninResponse contains AWS console signin URL.
type AWSSigninResponse struct {
	// SigninURL is the console signin URL.
	SigninURL string
}

// CloudConfig is the configuration for cloud service.
type CloudConfig struct {
	// SessionGetter returns an AWS session.
	SessionGetter awsutils.AWSSessionProvider
	// Clock is used to override time in tests.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates the config.
func (c *CloudConfig) CheckAndSetDefaults() error {
	if c.SessionGetter == nil {
		return trace.BadParameter("missing session getter")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

type cloud struct {
	cfg CloudConfig
}

// NewCloud creates a new cloud service.
func NewCloud(cfg CloudConfig) (Cloud, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cloud{
		cfg: cfg,
	}, nil
}

// GetAWSSigninURL generates AWS management console federation sign-in URL.
//
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_enable-console-custom-url.html
func (c *cloud) GetAWSSigninURL(ctx context.Context, req AWSSigninRequest) (*AWSSigninResponse, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	federationURL := getFederationURL(req.TargetURL)
	signinToken, err := c.getAWSSigninToken(ctx, &req, federationURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signinURL, err := url.Parse(federationURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signinURL.RawQuery = url.Values{
		"Action":      []string{"login"},
		"SigninToken": []string{signinToken},
		"Destination": []string{req.TargetURL},
		"Issuer":      []string{req.Issuer},
	}.Encode()

	return &AWSSigninResponse{
		SigninURL: signinURL.String(),
	}, nil
}

// getAWSSigninToken gets the signin token required for the AWS sign in URL.
//
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_enable-console-custom-url.html
func (c *cloud) getAWSSigninToken(ctx context.Context, req *AWSSigninRequest, endpoint string, options ...func(*stscreds.AssumeRoleProvider)) (string, error) {
	// It is stated in the user guide linked above:
	// When you use DurationSeconds in an AssumeRole* operation, you must call
	// it as an IAM user with long-term credentials. Otherwise, the call to the
	// federation endpoint in step 3 fails.
	//
	// Experiments showed that the getSigninToken call will still succeed as
	// long as the "SessionDuration" is not provided when calling the API, when
	// the AWS session is using temporary credentials. However, when the
	// "SessionDuration" is not provided, the web console session duration will
	// be bound to the duration used in the next AssumeRole call.

	// Sign In requests target IAM endpoints which don't require a region.
	region := ""
	session, err := c.cfg.SessionGetter(ctx, region, req.Integration)
	if err != nil {
		return "", trace.Wrap(err)
	}

	temporarySession, err := isSessionUsingTemporaryCredentials(session)
	if err != nil {
		return "", trace.Wrap(err)
	}

	duration, err := c.getFederationDuration(req, temporarySession)
	if err != nil {
		return "", trace.Wrap(err)
	}

	options = append(options, func(creds *stscreds.AssumeRoleProvider) {
		// Setting role session name to Teleport username will allow to
		// associate CloudTrail events with the Teleport user.
		creds.RoleSessionName = awsutils.MaybeHashRoleSessionName(req.Identity.Username)

		// Setting web console session duration through AssumeRole call for AWS
		// sessions with temporary credentials.
		// Technically the session duration can be set this way for
		// non-temporary sessions. However, the AssumeRole call will fail if we
		// are requesting duration longer than the maximum session duration of
		// the role we are assuming. In addition, the session credentials may
		// not have permission to perform a get-role on the role. Therefore,
		// "SessionDuration" parameter will be defined when calling federation
		// endpoint below instead of here, for non-temporary sessions.
		//
		// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
		if temporarySession {
			creds.Duration = duration
		}

		if req.ExternalID != "" {
			creds.ExternalID = aws.String(req.ExternalID)
		}
	})
	stsCredentials, err := stsutils.NewCredentialsV1(session, req.Identity.RouteToApp.AWSRoleARN, options...).Get()
	if err != nil {
		return "", trace.Wrap(err)
	}

	tokenURL, err := url.Parse(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}

	sessionBytes, err := json.Marshal(stsSession{
		SessionID:    stsCredentials.AccessKeyID,
		SessionKey:   stsCredentials.SecretAccessKey,
		SessionToken: stsCredentials.SessionToken,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	values := url.Values{
		"Action":  []string{"getSigninToken"},
		"Session": []string{string(sessionBytes)},
	}
	if !temporarySession {
		values["SessionDuration"] = []string{strconv.Itoa(int(duration.Seconds()))}
	}

	tokenURL.RawQuery = values.Encode()
	resp, err := http.Get(tokenURL.String())
	if err != nil {
		return "", trace.Wrap(err)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("non-200 response from AWS federation endpoint: %q %v %q",
			resp.Status, resp.Header, string(respBytes))
	}

	var fedResp federationResponse
	if err := json.Unmarshal(respBytes, &fedResp); err != nil {
		return "", trace.Wrap(err)
	}

	return fedResp.SigninToken, nil
}

// isSessionUsingTemporaryCredentials checks if the current aws session is
// using temporary credentials.
//
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp.html
func isSessionUsingTemporaryCredentials(session *awssession.Session) (bool, error) {
	if session.Config == nil || session.Config.Credentials == nil {
		return false, trace.NotFound("session credentials not found")
	}

	credentials, err := session.Config.Credentials.Get()
	if err != nil {
		return false, trace.Wrap(err)
	}

	switch credentials.ProviderName {
	case ec2rolecreds.ProviderName:
		return false, nil

	case
		// stscreds.AssumeRoleProvider retrieves temporary credentials from the
		// STS service, and keeps track of their expiration time.
		// https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/stscreds/#AssumeRoleProvider
		stscreds.ProviderName,

		// stscreds.WebIdentityRoleProvider is used to retrieve credentials
		// using an OIDC token.
		// https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/stscreds/#WebIdentityRoleProvider
		//
		// IAM roles for EKS service accounts are also granted through the OIDC tokens.
		// https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
		stscreds.WebIdentityProviderName,

		// ssocreds.Provider is an AWS credential provider that retrieves
		// temporary AWS credentials by exchanging an SSO login token.
		// https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/ssocreds/#Provider
		ssocreds.ProviderName:
		return true, nil
	}

	// For other providers, make an assumption that a session token is only
	// required for temporary security credentials retrieved via STS, otherwise
	// it is an empty string.
	// https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/#NewStaticCredentials
	return credentials.SessionToken != "", nil
}

// getFederationDuration calculates the duration of the federated session.
func (c *cloud) getFederationDuration(req *AWSSigninRequest, temporarySession bool) (time.Duration, error) {
	maxDuration := maxSessionDuration
	if temporarySession {
		maxDuration = maxTemporarySessionDuration
	}

	duration := req.Identity.Expires.Sub(c.cfg.Clock.Now())
	if duration > maxDuration {
		duration = maxDuration
	}

	if duration < minimumSessionDuration {
		return 0, trace.AccessDenied("minimum AWS session duration is %v but Teleport identity expires in %v", minimumSessionDuration, duration)
	}
	return duration, nil
}

// stsSession combines parameters describing session built from temporary credentials.
type stsSession struct {
	// SessionID is the assumed credentials access key ID.
	SessionID string `json:"sessionId"`
	// SessionKey is the assumed credentials secret access key.
	SessionKey string `json:"sessionKey"`
	// SessionToken is the assumed credentials session token.
	SessionToken string `json:"sessionToken"`
}

// federationResponse describes response returned by the federation endpoint.
type federationResponse struct {
	// SigninToken is the AWS console federation signin token.
	SigninToken string `json:"SigninToken"`
}

// getFederationURL picks the AWS federation endpoint based on the AWS
// partition of the target URL.
//
// https://docs.aws.amazon.com/general/latest/gr/signin-service.html
// https://docs.amazonaws.cn/en_us/aws/latest/userguide/endpoints-Beijing.html
func getFederationURL(targetURL string) string {
	// TODO(greedy52) support region based sign-in.
	switch {
	// AWS GovCloud (US) Partition.
	case strings.HasPrefix(targetURL, constants.AWSUSGovConsoleURL):
		return "https://signin.amazonaws-us-gov.com/federation"

	// AWS China Partition.
	case strings.HasPrefix(targetURL, constants.AWSCNConsoleURL):
		return "https://signin.amazonaws.cn/federation"

	// AWS Standard Partition.
	default:
		return "https://signin.aws.amazon.com/federation"
	}
}

const (
	// maxSessionDuration is the max federation session duration, which is 12
	// hours. The federation endpoint will error out if we request more.
	//
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_enable-console-custom-url.html
	maxSessionDuration = 12 * time.Hour
	// maxTemporarySessionDuration is the max federation session duration when
	// the AWS session is using temporary credentials. The maximum is one hour,
	// which is the maximum duration you can set when role chaining.
	//
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_terms-and-concepts.html
	maxTemporarySessionDuration = time.Hour
	// minimumSessionDuration is the minimum federation session duration. The
	// AssumeRole call will error out if we request less than 15 minutes.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	minimumSessionDuration = 15 * time.Minute
)
