// Copyright 2022 Gravitational, Inc
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

package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2/google"
	directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

// addGsuiteClaims will fetch extra data from proprietary Google APIs if
// applicable to the connector, and it will add claims based on the fetched
// data. The current implementation adds a "groups" claim containing the Google
// Groups that the user is a member of.
//
// If clientOptions is not empty, it will be passed to the underlying google api
// client without loading the credentials according to the configuration of the
// connector. The credentials in the default case are loaded from the connector
// with the getGsuiteCredentialsForConnector function.
func addGsuiteClaims(ctx context.Context, connector types.OIDCConnector, claims jose.Claims, clientOptions ...option.ClientOption) (jose.Claims, error) {
	// If google_service_account_uri and google_service_account are not set, we
	// assume that this is a non-GWorkspace OIDC provider using the same
	// issuer URL as Google Workspace (e.g.
	// https://developers.google.com/identity/protocols/oauth2/openid-connect).
	if connector.GetIssuerURL() != "https://accounts.google.com" || (connector.GetGoogleServiceAccountURI() == "" && connector.GetGoogleServiceAccount() == "") {
		return claims, nil
	}

	email, exists, err := claims.StringClaim("email")
	if err != nil || !exists {
		return nil, trace.BadParameter("no email in oauth claims for Google Workspace account")
	}

	if len(clientOptions) == 0 {
		credentials, err := getGsuiteCredentialsForConnector(ctx, connector)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clientOptions = []option.ClientOption{option.WithCredentials(credentials)}
	}

	var gsuiteGroups []string
	if connector.GetGoogleTransitiveGroups() {
		gsuiteGroups, err = groupsFromGsuiteCloudidentity(ctx, email, clientOptions...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		gsuiteGroups, err = groupsFromGsuiteDirectory(ctx, email, clientOptions...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if len(gsuiteGroups) > 0 {
		gsuiteClaims := jose.Claims{"groups": gsuiteGroups}
		log.Debugf("Claims from Google Workspace: %v.", gsuiteClaims)
		claims, err = mergeClaims(claims, gsuiteClaims)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		log.Debugf("No Google Workspace claims.")
	}

	return claims, nil
}

func getGsuiteCredentialsForConnector(ctx context.Context, connector types.OIDCConnector) (*google.Credentials, error) {
	var jsonCredentials []byte
	var credentialLoadingMethod string
	if connector.GetGoogleServiceAccountURI() != "" {
		// load the google service account from URI
		credentialLoadingMethod = "google_service_account_uri"

		uri, err := utils.ParseSessionsURI(connector.GetGoogleServiceAccountURI())
		if err != nil {
			return nil, trace.BadParameter("failed to parse google_service_account_uri: %v", err)
		}
		jsonCredentials, err = os.ReadFile(uri.Path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else if connector.GetGoogleServiceAccount() != "" {
		// load the google service account from string
		credentialLoadingMethod = "google_service_account"
		jsonCredentials = []byte(connector.GetGoogleServiceAccount())
	}

	// The "Admin SDK Directory API" needs admin delegation (see
	// https://developers.google.com/admin-sdk/directory/v1/guides/delegation
	// and
	// https://developers.google.com/identity/protocols/oauth2/service-account#delegatingauthority )
	// and the "Cloud Identity API" needs an account with View permission on
	// all groups to work reliably.
	credentialsParams := google.CredentialsParams{
		Subject: connector.GetGoogleAdminEmail(),
	}

	if connector.GetGoogleTransitiveGroups() {
		log.Debugf("Loading credentials to fetch transitive groups")
		credentialsParams.Scopes = []string{cloudidentity.CloudIdentityGroupsReadonlyScope}
	} else {
		log.Debugf("Loading credentials to fetch direct groups")
		credentialsParams.Scopes = []string{directory.AdminDirectoryGroupReadonlyScope}
	}

	credentials, err := google.CredentialsFromJSONWithParams(ctx, jsonCredentials, credentialsParams)
	if err != nil {
		return nil, trace.BadParameter("unable to parse google service account from %v: %v", credentialLoadingMethod, err)
	}

	return credentials, nil
}

func groupsFromGsuiteDirectory(ctx context.Context, email string, clientOptions ...option.ClientOption) ([]string, error) {
	service, err := directory.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var groups []string
	err = service.Groups.List().
		UserKey(email).
		Pages(ctx, func(resp *directory.Groups) error {
			if resp == nil {
				return nil
			}
			for _, g := range resp.Groups {
				if g != nil && g.Email != "" {
					groups = append(groups, g.Email)
				}
			}
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return groups, nil
}

func groupsFromGsuiteCloudidentity(ctx context.Context, email string, clientOptions ...option.ClientOption) ([]string, error) {
	service, err := cloudidentity.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var groups []string
	err = service.Groups.Memberships.SearchTransitiveGroups("groups/-").
		// the google API docs claim that the query string is a CEL expression
		// (https://opensource.google/projects/cel) but the call will fail if
		// you use double quotes instead of single quotes in spite of them being
		// equivalent according to the CEL specs
		Query(fmt.Sprintf("member_key_id == '%s' && 'cloudidentity.googleapis.com/groups.discussion_forum' in labels", email)).
		Pages(ctx, func(resp *cloudidentity.SearchTransitiveGroupsResponse) error {
			if resp == nil {
				return nil
			}
			for _, g := range resp.Memberships {
				if g != nil && g.GroupKey != nil && g.GroupKey.Id != "" {
					groups = append(groups, g.GroupKey.Id)
				}
			}
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return groups, nil
}
