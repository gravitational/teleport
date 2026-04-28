package entraid

import (
	"context"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

const (
	// entraIDAttrGroupsOverageLink is the SAML attribute present when the user has more than 150 group memberships.
	entraIDAttrGroupsOverageLink = "http://schemas.microsoft.com/claims/groups.link"
	// entraIDAttrGroups is the SAML attribute containing the user's group memberships.
	entraIDAttrGroups = "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
	// entraIDAttrObjectIdentifier is the SAML attribute containing the user's OID.
	entraIDAttrObjectIdentifier = "http://schemas.microsoft.com/identity/claims/objectidentifier"
	// entraIDAttrTenantID is the SAML attribute containing the Entra tenant ID.
	entraIDAttrTenantID = "http://schemas.microsoft.com/identity/claims/tenantid"
)

// SAMLEntraIDGroupsProviderConfig holds the dependencies for the samlEntraIDGroupsProvider.
type SAMLEntraIDGroupsProviderConfig struct {
	// Connector is the SAML Connector for the current auth request.
	Connector types.SAMLConnector
	// AssertionInfo is the SAML assertion info for the current auth request.
	AssertionInfo *saml2.AssertionInfo
	// Logger provides the Logger to use.
	// If omitted, calling checkAndSetDefaults will create a default Logger.
	Logger *slog.Logger
	// NewGraphClient is a constructor for an MS Graph API client.
	// If omitted, calling checkAndSetDefaults uses NewGraphClient as default.
	NewGraphClient func(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error)
	// ResolveToken is a function to resolve a token credential from a SAML connector and assertion.
	ResolveToken func(ctx context.Context, connector types.SAMLConnector, assertionInfo *saml2.AssertionInfo) (azcore.TokenCredential, error)
}

// checkAndSetDefaults validates the fields on samlEntraIDGroupsProvider.
// For any missing required fields, it returns an error.
// For any missing optional fields, it sets a default value.
func (c *SAMLEntraIDGroupsProviderConfig) checkAndSetDefaults() error {
	switch {
	case c.Connector == nil:
		return trace.BadParameter("connector is required")
	case c.AssertionInfo == nil:
		return trace.BadParameter("assertionInfo is required")
	case c.ResolveToken == nil:
		return trace.BadParameter("resolveToken is required")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, "saml-entraid-groups-provider"))
	}
	c.Logger = c.Logger.With("connector", c.Connector.GetName())

	if c.NewGraphClient == nil {
		// Wrap default newGraphClient to pass nil for unused httpClient.
		c.NewGraphClient = func(tokenProvider azcore.TokenCredential, graphEndpoint string) (*msgraph.Client, error) {
			return newGraphClient(tokenProvider, graphEndpoint, nil)
		}
	}

	return nil
}

// SAMLEntraIDGroupsProvider is used to fetch a user's group memberships using MS Graph API when a SAML
// assertion indicates groups overage (membership of 150+ groups) and add them back to the SAML assertion.
type SAMLEntraIDGroupsProvider struct {
	SAMLEntraIDGroupsProviderConfig
}

// NewSAMLEntraIDGroupsProvider creates a samlEntraIDGroupsProvider with the given config.
func NewSAMLEntraIDGroupsProvider(cfg SAMLEntraIDGroupsProviderConfig) (*SAMLEntraIDGroupsProvider, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SAMLEntraIDGroupsProvider{cfg}, nil
}

// MaybeFetchEntraIDGroups fetches the groups and sets them on the groups attribute on the SAML assertion.
// If the provider is disabled or there is no groups overage, calling maybeFetchEntraIDGroups is
// a no-op and returns nil.
func (p SAMLEntraIDGroupsProvider) MaybeFetchEntraIDGroups(ctx context.Context) error {
	if p.Connector.IsEntraIDGroupsProviderDisabled() {
		p.Logger.DebugContext(ctx, "Entra ID groups provider is disabled")
		return nil
	}

	if !HasSAMLGroupsOverage(p.AssertionInfo) {
		return nil
	}

	oid, err := p.userObjectID()
	if err != nil {
		return trace.Wrap(err)
	}

	tokenProvider, err := p.ResolveToken(ctx, p.Connector, p.AssertionInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	groupsProvider := p.Connector.GetEntraIDGroupsProvider()

	graphClient, err := p.NewGraphClient(tokenProvider, getGraphEndpoint(groupsProvider))
	if err != nil {
		return trace.Wrap(err)
	}

	groups, err := getEntraGroups(ctx, graphClient, getEntraGroupType(groupsProvider), oid)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(groups) == 0 {
		p.Logger.DebugContext(ctx, "Zero supported Entra ID groups found with groups overage", "oid", oid)
		return nil
	}

	p.setGroups(groups)

	return nil
}

// setGroups sets the provided groups on the groups attribute of the SAML assertion.
func (p SAMLEntraIDGroupsProvider) setGroups(groups []string) {
	p.AssertionInfo.Values[entraIDAttrGroups] = samltypes.Attribute{
		Name: entraIDAttrGroups,
		Values: sliceutils.Map(groups, func(g string) samltypes.AttributeValue {
			return samltypes.AttributeValue{Value: g}
		}),
	}
}

// userObjectID returns the value of the object identifier attribute from the SAML assertion.
// If the attribute is missing then an error is returned.
func (p SAMLEntraIDGroupsProvider) userObjectID() (string, error) {
	oidAttr, ok := p.AssertionInfo.Values[entraIDAttrObjectIdentifier]
	if !ok {
		return "", trace.NotFound("objectidentifier attribute not found")
	}

	if len(oidAttr.Values) == 0 {
		return "", trace.NotFound("objectidentifier attribute is empty")
	}

	return oidAttr.Values[0].Value, nil
}

// HasSAMLGroupsOverage checks if the assertion contains the "groups.link" attribute
// which signals groups overage.
func HasSAMLGroupsOverage(assertionInfo *saml2.AssertionInfo) bool {
	_, ok := assertionInfo.Values[entraIDAttrGroupsOverageLink]
	return ok
}
