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

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Cloud provides cloud provider access related methods such as generating
// sign in URLs for management consoles.
type Cloud interface {
	// GetAWSSigninURL generates AWS management console federation sign-in URL.
	GetAWSSigninURL(AWSSigninRequest) (*AWSSigninResponse, error)
}

// AWSSigninRequest is a request to generate AWS console signin URL.
type AWSSigninRequest struct {
	// Identity is the identity of the user requesting signin URL.
	Identity *tlsca.Identity
	// TargetURL is the target URL within the console.
	TargetURL string
	// Issuer is the application public URL.
	Issuer string
}

// CheckAndSetDefaults validates the request.
func (r *AWSSigninRequest) CheckAndSetDefaults() error {
	if r.Identity == nil {
		return trace.BadParameter("missing Identity")
	}
	_, err := arn.Parse(r.Identity.RouteToApp.AWSRoleARN)
	if err != nil {
		return trace.Wrap(err)
	}
	if r.TargetURL == "" {
		r.TargetURL = consoleURL
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
	// Session is AWS session.
	Session *awssession.Session
	// Clock is used to override time in tests.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates the config.
func (c *CloudConfig) CheckAndSetDefaults() error {
	if c.Session == nil {
		session, err := awssession.NewSessionWithOptions(awssession.Options{
			SharedConfigState: awssession.SharedConfigEnable,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		c.Session = session
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

type cloud struct {
	cfg CloudConfig
	log logrus.FieldLogger
}

// NewCloud creates a new cloud service.
func NewCloud(cfg CloudConfig) (Cloud, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cloud{
		cfg: cfg,
		log: logrus.WithField(trace.Component, "cloud"),
	}, nil
}

// GetAWSSigninURL generates AWS management console federation sign-in URL.
//
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_enable-console-custom-url.html
func (c *cloud) GetAWSSigninURL(req AWSSigninRequest) (*AWSSigninResponse, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stsCredentials, err := stscreds.NewCredentials(c.cfg.Session, req.Identity.RouteToApp.AWSRoleARN,
		func(creds *stscreds.AssumeRoleProvider) {
			// Setting role session name to Teleport username will allow to
			// associate CloudTrail events with the Teleport user.
			creds.RoleSessionName = req.Identity.Username
		}).Get()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenURL, err := url.Parse(federationURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionBytes, err := json.Marshal(stsSession{
		SessionID:    stsCredentials.AccessKeyID,
		SessionKey:   stsCredentials.SecretAccessKey,
		SessionToken: stsCredentials.SessionToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Max AWS federation session duration is 12 hours. The federation endpoint
	// will error out if we request more.
	duration := req.Identity.Expires.Sub(c.cfg.Clock.Now())
	if duration > maxSessionDuration {
		duration = maxSessionDuration
	}

	tokenURL.RawQuery = url.Values{
		"Action":          []string{"getSigninToken"},
		"SessionDuration": []string{fmt.Sprintf("%d", int(duration.Seconds()))},
		"Session":         []string{string(sessionBytes)},
	}.Encode()

	resp, err := http.Get(tokenURL.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("non-200 response from AWS federation endpoint: %q %v %q",
			resp.Status, resp.Header, string(respBytes))
	}

	var fedResp federationResponse
	if err := json.Unmarshal(respBytes, &fedResp); err != nil {
		return nil, trace.Wrap(err)
	}

	signinURL, err := url.Parse(federationURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signinURL.RawQuery = url.Values{
		"Action":      []string{"login"},
		"SigninToken": []string{fedResp.SigninToken},
		"Destination": []string{req.TargetURL},
		"Issuer":      []string{req.Issuer},
	}.Encode()

	return &AWSSigninResponse{
		SigninURL: signinURL.String(),
	}, nil
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

const (
	// federationURL is the AWS federation endpoint.
	federationURL = "https://signin.aws.amazon.com/federation"
	// consoleURL is the default AWS console destination.
	consoleURL = "https://console.aws.amazon.com/ec2/v2/home"
	// maxSessionDuration is the max federation session duration.
	maxSessionDuration = 12 * time.Hour
)
