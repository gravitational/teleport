package sso

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/api"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// SAMLConnectorService defines an interface for querying and creating
// SAML connectors in the Teleport cluster
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
	OktaClient api.Client

	// SAMLConnectorService handles querying and creating SAML SSO connectors in
	// the Teleport cluster
	SAMLConnectorService SAMLConnectorService

	// The name of the teleport cluster, used to generate the Okta App name
	ClusterName string

	// Name to give the SAML SSO connector we create
	ConnectorName string

	// The public URL of the cluster proxy. Used to generate URLs to give the
	// Okta app (e.g. login redirection, etc).
	PublicURL *url.URL

	// Log receives any logging output
	Logger *slog.Logger

	// SigningKeypair is an optional keypair to use for the SAML connector. A sensible,
	// secure default will be generated if none is supplied.
	SigningKeypair *types.AsymmetricKeyPair
}

func (a *ConnectorArgs) Check() error {
	if a.OktaClient == nil {
		return trace.BadParameter("missing SSO connector parameter OktaClient")
	}
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
	return nil
}

// SAMLConnectorInfo holds data about the created SSO connector and underlying
// Okta SAML app. Returned from CreateSSOConnector().
type SAMLConnectorInfo struct {
	Connector    types.SAMLConnector
	OktaAppID    string
	OktaAppLabel string
	OktaAppName  string
}

