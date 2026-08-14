package common

import "context"

// AuthorizedScopesGetter a stripped down interface for [oktaapi.Client].
type AuthorizedScopesGetter interface {
	// GetAuthorizedScopes verifies and returns the list of configured OAuth scopes trimmed
	// down to those allowed by the configured credentials.
	GetAuthorizedScopes(ctx context.Context) ([]string, error)
}

// SyncSettings is implemented by:
//
//   - *types.PluginOktaSyncSettings
//
// It allows retrieving [GetOAuthScopesForSyncSettings] for each of the types above.
//
// Note: It's also implemented by *oktav1.*IntegrationRequest, but
// [GetOAuthScopesForIntegrationRequest] should be used for those.
type SyncSettings interface {
	GetEnableUserSync() bool
	GetEnableAppGroupSync() bool
	GetEnableAccessListSync() bool
	GetEnableBidirectionalSync() bool
	GetEnableSystemLogExport() bool
}

// IntegrationRequest is implemented by:
//
//   - *oktav1.CreateIntegrationRequest
//   - *oktav1.UpdateIntegrationRequest
//
// It allows retrieving [GetOAuthScopesForIntegrationRequest] for each of the types above.
type IntegrationRequest interface {
	SyncSettings
	GetScimToken() string
}
