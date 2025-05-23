package common

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
)

var (
	// readOAuthScopes required for User sync or read-only integration.
	readOAuthScopes = []string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeGroupsRead,
	}
	// allOAuthScopes required when bidirectional sync is enabled.
	allOAuthScopes = append(
		readOAuthScopes,
		oktaapi.ScopeAppsManage,
		oktaapi.ScopeGroupsManage,
	)
	siemScopes = []string{
		oktaapi.ScopeOktaLogsRead,
		oktaapi.ScopeOktaAPITokensRead,
		oktaapi.ScopeRolesRead,
	}
)

// GetReadOnlyOAuthScopes returns the minimal Okta client scopes for OAuth credentials required
// for:
//   - SCIM
//   - User sync
//   - App and Group sync
//   - read-only Access List sync
func GetReadOnlyOAuthScopes() []string {
	return readOAuthScopes[:]
}

// GetSIEMOAuthScopes returns the Okta client scopes for OAuth credentials required
// for:
//   - read-only logs
//   - read-only API tokens
//   - read-only roles
func GetSIEMOAuthScopes() []string {
	return siemScopes[:]
}

// GetOAuthScopesForSyncSettings determines Okta OAuth scopes required for the given sync level.
// Consider using [GetOAuthScopesForIntegrationRequest] first.
func GetOAuthScopesForSyncSettings(syncSettings SyncSettings) []string {
	if syncSettings.GetEnableBidirectionalSync() {
		scopes := allOAuthScopes[:]
		if syncSettings.GetEnableSystemLogExport() {
			scopes = append(scopes, siemScopes...)
		}
		return scopes
	}
	// User sync is required for all other sync levels so it's enough to only check that.
	if syncSettings.GetEnableUserSync() {
		scopes := readOAuthScopes[:]
		if syncSettings.GetEnableSystemLogExport() {
			scopes = append(scopes, siemScopes...)
		}
		return scopes
	}
	return nil
}

// GetOAuthScopesForIntegrationRequest extends [GetOAuthScopesForSyncSettings] with determining
// OAuth scopes for SCIM (if enabled).
func GetOAuthScopesForIntegrationRequest(req IntegrationRequest) []string {
	scopes := GetOAuthScopesForSyncSettings(req)
	if len(scopes) != 0 {
		return scopes
	}
	if req.GetScimToken() != "" {
		return readOAuthScopes[:]
	}
	return nil
}

// CheckClientOAuthScopes verifies the Okta client's OAuth credentials are authorized to the given
// scopes.
func CheckClientOAuthScopes(ctx context.Context, oktaClient AuthorizedScopesGetter, scopes ...string) error {
	if len(scopes) == 0 {
		return trace.BadParameter("no OAuth scopes to check")
	}
	authorizedScopes, err := oktaClient.GetAuthorizedScopes(ctx)
	if err != nil {
		return trace.Wrap(err, "getting authorized OAuth scopes from Okta")
	}
	var errs []error
	for _, s := range scopes {
		if !slices.Contains(authorizedScopes, s) {
			errs = append(errs, trace.BadParameter("scope %q missing", s))
		}
	}
	return trace.NewAggregate(errs...)
}