// CreateSAMLConnector automates the creation of an Okta SAML app and
// corresponding SAML SSO connector in Teleport.
func CreateSAMLConnector(ctx context.Context, args ConnectorArgs) (*SAMLConnectorInfo, error) {
	if err := args.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove the MFA resp from the context before getting the connector.
	// Otherwise, it will be consumed before the Create which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	getConnectorCtx := mfa.ContextWithMFAResponse(ctx, nil)
	_, err := args.SAMLConnectorService.GetSAMLConnector(getConnectorCtx, args.ConnectorName, false)
	if err == nil {
		return nil, trace.AlreadyExists("SAML SSO connector %q already exists", args.ConnectorName)
	}

	// First, we create the Okta side of the connection: an Okta app that will
	// allow members of the upstream Okta organization to log into Teleport via
	// SAML.
	app, err := createOktaSAMLApp(ctx, args.OktaClient, args.ClusterName, args.PublicURL, args.ConnectorName, args.Logger)
	if err != nil {
		return nil, trace.Wrap(err, "creating Okta SAML app")
	}

	// The "Everyone" group exists in all Okta instances, and contains all users
	// in the system. By assigning this group to our new Okta SAML app, we grant
	// all Okta users in the upstream Okta system the right to log into Teleport
	// using their Okta credentials.
	//
	//  See also: https://support.okta.com/help/s/article/The-Everyone-Group-in-Okta

	everyone, err := findOktaBuiltinGroup(ctx, args.OktaClient, api.OktaGroupEveryone)
	if trace.IsNotFound(err) {
		args.Logger.WarnContext(ctx, "Skipping assigning Everyone built-in group to Okta app because it was not found - probably not allowed by Okta resource sets",
			"app_name", app.Name, "app_id", app.Id)
	} else if err != nil {
		return nil, trace.Wrap(err, "finding Okta group Everyone")
	} else {
		args.Logger.InfoContext(ctx, "Assigning Everyone built-in group to Okta app",
			"app_name", app.Name, "app_id", app.Id, "group_id", everyone.Id)

		err := args.OktaClient.AssignGroupToApplication(ctx, api.OktaGroupID(everyone.Id), api.OktaAppID(app.Id))
		if err != nil {
			return nil, trace.Wrap(err, "assigning everyone to app")
		}
	}

	// Now that we have configured the Okta side of the SSO connection, we need
	// to create the corresponding Teleport side. This will allow the Teleport
	// cluster to offer SSO logins via Okta, and to receive profile information
	// from the Okta users via SAML.

	metadataURL, metadataContentType, err := extractMetadataURL(app.Links)
	if err != nil {
		return nil, trace.Wrap(err, "extracting SAML app entity metadata link")
	}

	args.Logger.InfoContext(ctx, "Downloading entity metadata for app",
		"app_id", app.Id,
		"metadata_url", metadataURL,
		"content_type", metadataContentType,
	)
	var acceptableContentTypes []string
	if metadataContentType != "" {
		acceptableContentTypes = append(acceptableContentTypes, metadataContentType)
	}
	metadata, err := args.OktaClient.DoHttp(ctx, http.MethodGet, metadataURL,
		acceptableContentTypes)
	if err != nil {
		return nil, trace.Wrap(err, "fetching SAML app entity metadata")
	}

	args.Logger.DebugContext(ctx, "Generating connector display name from Okta Organization")
	connectorDisplayName, err := generateConnectorName(ctx, args.OktaClient)
	if err != nil {
		return nil, trace.Wrap(err, "generating connector name")
	}

	// Note that we create the SSO connector with a role mapping gives all users
	// in the Okta Everyone group the "okta-requester" role so that they can at least log
	// into the Teleport cluster, but the only thing they can do is request
	// access from an admin.
	args.Logger.DebugContext(ctx, "Constructing SAML connector resource")
	connector, err := types.NewSAMLConnector(args.ConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: args.PublicURL.JoinPath("/v1/webapi/saml/acs", args.ConnectorName).String(),
		Display:                  connectorDisplayName,
		EntityDescriptor:         string(metadata),
		SigningKeyPair:           args.SigningKeypair,
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "groups",
				Value: api.OktaGroupEveryone,
				Roles: []string{teleport.SystemOktaRequesterRoleName},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	meta := connector.GetMetadata()
	meta.Labels = map[string]string{
		types.OriginLabel:         types.OriginOkta,
		eteleport.OktaOrgURLLabel: args.OktaClient.OrgURL(),
		eteleport.OktaAppIDLabel:  app.Id,
	}
	connector.SetMetadata(meta)

	args.Logger.InfoContext(ctx, "Creating new SAML connector", "connector_name", connector.GetName())
	if _, err := args.SAMLConnectorService.CreateSAMLConnector(ctx, connector); err != nil {
		return nil, trace.Wrap(err, "creating Okta SAML connector")
	}

	info := &SAMLConnectorInfo{
		Connector:    connector,
		OktaAppID:    app.Id,
		OktaAppName:  app.Name,
		OktaAppLabel: app.Label,
	}

	return info, nil
}

// ValidateSAMLConnector examines SAML Auth connector to see if it is configured
// for use with the Okta integration and extracts the appropriate metadata.
func ValidateSAMLConnector(ctx context.Context, connector types.SAMLConnector, oktaClient api.Client) (*SAMLConnectorInfo, error) {
	labels := connector.GetMetadata().Labels

	connectorAppID, present := labels[eteleport.OktaAppIDLabel]
	if !present {
		return nil, trace.BadParameter("missing Okta App ID")
	}

	connectorOrg := labels[eteleport.OktaOrgURLLabel]
	if connectorOrg != oktaClient.OrgURL() {
		return nil, trace.BadParameter("SAML connector bound to different Okta organization: %q", connectorOrg)
	}

	if connector.Origin() != types.OriginOkta {
		return nil, trace.BadParameter("invalid origin label: %q", connector.Origin())
	}

	app, err := oktaClient.GetApplication(ctx, api.OktaAppID(connectorAppID), &okta.SamlApplication{})
	if err != nil {
		return nil, trace.Wrap(err, "fetching Okta App ID %s", connectorAppID)
	}

	samlApp, ok := app.(*okta.SamlApplication)
	if !ok {
		return nil, trace.BadParameter("invalid Okta App type: %T", app)
	}

	info := &SAMLConnectorInfo{
		Connector:    connector,
		OktaAppID:    connectorAppID,
		OktaAppName:  samlApp.Name,
		OktaAppLabel: samlApp.Label,
	}

	return info, nil
}

// box creates a "boxed" (i.e. heap-allocated) copy of any value. Helpful when
// filling out optional REST API fields.
func box[T any](v T) *T {
	boxed := new(T)
	(*boxed) = v
	return boxed
}

// createOktaSAMLApp creates the Okta-side of the SAML SSO connector: a SAML app
// that will let used log in via Okta SSO.
func createOktaSAMLApp(ctx context.Context, oktaClient api.Client, clusterName string, publicURL *url.URL, connectorName string, logger *slog.Logger) (*okta.SamlApplication, error) {
	teleportEndpoint := publicURL.JoinPath("v1/webapi/saml/acs", connectorName).String()

	oktaAppRequest := &okta.SamlApplication{
		Label:         "Teleport " + clusterName,
		Accessibility: &okta.ApplicationAccessibility{},
		Visibility: &okta.ApplicationVisibility{
			Hide: &okta.ApplicationVisibilityHide{},
		},
		Features:   []string{},
		SignOnMode: "SAML_2_0",
		Credentials: &okta.ApplicationCredentials{
			UserNameTemplate: &okta.ApplicationCredentialsUsernameTemplate{
				Template: "${source.login}",
				Type:     "BUILT_IN",
			},
			Signing: &okta.ApplicationCredentialsSigning{},
		},
		Settings: &okta.SamlApplicationSettings{
			Notifications: &okta.ApplicationSettingsNotifications{
				Vpn: &okta.ApplicationSettingsNotificationsVpn{
					Network: &okta.ApplicationSettingsNotificationsVpnNetwork{
						Connection: "DISABLED",
					},
				},
			},
			SignOn: &okta.SamlApplicationSettingsSignOn{
				SsoAcsUrl:             teleportEndpoint,
				Destination:           teleportEndpoint,
				Recipient:             teleportEndpoint,
				Audience:              teleportEndpoint,
				IdpIssuer:             "http://www.okta.com/${org.externalKey}",
				SubjectNameIdFormat:   "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
				SubjectNameIdTemplate: "${user.userName}",
				SignatureAlgorithm:    "RSA_SHA256",
				DigestAlgorithm:       "SHA256",
				AuthnContextClassRef:  "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
				AssertionSigned:       box(true),
				ResponseSigned:        box(true),
				AttributeStatements: []*okta.SamlAttributeStatement{
					{
						Type:      "EXPRESSION",
						Name:      "username",
						Namespace: "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
						Values:    []string{"user.login"},
					}, {
						Type:        "GROUP",
						Name:        "groups",
						Namespace:   "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
						FilterType:  "REGEX",
						FilterValue: ".*",
					},
				},
			},
		},
	}

	logger.InfoContext(ctx, "Creating Okta App for Teleport SSO endpoint",
		"app", oktaAppRequest.Label,
		"endpoint", teleportEndpoint,
	)

	appResponse, err := oktaClient.CreateApplication(ctx, oktaAppRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actualApp, ok := appResponse.(*okta.SamlApplication)
	if !ok {
		return nil, trace.BadParameter("Okta API returned unexpected application type: %t", appResponse)
	}

	logger.InfoContext(ctx, "Created Okta App", "app_id", actualApp.Id, "app_label", actualApp.Label)
	return actualApp, nil
}

func generateConnectorName(ctx context.Context, oktaClient api.Client) (string, error) {
	orgName, err := oktaClient.OrgName(ctx)
	switch {
	case err == nil:
		return orgName, nil

	case trace.IsAccessDenied(err):
		// The user does not have the rights to fetch okta settings, so fall
		// back try to trying to generate a name based on the base endpoint url
		url, err := url.Parse(oktaClient.OrgURL())
		if err != nil {
			return "", trace.Wrap(err)
		}
		return strings.Split(url.Host, ".")[0], nil

	default:
		return "", trace.Wrap(err)
	}
}

func findOktaBuiltinGroup(ctx context.Context, oktaClient api.Client, name string) (*okta.Group, error) {
	result := (*okta.Group)(nil)
	err := oktaClient.IterateGroups(ctx, func(g *okta.Group) error {
		if g.Type == "BUILT_IN" && g.Profile.Name == name {
			result = g
			return trace.Wrap(api.ErrStopIteration)
		}
		return nil
	})

	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case result == nil:
		return nil, trace.NotFound(name)
	default:
		return result, nil
	}
}

func extractMetadataURL(input any) (*url.URL, string, error) {
	links := api.EmbeddedLinks{}
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
