package sso

import (
	"cmp"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/defaults"
)

// SAMLConnectorService defines an interface for querying and creating SAML connectors in the
// Teleport cluster. It is supposed to be the Auth Server.
type SAMLConnectorService interface {
	// CreateSAMLConnector creates a SAML connector
	CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error)
	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
}

// ConnectorArgs supplies all of the parameters required for creating both ends
// of the SSO connector.
type ConnectorArgs struct {
	// OktaClient is our connection to the upstream Okta organization.
	OktaClient oktaapi.Interface

	// SAMLConnectorService handles querying and creating SAML SSO connectors in
	// the Teleport cluster
	SAMLConnectorService SAMLConnectorService

	// The name of the teleport cluster, used to generate the Okta App name
	ClusterName string

	// Name to give the SAML SSO connector we create
	ConnectorName string

	// DisplayName is the human-readable display name for the SAML SSO connector.
	// Falls back to ConnectorName if not set.
	DisplayName string

	// The public URL of the cluster proxy. Used to generate URLs to give the
	// Okta app (e.g. login redirection, etc).
	PublicURL *url.URL

	// Log receives any logging output
	Logger *slog.Logger

	// SigningKeypair is an optional keypair to use for the SAML connector. A sensible,
	// secure default will be generated if none is supplied.
	SigningKeypair *types.AsymmetricKeyPair

	// MetadataURL is the URL to fetch the SAML metadata from
	// the Okta app.
	MetadataURL string

	// HTTPClient is the HTTP client to use for fetching the metadata.
	HTTPClient http.RoundTripper
}

func (a *ConnectorArgs) Check() error {
	if a.SAMLConnectorService == nil {
		return trace.BadParameter("missing SSO connector parameter SAMLConnectorService")
	}
	if a.ClusterName == "" {
		return trace.BadParameter("missing SSO connector parameter SAMLConnectorService")
	}
	if a.ConnectorName == "" {
		return trace.BadParameter("missing SSO connector parameter ConnectorName")
	}
	if a.PublicURL == nil {
		return trace.BadParameter("missing SSO connector parameter PublicURL")
	}
	if a.Logger == nil {
		return trace.BadParameter("missing SSO connector parameter Logger")
	}
	if a.HTTPClient == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
		a.HTTPClient = tr
	}
	return nil
}

// SAMLConnectorInfo holds data about the created SSO connector and underlying
// Okta SAML app. Returned from CreateSSOConnector().
type SAMLConnectorInfo struct {
	Connector    types.SAMLConnector
	OktaAppID    string
	OktaAppLabel string
	OktaAppName  string
	OktaOrg      string
}

