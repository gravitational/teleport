package okta

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
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
	OktaClient oktaClient

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
	Log logrus.FieldLogger
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
	if a.Log == nil {
		return trace.BadParameter("missing SSO connector parameter Log")
	}
	return nil
}

// CreateSSOConnector automates the creation of an Okta SAML app and
// corresponding SAML SSO connector in Teleport.
func CreateSSOConnector(ctx context.Context, args ConnectorArgs) error {
	if err := args.Check(); err != nil {
		return trace.Wrap(err)
	}

	_, err := args.SAMLConnectorService.GetSAMLConnector(ctx, args.ConnectorName, false)
	if err == nil {
		return trace.AlreadyExists("SAML SSO connector %q already exists", args.ConnectorName)
	}

	// First, we create the Okta side of the connection: an Okta app that will
	// allow members of the upstream Okta organization to log into Teleport via
	// SAML.

	app, err := createOktaSAMLApp(ctx, args.OktaClient, args.ClusterName, args.PublicURL, args.ConnectorName, args.Log)
	if err != nil {
		return trace.Wrap(err, "creating Okta SAML app")
	}

	// The "Everyone" group exists in all Okta instances, and contains all users
	// in the system. By assigning this group to our new Okta SAML app, we grant
	// all Okta users in the upstream Okta system the right to log into Teleport
	// using their Okta credentials.
	//
	//  See also: https://support.okta.com/help/s/article/The-Everyone-Group-in-Okta

	everyone, err := findOktaBuiltinGroup(ctx, args.OktaClient, oktaGroupEveryone)
	if err != nil {
		return trace.Wrap(err, "finding Okta group Everyone")
	}

	args.Log.Infof("Assigning everyone (group %s) to app %s (%s)", everyone.Id, app.Name, app.Id)
	err = args.OktaClient.assignGroupToApplicationByID(ctx, everyone.Id, app.Id)
	if err != nil {
		return trace.Wrap(err, "assigning everyone to app")
	}

	// Now that we have configured the Okta side of the SSO connection, we need
	// to create the corresponding Teleport side. This will allow the Teleport
	// cluster to offer SSO logins via Okta, and to receive profile information
	// from the Okta users via SAML.

	metadataURL, metadataContentType, err := extractMetadataURL(app.Links)
	if err != nil {
		return trace.Wrap(err, "extracting SAML app entity metadata link")
	}

	args.Log.Infof("Downloading entity metadata for app %s from %s as %q",
		app.Id, metadataURL, metadataContentType)
	var acceptableContentTypes []string
	if metadataContentType != "" {
		acceptableContentTypes = append(acceptableContentTypes, metadataContentType)
	}
	metadata, err := args.OktaClient.doHttp(ctx, http.MethodGet, metadataURL,
		acceptableContentTypes)
	if err != nil {
		return trace.Wrap(err, "fetching SAML app entity metadata")
	}

	connectorDisplayName, err := generateConnectorName(ctx, args.OktaClient)
	if err != nil {
		return trace.Wrap(err, "generating connector name")
	}

	// Note that we create the SSO connector with a role mapping gives all users
	// in the Okta Everyone group the "requester" role so that they can at least log
	// into the Teleport cluster, but the only thing they can do is request
	// access from an admin.

	connector, err := types.NewSAMLConnector(args.ConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: args.PublicURL.JoinPath("/v1/webapi/saml/acs", args.ConnectorName).String(),
		Display:                  connectorDisplayName,
		EntityDescriptor:         string(metadata),
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "groups",
				Value: oktaGroupEveryone,
				Roles: []string{teleport.PresetRequesterRoleName},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	args.Log.Infof("Creating new SAML SSO connector %s for Okta org %s",
		connector.GetName(),
		args.PublicURL)
	if _, err := args.SAMLConnectorService.CreateSAMLConnector(ctx, connector); err != nil {
		return trace.Wrap(err, "creating Okta SAML connector")
	}

	return nil
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
func createOktaSAMLApp(ctx context.Context, oktaClient oktaClient, clusterName string, publicURL *url.URL, connectorName string, log logrus.FieldLogger) (*okta.SamlApplication, error) {
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

	log.Infof("Creating Okta App %q for Teleport SSO endpoint %s",
		oktaAppRequest.Label, teleportEndpoint)

	appResponse, err := oktaClient.createApplication(ctx, oktaAppRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actualApp, ok := appResponse.(*okta.SamlApplication)
	if !ok {
		return nil, trace.BadParameter("Okta API returned unexpected application type: %t", appResponse)
	}

	log.Infof("Created Okta App %s: %q", actualApp.Id, actualApp.Label)
	return actualApp, nil
}

func generateConnectorName(ctx context.Context, oktaClient oktaClient) (string, error) {
	orgName, err := oktaClient.orgName(ctx)
	switch {
	case err == nil:
		return orgName, nil

	case trace.IsAccessDenied(err):
		// The user does not have the rights to fetch okta settings, so fall
		// back try to trying to generate a name based on the base endpoint url
		url, err := url.Parse(oktaClient.orgURL())
		if err != nil {
			return "", trace.Wrap(err)
		}
		return strings.Split(url.Host, ".")[0], nil

	default:
		return "", trace.Wrap(err)
	}
}

func findOktaBuiltinGroup(ctx context.Context, oktaClient oktaClient, name string) (*okta.Group, error) {
	result := (*okta.Group)(nil)
	err := oktaClient.iterateGroups(ctx, func(g *okta.Group) error {
		if g.Type == "BUILT_IN" && g.Profile.Name == name {
			result = g
			return stopIteration
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
	links := embeddedLinks{}
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
