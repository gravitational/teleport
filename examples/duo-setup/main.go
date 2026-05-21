// duo-setup is a proof-of-concept tool that configures a Teleport cluster
// to use Duo as a SAML SSO identity provider and provisions a SCIM plugin
// so Duo can sync users/groups into Teleport.
//
// It connects to the cluster using the active tsh profile (or an identity
// file with --identity) and:
//
//  1. Upserts a SAML connector named "duo" that maps any SAML "groups"
//     attribute value to the Teleport "requester" role.
//  2. Creates a SCIM plugin with OAuth credentials pointing at that
//     connector.
//
// On success it prints everything that needs to be configured on the Duo
// side: the SP ACS URL and entity ID for the SAML app, plus the SCIM base
// URL, OAuth token URL, client ID and client secret.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/gravitational/teleport/api/client"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	connectorName = "duo"
	pluginName    = types.PluginTypeSCIM
	editorRole    = "editor"
)

type config struct {
	identityFile string
	proxyAddr    string

	entityDescriptorURL string
	issuer              string
	audience            string
	acsURL              string
	display             string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.identityFile, "identity", "", "Path to a Teleport identity file. If empty, the active tsh profile is used.")
	flag.StringVar(&cfg.proxyAddr, "proxy", "", "Teleport proxy address. Required when --identity is set.")

	flag.StringVar(&cfg.entityDescriptorURL, "entity-descriptor-url", "", "Duo SAML IdP metadata URL (required).")
	flag.StringVar(&cfg.issuer, "issuer", "", "SAML SP issuer / entity ID Teleport will present to Duo (required).")
	flag.StringVar(&cfg.audience, "audience", "", "SAML SP audience Teleport will accept from Duo (required).")
	flag.StringVar(&cfg.acsURL, "acs-url", "", "SAML Assertion Consumer Service URL. Defaults to https://<proxy>/v1/webapi/saml/acs/duo.")
	flag.StringVar(&cfg.display, "display", "Duo", "Display name shown on the Teleport login screen.")

	flag.Parse()

	if err := run(context.Background(), cfg); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(ctx context.Context, cfg config) error {
	if err := validate(&cfg); err != nil {
		return err
	}

	clt, err := newClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to Teleport: %w", err)
	}
	defer clt.Close()

	ping, err := clt.Ping(ctx)
	if err != nil {
		return fmt.Errorf("pinging cluster: %w", err)
	}
	proxyPublicAddr := ping.GetProxyPublicAddr()
	if cfg.acsURL == "" {
		cfg.acsURL = fmt.Sprintf("https://%s/v1/webapi/saml/acs/%s", proxyPublicAddr, connectorName)
	}

	if err := upsertSAMLConnector(ctx, clt, cfg); err != nil {
		return fmt.Errorf("creating SAML connector: %w", err)
	}

	creds := generateOAuthCredentials()
	if err := createSCIMPlugin(ctx, clt, creds); err != nil {
		return fmt.Errorf("creating SCIM plugin: %w", err)
	}

	printDuoInstructions(cfg, proxyPublicAddr, creds)
	return nil
}

func validate(cfg *config) error {
	missing := []string{}
	if cfg.entityDescriptorURL == "" {
		missing = append(missing, "--entity-descriptor-url")
	}
	if cfg.issuer == "" {
		missing = append(missing, "--issuer")
	}
	if cfg.audience == "" {
		missing = append(missing, "--audience")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flag(s): %s", strings.Join(missing, ", "))
	}
	if cfg.identityFile != "" && cfg.proxyAddr == "" {
		return fmt.Errorf("--proxy is required when --identity is set")
	}
	return nil
}

func newClient(ctx context.Context, cfg config) (*client.Client, error) {
	clientCfg := client.Config{}
	if cfg.identityFile != "" {
		clientCfg.Credentials = []client.Credentials{client.LoadIdentityFile(cfg.identityFile)}
		clientCfg.Addrs = []string{cfg.proxyAddr}
	} else {
		clientCfg.Credentials = []client.Credentials{client.LoadProfile("", "")}
	}
	return client.New(ctx, clientCfg)
}

func upsertSAMLConnector(ctx context.Context, clt *client.Client, cfg config) error {
	connector, err := types.NewSAMLConnector(connectorName, types.SAMLConnectorSpecV2{
		Display:                  cfg.display,
		Issuer:                   cfg.issuer,
		Audience:                 cfg.audience,
		ServiceProviderIssuer:    cfg.issuer,
		AssertionConsumerService: cfg.acsURL,
		EntityDescriptorURL:      cfg.entityDescriptorURL,
		AttributesToRoles: []types.AttributeMapping{{
			Name:  "groups",
			Value: "*",
			Roles: []string{editorRole},
		}},
	})
	if err != nil {
		return err
	}

	if _, err := clt.UpsertSAMLConnector(ctx, connector); err != nil {
		return err
	}
	return nil
}

func createSCIMPlugin(ctx context.Context, clt *client.Client, creds *oauthCreds) error {
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Name:   pluginName,
			Labels: map[string]string{"teleport.dev/hosted-plugin": "true"},
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{
				Scim: &types.PluginSCIMSettings{
					ConnectorInfo: &types.PluginSCIMSettings_ConnectorInfo{
						Name: connectorName,
						Type: types.KindSAML,
					},
				},
			},
		},
	}

	_, err := clt.PluginsClient().CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
		Plugin:                plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{creds.stored},
	})
	return err
}

type oauthCreds struct {
	stored       *types.PluginStaticCredentialsV1
	clientID     string
	clientSecret string
}

func generateOAuthCredentials() *oauthCreds {
	clientID := uuid.NewString()
	clientSecret := uuid.NewString() + uuid.NewString()
	return &oauthCreds{
		clientID:     clientID,
		clientSecret: clientSecret,
		stored: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{Metadata: types.Metadata{
				Name: fmt.Sprintf("%s-%s", pluginName, uuid.NewString()),
			}},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
					OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
						ClientId:     clientID,
						ClientSecret: clientSecret,
					},
				},
			},
		},
	}
}

func printDuoInstructions(cfg config, proxyPublicAddr string, creds *oauthCreds) {
	w := os.Stdout
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Teleport is now configured. Configure Duo with the following:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -- SAML application --")
	fmt.Fprintf(w, "  Assertion Consumer Service URL: %s\n", cfg.acsURL)
	fmt.Fprintf(w, "  Service Provider Entity ID:     %s\n", cfg.issuer)
	fmt.Fprintf(w, "  Audience:                       %s\n", cfg.audience)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -- SCIM provisioning --")
	fmt.Fprintf(w, "  Base URL:            https://%s/v1/webapi/scim/%s\n", proxyPublicAddr, pluginName)
	fmt.Fprintf(w, "  OAuth token URL:     https://%s/v1/webapi/plugin/%s/token\n", proxyPublicAddr, pluginName)
	fmt.Fprintf(w, "  OAuth client ID:     %s\n", creds.clientID)
	fmt.Fprintf(w, "  OAuth client secret: %s\n", creds.clientSecret)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "These secrets are shown once — record them now.")
}
