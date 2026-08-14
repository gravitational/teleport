package entraid

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph"
)

const (
	entraIDIssuerHost = "login.microsoftonline.com"
	// entraIDGroupsClaim defines groups claim name used by Entra ID.
	entraIDGroupsClaim = "groups"
)

// Claim name and source are standard values based on OIDC 1.0 spec
// https://openid.net/specs/openid-connect-core-1_0.html#AggregatedDistributedClaims.
const (
	// oidcClaimName defines name of the claim object that defines the name of the "groups" claim source.
	oidcClaimName = "_claim_names"
	// oidcClaimSource defines name of the claim object that contains the "groups" claim source.
	oidcClaimSource = "_claim_sources"
)

// oidcDistributedClaimSource defines claim source object based on OIDC 1.0 spec
// https://openid.net/specs/openid-connect-core-1_0.html#AggregatedDistributedClaims.
// Note: The claim source object may also contain accessToken field.
// Entra ID does not include accessToken for groups claim source
// so the field is not defined below.
type oidcDistributedClaimSource struct {
	// Endpoint is a Graph API endpoint to fetch user groups.
	Endpoint string `json:"endpoint"`
}

func IsEntraIDConnector(connector types.OIDCConnector) bool {
	issuer, err := url.Parse(connector.GetIssuerURL())
	if err != nil {
		// Error swallowed as Entra ID issuer url is expected
		// to be a valid URL and there is nothing to handle if
		// this is not an Entra ID connector.
		return false
	}

	return issuer.Host == entraIDIssuerHost
}

type OIDCEntraIDGroupsProvider struct {
	Connector  types.OIDCConnector
	IDToken    *oidc.Tokens[*oidc.IDTokenClaims]
	Logger     *slog.Logger
	HTTPClient *http.Client
}

// MaybeFetchEntraIDGroups lists group from Microsoft
// Graph API if a groups claim source is found in the OIDC
// claim and Entra ID groups provider is not disabled in the
// OIDC connector spec.
// "groups" claim source is only expected when user's
// group membership count exceeds 200 item limit of
// the Entra ID OIDC groups claim.
// https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims
func (p OIDCEntraIDGroupsProvider) MaybeFetchEntraIDGroups(ctx context.Context, graphClient *msgraph.Client) error {
	if p.Connector.IsEntraIDGroupsProviderDisabled() {
		return nil
	}

	// Per Microsoft docs, we only need to find group claim sources if
	// "groups" claim is not found.
	// https://learn.microsoft.com/en-us/security/zero-trust/develop/configure-tokens-group-claims-app-roles#group-overages
	_, ok := p.IDToken.IDTokenClaims.Claims[entraIDGroupsClaim]
	if ok {
		return nil
	}

	userID, ok := p.IDToken.IDTokenClaims.Claims["oid"].(string)
	if !ok {
		return trace.NotFound("oid claim not found")
	}

	claimName, err := entraGroupsClaimName(p.IDToken.IDTokenClaims.Claims)
	if err != nil {
		if trace.IsNotFound(err) {
			// Its possible for a user to have zero group membership,
			// resulting in an empty groups claim and empty claim sources.
			p.Logger.DebugContext(ctx, "Missing both groups claim and groups source claim", "oid", userID, "error", err)
			return nil
		}
		return trace.Wrap(err)
	}

	// We are only interested to check if group source claim exists.
	// The endpoint returned by Entra ID points to now-retired
	// Azure AD Graph endpoint (graph.windows.net) and we will always build a
	// new endpoint that points to newer Microsoft Graph API national cloud
	// deployment endpoint.
	if _, err := entraGroupsClaimSourceEndpoint(p.IDToken.IDTokenClaims.Claims, claimName); err != nil {
		return trace.Wrap(err)
	}

	tenantID, ok := p.IDToken.IDTokenClaims.Claims["tid"].(string)
	if !ok {
		return trace.NotFound("tid claim not found")
	}

	tokenProvider, err := azidentity.NewClientSecretCredential(tenantID, p.Connector.GetClientID(), p.Connector.GetClientSecret(), nil /* ClientSecretCredentialOptions */)
	if err != nil {
		return trace.Wrap(err)
	}

	groupsProvider := p.Connector.GetEntraIDGroupsProvider()

	if graphClient == nil {
		graphClient, err = newGraphClient(tokenProvider, getGraphEndpoint(groupsProvider), p.HTTPClient)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	groups, err := getEntraGroups(ctx, graphClient, getEntraGroupType(groupsProvider), userID)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(groups) > 0 {
		p.IDToken.IDTokenClaims.Claims[entraIDGroupsClaim] = groups
	}

	return nil
}

// entraGroupsClaimName returns "groups" source name from OIDC _claim_names claim.
// https://openid.net/specs/openid-connect-core-1_0.html#AggregatedDistributedClaims
// Sample:
//
//	{
//	  "_claim_names": {
//	    "groups": "src1"
//	  }
//	}
func entraGroupsClaimName(claims map[string]any) (string, error) {
	rawClaimNames, ok := claims[oidcClaimName]
	if !ok {
		return "", trace.NotFound("%q not found", oidcClaimName)
	}
	claimNamesBytes, err := json.Marshal(rawClaimNames)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var claimNames map[string]string
	if err = json.Unmarshal(claimNamesBytes, &claimNames); err != nil {
		return "", trace.Wrap(err)
	}
	groupsClaimName, ok := claimNames[entraIDGroupsClaim]
	if !ok {
		return "", trace.NotFound("groups claim name not found")
	}

	return groupsClaimName, nil
}

// entraGroupsClaimSourceEndpoint returns "groups" source from OIDC _claim_sources claim.
// Group source object lookup is based on source name defined in the _claim_names claim.
// https://openid.net/specs/openid-connect-core-1_0.html#AggregatedDistributedClaims
// Sample:
//
//	{
//	  "_claim_sources": {
//	    "src1": {
//	      "endpoint": "https://graph.windows.net/<tenant-id>/users/<user-id>/getMemberObjects"
//	    }
//	  }
//	}
func entraGroupsClaimSourceEndpoint(claims map[string]any, name string) (string, error) {
	rawClaimSources, ok := claims[oidcClaimSource]
	if !ok {
		return "", trace.BadParameter("claim sources not found")
	}
	claimSourcesBytes, err := json.Marshal(rawClaimSources)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var sources map[string]oidcDistributedClaimSource
	err = json.Unmarshal(claimSourcesBytes, &sources)
	if err != nil {
		return "", trace.Wrap(err)
	}

	groupsClaimSource, ok := sources[name]
	if !ok {
		return "", trace.BadParameter("missing %q claim source", name)
	}
	if groupsClaimSource.Endpoint == "" {
		return "", trace.BadParameter("empty value for groups claim source endpoint")
	}

	return groupsClaimSource.Endpoint, nil
}