// CreateSAMLConnectorFromMetadataURL creates a new SAML connector in Teleport based on the OKta SAML Application metadataURL.
func CreateSAMLConnectorFromMetadataURL(ctx context.Context, args ConnectorArgs) (*SAMLConnectorInfo, error) {
	idpMetadata, err := fetchSSOIdPMetadata(ctx, args.MetadataURL, args.HTTPClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := types.NewSAMLConnector(args.ConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: args.PublicURL.JoinPath("/v1/webapi/saml/acs", args.ConnectorName).String(),
		Display:                  cmp.Or(args.DisplayName, args.ConnectorName),
		EntityDescriptor:         string(idpMetadata),
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "groups",
				Value: oktaapi.OktaGroupEveryone,
				Roles: []string{teleport.SystemOktaRequesterRoleName},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaOrg, err := ExtractOktaOrganizationFromURL(args.MetadataURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	meta := connector.GetMetadata()
	meta.Labels = map[string]string{
		types.OriginLabel:         types.OriginOkta,
		eteleport.OktaOrgURLLabel: oktaOrg,
	}

	connector.SetMetadata(meta)
	if _, err := args.SAMLConnectorService.CreateSAMLConnector(ctx, connector); err != nil {
		return nil, trace.Wrap(err, "creating Okta SAML connector")
	}

	info := &SAMLConnectorInfo{
		Connector: connector,
		OktaOrg:   oktaOrg,
	}
	return info, nil
}

// FetchOktaSAMLConnectorInfo tries to extract Okta app ID for the Okta SAML app for the given
// connector. It first tries to get it from annotation but if not available it tries to extract the
// Okta app name it from SAML connector SSO URL and then query Okta to retrieve tha app ID.
func FetchOktaSAMLConnectorInfo(ctx context.Context, oktaClient oktaapi.Interface, samlConnector types.SAMLConnector) (*SAMLConnectorInfo, error) {
	if samlConnector.GetSSO() == "" {
		return nil, trace.BadParameter("SAML connector %q has not SSO URL set", samlConnector.GetName())
	}
	oktaAppName, err := ExtractOktaAppNameFromSsoUrl(samlConnector.GetSSO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaAppId := ""
	oktaAppLabel := ""
	err = oktaClient.IterateApps(
		ctx,
		func(a okta.App) error {
			var ok bool
			app, ok := a.(*okta.Application)
			if !ok {
				return trace.BadParameter("unable to process Okta application of unknown type %T", a)
			}
			if oktaAppId != "" {
				return trace.BadParameter("this is a bug: more than one Okta App [%q, %q] found for App name = %q", app.Id, oktaAppId, oktaAppName)
			}
			oktaAppId = app.Id
			oktaAppLabel = app.Label
			return nil
		},
		query.WithQ(oktaAppName),
	)
	if trace.IsAccessDenied(err) {
		// The scopes are checked on top of this function so it's probably admin role permission.
		return nil, trace.Wrap(err, `Insufficient Okta API permissions to list Okta apps. Ensure "View application and their details" permission is set to the custom admin role assigned to the provided Okta credential.`)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	if oktaAppId == "" {
		return nil, trace.NotFound("No Okta App for Okta App name = %q found. This could be a problem with the app excluded from the Okta resource set.", oktaAppName)
	}
	return &SAMLConnectorInfo{
		Connector:    samlConnector,
		OktaOrg:      oktaClient.GetOrgUrl(),
		OktaAppID:    oktaAppId,
		OktaAppName:  oktaAppName,
		OktaAppLabel: oktaAppLabel,
	}, nil
}

// ExtractOktaAppNameFromSsoUrl extracts Okta app name from SAML location URL stored in saml.spec.sso field.
//
// Example SSO URL for Okta:
// https://example.com/app/trial-123456_teleportsamlconnectorapp_1/exkmtd8mclmbcpx01697/sso/saml
func ExtractOktaAppNameFromSsoUrl(ssoUrlText string) (string, error) {
	ssoUrl, err := url.Parse(ssoUrlText)
	if err != nil {
		return "", trace.Wrap(err)
	}

	ssoUrlPathSegments := strings.Split(strings.Trim(ssoUrl.Path, "/"), "/")
	if len(ssoUrlPathSegments) != 5 {
		return "", trace.BadParameter("expected 5 path segments but found (%d) = %v",
			len(ssoUrlPathSegments), ssoUrlPathSegments)
	}
	for _, test := range []struct {
		index int
		value string
	}{
		{index: 0, value: "app"},
		{index: 3, value: "sso"},
		{index: 4, value: "saml"},
	} {
		if ssoUrlPathSegments[test.index] != test.value {
			return "", trace.BadParameter("expected path segment (%d) to be %q", test.index+1, test.value)
		}
	}

	return ssoUrlPathSegments[1], nil
}

func fetchSSOIdPMetadata(ctx context.Context, metadataURL string, rt http.RoundTripper) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	httpClient := http.Client{
		Transport: rt,
	}
	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, trace.Wrap(err, "fetching SAML app entity metadata")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("failed to fetch IdP metadata from the %q URL, http status: %s", metadataURL, resp.Status)
	}
	metadata, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err, "reading metadata response")
	}
	return metadata, nil
}

// ValidateSAMLConnector examines SAML Auth connector to see if it is configured
// for use with the Okta integration and extracts the appropriate metadata.
func ValidateSAMLConnector(ctx context.Context, oktaClient oktaapi.Interface, connector types.SAMLConnector) (*SAMLConnectorInfo, error) {
	oktaOrg, err := getOktaOrgFromSAMLConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if oktaOrg != oktaClient.GetOrgUrl() {
		return nil, trace.BadParameter("SAML connector %q bound to %q Okta organization but expected %q", connector.GetName(), oktaOrg, oktaClient.GetOrgUrl())
	}

	info, err := FetchOktaSAMLConnectorInfo(ctx, oktaClient, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := connector.GetMetadata().Labels
	connectorAppID, present := labels[eteleport.OktaAppIDLabel]
	if present && info.OktaAppID != connectorAppID {
		return nil, trace.BadParameter("SAML connector app ID label value is %q but found Okta SAML app ID is %q", connectorAppID, info.OktaAppID)
	}

	// It would be nice to check if the app is of type *okta.SamlApplication here, but it will
	// cost us another request and doesn't bring that much value.

	return info, nil
}

func getOktaOrgFromSAMLConnector(connector types.SAMLConnector) (string, error) {
	if connector.GetSSO() == "" {
		return "", trace.BadParameter("SAML connector %q has not SSO URL set", connector.GetName())
	}
	labels := connector.GetMetadata().Labels
	if org, ok := labels[eteleport.OktaOrgURLLabel]; ok {
		return org, nil
	}
	org, err := ExtractOktaOrganizationFromURL(connector.GetSSO())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return org, nil
}

func extractMetadataURL(input any) (*url.URL, string, error) {
	links := oktaapi.EmbeddedLinks{}
	if err := mapstructure.Decode(input, &links); err != nil {
		return nil, "", trace.Wrap(err, "parsing embedded links")
	}

	if links.Metadata == nil {
		return nil, "", trace.BadParameter("missing metadata")
	}

	if links.Metadata.Href == "" {
		return nil, "", trace.BadParameter("missing href")
	}

	url, err := url.Parse(links.Metadata.Href)
	if err != nil {
		return nil, "", trace.Wrap(err, "parsing metadata URL")
	}

	return url, links.Metadata.Type, nil
}

// ExtractOktaOrganizationFromURL extracts the Okta organization URL from the
// given Okta URL.
func ExtractOktaOrganizationFromURL(oktaURL string) (string, error) {
	u, err := url.Parse(oktaURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return u.Scheme + "://" + u.Host, nil
}
