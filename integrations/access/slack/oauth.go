// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slack

import (
	"context"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/slack-go/slack"

	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

// Authorizer implements oauth2.Authorizer for Slack API.
type Authorizer struct {
	client       *http.Client
	clientID     string
	clientSecret string
}

func newAuthorizer(client *http.Client, clientID string, clientSecret string) *Authorizer {
	return &Authorizer{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// NewAuthorizer returns a new Authorizer.
//
// clientID is the Client ID for this Slack app as specified by OAuth2.
// clientSecret is the Client Secret for this Slack app as specified by OAuth2.
func NewAuthorizer(clientID string, clientSecret string) *Authorizer {
	return newAuthorizer(http.DefaultClient, clientID, clientSecret)
}

// Exchange implements oauth.Exchanger
func (a *Authorizer) Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error) {
	result, err := slack.GetOAuthV2ResponseContext(ctx, a.client, a.clientID, a.clientSecret, authorizationCode, redirectURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !result.Ok {
		return nil, trace.Errorf("%s", result.Error)
	}

	return &storage.Credentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// Refresh implements oauth.Refresher
func (a *Authorizer) Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error) {
	result, err := slack.RefreshOAuthV2TokenContext(ctx, a.client, a.clientID, a.clientSecret, refreshToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !result.Ok {
		return nil, trace.Errorf("%s", result.Error)
	}

	return &storage.Credentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

var _ oauth.Authorizer = &Authorizer{}
