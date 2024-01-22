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

package slack

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

// Authorizer implements oauth2.Authorizer for Slack API.
type Authorizer struct {
	client *resty.Client

	clientID     string
	clientSecret string
}

func newAuthorizer(client *resty.Client, clientID string, clientSecret string) *Authorizer {
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
	client := makeSlackClient(slackAPIURL)
	return newAuthorizer(client, clientID, clientSecret)
}

// Exchange implements oauth.Exchanger
func (a *Authorizer) Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error) {
	var result AccessResponse

	_, err := a.client.R().
		SetQueryParam("client_id", a.clientID).
		SetQueryParam("client_secret", a.clientSecret).
		SetQueryParam("code", authorizationCode).
		SetQueryParam("redirect_uri", redirectURI).
		SetResult(&result).
		Post("oauth.v2.access")

	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !result.Ok {
		return nil, trace.Errorf("%s", result.Error)
	}

	return &storage.Credentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(result.ExpiresInSeconds) * time.Second),
	}, nil
}

// Refresh implements oauth.Refresher
func (a *Authorizer) Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error) {
	var result AccessResponse
	_, err := a.client.R().
		SetQueryParam("client_id", a.clientID).
		SetQueryParam("client_secret", a.clientSecret).
		SetQueryParam("grant_type", "refresh_token").
		SetQueryParam("refresh_token", refreshToken).
		SetResult(&result).
		Post("oauth.v2.access")

	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !result.Ok {
		return nil, trace.Errorf("%s", result.Error)
	}

	return &storage.Credentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(result.ExpiresInSeconds) * time.Second),
	}, nil
}

var _ oauth.Authorizer = &Authorizer{}
