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

const (
	// googleWorkspaceIssuerURL is the issuer URL for Google Workspace accounts.
	googleWorkspaceIssuerURL = "https://accounts.google.com"

	// googleGroupsClaim is the OIDC claim that we inject into the claims
	// returned for Google Workspace users, containing the email addresses of
	// the Google Groups that the user belongs to.
	googleGroupsClaim = "groups"
)

// isGoogleWorkspaceConnector returns true if the connector is a OIDC connector
// for Google Workspace, configured to fetch extra claims.
func isGoogleWorkspaceConnector(connector types.OIDCConnector) bool {
	// If google_service_account_uri and google_service_account are not set, we
	// assume that this is a non-Google Workspace OIDC provider using the same
	// issuer URL as Google Workspace (e.g.
	// https://developers.google.com/identity/protocols/oauth2/openid-connect).
	return connector.GetIssuerURL() == googleWorkspaceIssuerURL &&
		(connector.GetGoogleServiceAccountURI() != "" || connector.GetGoogleServiceAccount() != "")
}

// addGoogleWorkspaceClaims will fetch extra data from proprietary Google APIs
// and it will add claims based on the fetched data. The current implementation
// adds a "groups" claim containing the Google Groups that the user is a member
// of.
func addGoogleWorkspaceClaims(ctx context.Context, connector types.OIDCConnector, claims jose.Claims) (jose.Claims, error) {
	email, exists, err := claims.StringClaim("email")
	if err != nil || !exists {
		return nil, trace.BadParameter("no `email` in oauth claims for Google Workspace account")
	}

	credentials, err := getGoogleWorkspaceCredentials(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var googleGroups []string
	if connector.GetGoogleTransitiveGroups() {
		googleGroups, err = groupsFromGoogleCloudIdentity(ctx, email, option.WithCredentials(credentials))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		googleGroups, err = groupsFromGoogleDirectory(ctx, email, "", option.WithCredentials(credentials))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if len(googleGroups) > 0 {
		googleClaims := jose.Claims{googleGroupsClaim: googleGroups}
		log.Debugf("Claims from Google Workspace: %v.", googleClaims)
		claims, err = mergeClaims(claims, googleClaims)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		log.Debugf("No Google Workspace claims.")
	}

	return claims, nil
}

func getGoogleWorkspaceCredentials(ctx context.Context, connector types.OIDCConnector) (*google.Credentials, error) {
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

func groupsFromGoogleDirectory(ctx context.Context, email, filterDomain string, clientOptions ...option.ClientOption) ([]string, error) {
	service, err := directory.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	call := service.Groups.List().UserKey(email)
	if filterDomain != "" {
		call = call.Domain(filterDomain)
	}

	var groups []string
	err = call.Pages(ctx, func(resp *directory.Groups) error {
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

func groupsFromGoogleCloudIdentity(ctx context.Context, email string, clientOptions ...option.ClientOption) ([]string, error) {
	service, err := cloudidentity.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// SearchTransitiveGroups takes a fixed parameter as part of the URL
	// ("Format: `groups/{group}`, where `group` is always '-'") and a query
	// parameter that the google API docs claim to be a CEL expression
	// (https://opensource.google/projects/cel) that filters the results based
	// on `member_key_id`, optionally `member_key_namespace`, and `labels`. The
	// query parameter doesn't seem to actually be a CEL expression, and even
	// changing the single quotes into double quotes (which is fine according to
	// the CEL grammar) makes every API call fail with an "Unauthorized" error
	// message.
	//
	// The query string was lifted directly from
	// https://cloud.google.com/identity/docs/how-to/query-memberships#searching_for_all_group_memberships_of_a_member
	// and some more informations on group labels can be found at
	// https://cloud.google.com/identity/docs/groups#group_properties .
	// The actual docs for the API call are at
	// https://cloud.google.com/identity/docs/reference/rest/v1/groups.memberships/searchTransitiveGroups .
	call := service.Groups.Memberships.SearchTransitiveGroups("groups/-").
		Query(fmt.Sprintf("member_key_id == '%s' && 'cloudidentity.googleapis.com/groups.discussion_forum' in labels", email))

	var groups []string
	err = call.Pages(ctx, func(resp *cloudidentity.SearchTransitiveGroupsResponse) error {
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
