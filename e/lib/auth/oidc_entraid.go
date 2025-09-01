package auth

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

func isEntraIDConnector(connector types.OIDCConnector) bool {
	issuer, err := url.Parse(connector.GetIssuerURL())
	if err != nil {
		// Error swallowed as Entra ID issuer url is expected
		// to be a valid URL and there is nothing to handle if
		// this is not an Entra ID connector.
		return false
	}

	return issuer.Host == entraIDIssuerHost
}

type entraIDGroupsProvider struct {
	connector  types.OIDCConnector
	idToken    *oidc.Tokens[*oidc.IDTokenClaims]
	logger     *slog.Logger
	httpClient *http.Client
}

// maybeFetchEntraIDGroups lists group from Microsoft
// Graph API if a groups claim source is found in the OIDC
// claim and Entra ID groups provider is not disabled in the
// OIDC connector spec.
// "groups" claim source is only expected when user's
// group membership count exceeds 200 item limit of
// the Entra ID OIDC groups claim.
// https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims
func (p entraIDGroupsProvider) maybeFetchEntraIDGroups(ctx context.Context, graphClient *msgraph.Client) error {
	if p.connector.IsEntraIDGroupsProviderDisabled() {
		return nil
	}

	// Per Microsoft docs, we only need to find group claim sources if
	// "groups" claim is not found.
	// https://learn.microsoft.com/en-us/security/zero-trust/develop/configure-tokens-group-claims-app-roles#group-overages
	_, ok := p.idToken.IDTokenClaims.Claims[entraIDGroupsClaim]
	if ok {
		return nil
	}

	userID, ok := p.idToken.IDTokenClaims.Claims["oid"].(string)
	if !ok {
		return trace.NotFound("oid claim not found")
	}

	claimName, err := entraGroupClaimName(p.idToken.IDTokenClaims.Claims)
	if err != nil {
		if trace.IsNotFound(err) {
			// Its possible for a user to have zero group membership,
			// resulting in an empty groups claim and empty claim sources.
			logger.DebugContext(ctx, "Missing both groups claim and groups source claim", "oid", userID, "error", err)
			return nil
		}
		return trace.Wrap(err)
	}

	// We are only interested to check if group source claim exists.
	// The endpoint returned by Entra ID points to now-retired
	// Azure AD Graph endpoint (graph.windows.net) and we will always build a
	// new endpoint that points to newer Microsoft Graph API national cloud
	// deployment endpoint.
	if _, err := entraClaimSourceEndpoint(p.idToken.IDTokenClaims.Claims, claimName); err != nil {
		return trace.Wrap(err)
	}

	if graphClient == nil {
		graphClient, err = newGraphClient(p.connector, p.idToken.IDTokenClaims.Claims, p.httpClient)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var groups []string
	groupType := getGroupType(p.connector)
	if err = graphClient.IterateUsersTransitiveMemberOf(ctx, userID, groupType, func(group *msgraph.Group) bool {
		if group == nil {
			return false
		}
		if group.ID == nil {
			return false
		}
		groups = append(groups, *group.ID)
		return true
	}); err != nil {
		return trace.Wrap(err)
	}

	if len(groups) > 0 {
		p.idToken.IDTokenClaims.Claims[entraIDGroupsClaim] = groups
	}

	return nil
}

// getGroupsClaimName returns "groups" source name from OIDC _claim_names claim.
// https://openid.net/specs/openid-connect-core-1_0.html#AggregatedDistributedClaims
// Sample:
//
//	{
//	  "_claim_names": {
//	    "groups": "src1"
//	  }
//	}
func entraGroupClaimName(claims map[string]any) (string, error) {
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

// getGroupsClaimSourceEndpoint returns "groups" source from OIDC _claim_sources claim.
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
func entraClaimSourceEndpoint(claims map[string]any, name string) (string, error) {
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

func newGraphClient(
	connector types.OIDCConnector,
	claims map[string]any,
	httpClient *http.Client,
) (*msgraph.Client, error) {
	tenantID, ok := claims["tid"].(string)
	if !ok {
		return nil, trace.NotFound("tid claim not found")
	}

	tokenProvider, err := azidentity.NewClientSecretCredential(tenantID, connector.GetClientID(), connector.GetClientSecret(), nil /* ClientSecretCredentialOptions */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := msgraph.NewClient(msgraph.Config{
		TokenProvider: tokenProvider,
		GraphEndpoint: getGraphEndpoint(connector),
		HTTPClient:    httpClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func getGraphEndpoint(connector types.OIDCConnector) string {
	graphEndpoint := types.MSGraphDefaultEndpoint
	entra := connector.GetEntraIDGroupsProvider()
	if entra == nil {
		return graphEndpoint
	}
	if entra.GraphEndpoint != "" {
		graphEndpoint = entra.GraphEndpoint
	}
	return graphEndpoint
}

func getGroupType(connector types.OIDCConnector) string {
	groupType := types.EntraIDSecurityGroups
	entra := connector.GetEntraIDGroupsProvider()
	if entra == nil {
		return groupType
	}
	if entra.GroupType != "" {
		groupType = entra.GroupType
	}
	return groupType
}
