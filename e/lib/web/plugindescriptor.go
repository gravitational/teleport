package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/accessgraph/github"
	"github.com/gravitational/teleport/e/lib/accessgraph/gitlab"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/integrations/access/opsgenie"
	"github.com/gravitational/teleport/integrations/access/servicenow"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

// pluginDescriptor describes various plugin operations that need individual
// handling for each plugin type (e.g Slack OAuth vs PagerDuty API key). This
// is an attempt to consolidate all of the different and inconsistent behaviors
// of the various plugins into one place - and in a reasonably extensible way -
// rather than scattered throughout the web interface in various giant
// switch statements.
//
// This first cut of the interface only abstracts away the bare minimum required
// for OIDC to work with Slack and fail for everything else; that's why this
// interface has `TranslateCallbackCookie()` instead of a more generic method
// `HandleCallbackRequest()`-type method.
//
// Given that interpretations of what constitutes a valid OAuth 2 Code Grant
// Flow can vary wildly from vendor to vendor, we probably *will* have to upgrade
// `TranslateCallbackCookie()` into a fully-fledged `HandleCallbackRequest()`
// when adding any subsequent OAuth-enabled plugins. Doing that now would
// be getting ahead of ourselves, as we don't yet know what will need to be
// factored out.
type pluginDescriptor interface {
	// HandleValidateConfigRequest handles a request to validate (possibly
	// partial) plugin configuration prior to actually enrolling the plugin
	// itself.
	HandleValidateConfigRequest(context.Context, *web.SessionContext, url.Values, *Plugin) error

	// HandleInstallRequest handles the http request that asks Teleport to
	// install a plugin into itself.
	// For plugins that don't require OAuth (eg: slack).
	HandleInstallRequest(context.Context, *web.SessionContext, http.ResponseWriter, *http.Request, *Plugin) (*ui.Plugin, error)

	// HandleOAuthStart handles the requirements required to begin an
	// OAuth2 grant flow (setting required cookie and returning the
	// OAuth URL to be redirected to) for OAuth plugins.
	HandleOAuthStart(context.Context, *web.SessionContext, http.ResponseWriter, *http.Request, *Plugin) (*ui.OAuthPluginStartResponse, error)

	// TranslateCallbackCookie translates cookie data set in an onboarding
	// workflow into plugin settings for a given plugin implementation.
	TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error
}

// pluginInstallerFn allows any standalone installer function with the right
// signature to be a plugin descriptor. Any attempt to handle an OAuth
// callback via `TranslateCallbackCookie` will return a not implemented
// error.
type pluginInstallerFn func(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error)

// HandleInstallRequest invokes the wrapped handler function
func (fn pluginInstallerFn) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	return fn(ctx, sessCtx, w, r, p)
}

// HandleValidateConfigRequest implements PluginDescriptor for pluginInstallerFn, always
// returning "Not Implemented".
func (fn pluginInstallerFn) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	return trace.NotImplemented("HandleTestConfigRequest")
}

// HandleOAuthStart implements PluginDescriptor for pluginInstallerFn, always
// returning "Not Implemented".
func (pluginInstallerFn) HandleOAuthStart(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.OAuthPluginStartResponse, error) {
	return nil, trace.NotImplemented("HandleOAuthStart")
}

// TranslateCallbackCookie implements PluginDescriptor for pluginInstallerFn, always
// returning "Not Implemented".
func (fn pluginInstallerFn) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	return trace.NotImplemented("TranslateCallbackCookie")
}

// defaultPluginDescriptors is the top-level mapping between plugin types and the
// descriptor that provides their customized behavior. For now, this map
// should be considered static, immutable data once the package is
// initialized, but there is nothing stopping us from wrapping it in mutexes
// and making it dynamic data in the future.
var defaultPluginDescriptors map[types.PluginType]pluginDescriptor = map[types.PluginType]pluginDescriptor{
	types.PluginTypeDiscord:           pluginInstallerFn(installDiscordPlugin),
	types.PluginTypeJamf:              pluginInstallerFn(installJamfPlugin),
	types.PluginTypeIntune:            pluginInstallerFn(installIntunePlugin),
	types.PluginTypeJira:              pluginInstallerFn(installJiraPlugin),
	types.PluginTypeOkta:              oktaPluginDescriptor{},
	types.PluginTypeOpsgenie:          pluginInstallerFn(installOpsgeniePlugin),
	types.PluginTypePagerDuty:         pluginInstallerFn(installPagerdutyPlugin),
	types.PluginTypeMattermost:        pluginInstallerFn(installMattermostPlugin),
	types.PluginTypeServiceNow:        pluginInstallerFn(installServiceNowPlugin),
	types.PluginTypeSlack:             slackDescriptor{},
	types.PluginTypeGitlab:            pluginInstallerFn(installGitlabPlugin),
	types.PluginTypeGithub:            pluginInstallerFn(installGithubPlugin),
	types.PluginTypeEntraID:           entraIDPluginDescriptor{},
	types.PluginTypeDatadog:           pluginInstallerFn(installDatadogPlugin),
	types.PluginTypeAWSIdentityCenter: awsICPluginDescriptor{},
	types.PluginTypeMSTeams:           pluginInstallerFn(installMSTeamsPlugin),
	types.PluginTypeEmail:             pluginInstallerFn(installEmailPlugin),
	types.PluginTypeNetIQ:             netIQPluginDescriptor{},
	types.PluginTypeSCIM:              pluginInstallerFn(installSCIMPlugin),
}

func installDiscordPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	token := r.FormValue("token")
	channelsText := r.FormValue("channels")

	if token == "" {
		return nil, trace.BadParameter("missing API token")
	}

	if channelsText == "" {
		return nil, trace.BadParameter("missing channels")
	}

	channels := &types.DiscordChannels{}
	for text := range strings.SplitSeq(channelsText, ",") {
		text = strings.TrimSpace(text)
		if text != "" {
			channels.ChannelIds = append(channels.ChannelIds, text)
		}
	}

	if len(channels.ChannelIds) == 0 {
		return nil, trace.BadParameter("missing or malformed channels: %q", channelsText)
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeDiscord,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Discord{
					Discord: &types.PluginDiscordSettings{
						RoleToRecipients: map[string]*types.DiscordChannels{
							types.Wildcard: channels,
						},
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"discord/key": "value",
					},
					Name: types.PluginTypeDiscord,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: token,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui, nil
}

func installOpsgeniePlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiEndpoint := r.FormValue("apiEndpoint")
	apiKey := r.FormValue("apiKey")
	scheduleName := r.FormValue("scheduleName")

	ogClient, err := opsgenie.NewClient(opsgenie.ClientConfig{
		APIKey:      apiKey,
		APIEndpoint: apiEndpoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ogClient.CheckHealth(ctx); err != nil {
		logger.Get(ctx).WarnContext(ctx, "Error performing Opsgenie health check",
			"error", err.Error())
		return nil, trace.Wrap(err)
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeOpsgenie,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Opsgenie{
					Opsgenie: &types.PluginOpsgenieAccessSettings{
						DefaultSchedules: []string{scheduleName},
						ApiEndpoint:      apiEndpoint,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"opsgenie/api-endpoint": apiEndpoint,
					},
					Name: types.PluginTypeOpsgenie,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: apiKey,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installGithubPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	const (
		// multipartFormBufSize is a buffer size for ParseMultipartForm
		multipartFormBufSize = 8192
	)

	if err := r.ParseMultipartForm(multipartFormBufSize); err != nil {
		return nil, trace.Wrap(err)
	}

	var prvKey bytes.Buffer
	file, _, err := r.FormFile("privateKey")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()
	if _, err := io.Copy(&prvKey, file); err != nil {
		return nil, trace.Wrap(err)
	}
	if prvKey.Len() == 0 {
		return nil, trace.BadParameter("empty private key")
	}

	organizationName := r.FormValue("organizationName")
	if organizationName == "" {
		return nil, trace.BadParameter("missing Organization name")
	}

	clientID := r.FormValue("clientID")
	if clientID == "" {
		return nil, trace.BadParameter("missing Client ID")
	}

	startDate := r.FormValue("startDate")
	if startDate == "" {
		return nil, trace.BadParameter("missing Start date")
	}
	t, err := time.ParseInLocation(time.RFC3339, startDate, time.UTC)
	if err != nil {
		return nil, trace.BadParameter("invalid Start date format: %v", err)
	}

	if err := github.GithubInstanceConnectionTest(ctx, github.GithubConfig{
		ClientID:           clientID,
		PrivateKey:         prvKey.Bytes(),
		Organization:       organizationName,
		BootstrapStartDate: t,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccessGraph,
			Metadata: types.Metadata{
				Labels: map[string]string{
					types.TeleportNamespace + "/hosted-plugin": "true",
				},
				Name: organizationName,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Github{
					Github: &types.PluginGithubSettings{
						ApiEndpoint:      "", /* TODO: set the API endpoint */
						ClientId:         clientID,
						OrganizationName: organizationName,
						StartDate:        t,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name:   types.PluginTypeGithub + "-" + organizationName + "-private-key",
					Labels: map[string]string{},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_PrivateKey{
					PrivateKey: prvKey.Bytes(),
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui, nil
}

func installGitlabPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiKey := r.FormValue("apiKey")
	if apiKey == "" {
		return nil, trace.BadParameter("missing API key")
	}
	apiEndpoint := r.FormValue("apiEndpoint")
	if apiEndpoint == "" {
		return nil, trace.BadParameter("missing API endpoint")
	}

	if err := gitlab.GitlabInstanceConnectionTest(ctx, apiEndpoint, apiKey); err != nil {
		return nil, trace.Wrap(err)
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccessGraph,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeGitlab,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Gitlab{
					Gitlab: &types.PluginGitlabSettings{
						ApiEndpoint: apiEndpoint,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeGitlab,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: apiKey,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui, nil
}

func installJamfPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiEndpoint := r.FormValue("apiEndpoint")
	if apiEndpoint == "" {
		return nil, trace.BadParameter("jamf API endpoint required")
	}

	clientId := r.FormValue("clientId")
	clientSecret := r.FormValue("clientSecret")
	if clientId == "" || clientSecret == "" {
		return nil, trace.BadParameter("jamf API credentials required")
	}

	pluginReq := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindMDM,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeJamf,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Jamf{
					Jamf: &types.PluginJamfSettings{
						JamfSpec: &types.JamfSpecV1{
							ApiEndpoint: apiEndpoint,
						},
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"jamf/api-endpoint": apiEndpoint,
					},
					Name: types.PluginTypeJamf,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
					OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
						ClientId:     clientId,
						ClientSecret: clientSecret,
					},
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, pluginReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installIntunePlugin(ctx context.Context, sessCtx *web.SessionContext, _ http.ResponseWriter, r *http.Request, _ *Plugin) (*ui.Plugin, error) {
	tenant := r.FormValue("tenant")
	if tenant == "" {
		return nil, trace.BadParameter("tenant required")
	}

	clientID := r.FormValue("clientId")
	clientSecret := r.FormValue("clientSecret")
	if clientID == "" || clientSecret == "" {
		return nil, trace.BadParameter("API credentials required")
	}

	pluginReq := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindMDM,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeIntune,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Intune{
					Intune: &types.PluginIntuneSettings{
						Tenant: tenant,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"intune/tenant": tenant,
					},
					Name: types.PluginTypeIntune,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
					OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
						ClientId:     clientID,
						ClientSecret: clientSecret,
					},
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, pluginReq)
	return ui, trace.Wrap(err)
}

func installServiceNowPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiEndpoint := r.FormValue("apiEndpoint")
	username := r.FormValue("username")
	password := r.FormValue("password")
	closeCode := r.FormValue("closeCode")

	snClient, err := servicenow.NewClient(servicenow.ClientConfig{
		APIEndpoint: apiEndpoint,
		Username:    username,
		APIToken:    password,
		CloseCode:   closeCode,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := snClient.CheckHealth(ctx); err != nil {
		logger.Get(ctx).WarnContext(ctx, "Error performing ServiceNow health check",
			"error", err.Error())
		return nil, trace.Wrap(err)
	}

	pluginReq := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeServiceNow,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_ServiceNow{
					ServiceNow: &types.PluginServiceNowSettings{
						ApiEndpoint: apiEndpoint,
						CloseCode:   closeCode,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"servicenow/api-endpoint": apiEndpoint,
					},
					Name: types.PluginTypeServiceNow,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
					BasicAuth: &types.PluginStaticCredentialsBasicAuth{
						Username: username,
						Password: password,
					},
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, pluginReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installJiraPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	addr := r.FormValue("addr")
	if addr == "" {
		return nil, trace.BadParameter("missing Jira server url")
	}

	jiraURL, err := lib.AddrToURL(addr)
	if err != nil {
		return nil, trace.Wrap(err, "malformed Jira server url")
	}

	username := r.FormValue("username")
	if username == "" {
		return nil, trace.BadParameter("missing Jira user name")
	}

	apiKey := r.FormValue("apiKey")
	if apiKey == "" {
		return nil, trace.BadParameter("missing Jira API key")
	}

	projectID := r.FormValue("project")
	if projectID == "" {
		return nil, trace.BadParameter("missing Jira project key")
	}

	issueType := r.FormValue("issueType")
	if issueType == "" {
		return nil, trace.BadParameter("missing Jira issue type")
	}

	pluginReq := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeJira,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Jira{
					Jira: &types.PluginJiraSettings{
						ServerUrl:  jiraURL.String(),
						ProjectKey: projectID,
						IssueType:  issueType,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"jira/address":   jiraURL.String(),
						"jira/project":   projectID,
						"jira/issueType": issueType,
					},
					Name: types.PluginTypeJira,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
					// JIRA does issue API keys, but they are for all intents and
					// purposes an alternative password for a given user. Even with
					// an API key you still require a username to log in, so we may
					// as well just treat it like a basic auth scenario
					BasicAuth: &types.PluginStaticCredentialsBasicAuth{
						Username: username,
						Password: apiKey,
					},
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, pluginReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

const pagerDutyEndPoint = "https://api.pagerduty.com"

func installPagerdutyPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	email := r.FormValue("email")
	if len(email) == 0 {
		return nil, trace.BadParameter("missing PagerDuty User Email")
	}

	apiKey := r.FormValue("apiKey")
	if len(apiKey) == 0 {
		return nil, trace.BadParameter("missing PagerDuty API key")
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypePagerDuty,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_PagerDuty{
					PagerDuty: &types.PluginPagerDutySettings{
						ApiEndpoint: pagerDutyEndPoint,
						UserEmail:   email,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"pagerduty/email":        email,
						"pagerduty/api_endpoint": pagerDutyEndPoint,
					},
					Name: types.PluginTypePagerDuty,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: apiKey,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installMattermostPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	url := r.FormValue("url")
	if len(url) == 0 {
		return nil, trace.BadParameter("missing Mattermost server URL")
	}

	token := r.FormValue("token")
	if len(token) == 0 {
		return nil, trace.BadParameter("missing Mattermost bot access token")
	}

	// Optional fields.
	email := r.FormValue("email")
	channel := r.FormValue("channel")
	team := r.FormValue("team")

	// If one field is defined, both should be required.
	if len(channel) > 0 || len(team) > 0 {
		if len(channel) == 0 {
			return nil, trace.BadParameter("missing Mattermost channel name")
		}
		if len(team) == 0 {
			return nil, trace.BadParameter("missing Mattermost team name")
		}
	}

	labels := map[string]string{
		"mattermost/channel":    channel,
		"mattermost/team":       team,
		"mattermost/server-url": url,
	}
	if len(email) != 0 {
		labels["mattermost/email"] = email
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeMattermost,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Mattermost{
					Mattermost: &types.PluginMattermostSettings{
						ServerUrl:     url,
						Channel:       channel,
						Team:          team,
						ReportToEmail: email,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: labels,
					Name:   types.PluginTypeMattermost,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: token,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installDatadogPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiEndpoint := r.FormValue("apiEndpoint")
	if len(apiEndpoint) == 0 {
		return nil, trace.BadParameter("missing API Endpoint")
	}
	fallbackRecipient := r.FormValue("fallbackRecipient")
	if len(fallbackRecipient) == 0 {
		return nil, trace.BadParameter("missing Fallback Recipient")
	}
	apiKey := r.FormValue("apiKey")
	if len(apiKey) == 0 {
		return nil, trace.BadParameter("missing Datadog API key")
	}
	applicationKey := r.FormValue("applicationKey")
	if len(applicationKey) == 0 {
		return nil, trace.BadParameter("missing Datadog Application key")
	}
	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeDatadog,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Datadog{
					Datadog: &types.PluginDatadogAccessSettings{
						ApiEndpoint:       apiEndpoint,
						FallbackRecipient: fallbackRecipient,
					},
				},
			},
		},
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: types.DatadogCredentialAPIKey,
						Labels: map[string]string{
							types.DatadogCredentialLabel: types.DatadogCredentialAPIKey,
						},
					},
				},
				Spec: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: apiKey,
					},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: types.DatadogCredentialApplicationKey,
						Labels: map[string]string{
							types.DatadogCredentialLabel: types.DatadogCredentialApplicationKey,
						},
					},
				},
				Spec: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: applicationKey,
					},
				},
			},
		},
		CredentialLabels: map[string]string{
			"datadog/api_endpoint": apiEndpoint,
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installMSTeamsPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	appSecret := r.FormValue("appSecret")
	if len(appSecret) == 0 {
		return nil, trace.BadParameter("missing MsTeams app secret")
	}
	appID := r.FormValue("appID")
	if len(appID) == 0 {
		return nil, trace.BadParameter("missing MsTeams app ID")
	}
	tenantID := r.FormValue("tenantID")
	if len(tenantID) == 0 {
		return nil, trace.BadParameter("missing MsTeams tenant ID")
	}
	defaultRecipient := r.FormValue("defaultRecipient")
	if len(defaultRecipient) == 0 {
		return nil, trace.BadParameter("missing MsTeams default recipient")
	}
	teamsAppID := r.FormValue("teamsAppID")
	if teamsAppID == "" {
		teamsAppID = uuid.NewString()
	}
	region := r.FormValue("region")

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			Kind:    types.PluginTypeMSTeams,
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeMSTeams,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Msteams{
					Msteams: &types.PluginMSTeamsSettings{
						AppId:            appID,
						TenantId:         tenantID,
						TeamsAppId:       teamsAppID,
						Region:           region,
						DefaultRecipient: defaultRecipient,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{},
					Name:   types.PluginTypeMSTeams,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: appSecret,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installEmailPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	switch r.FormValue("service") {
	case "mailgun":
		return installMailgunPlugin(ctx, sessCtx, w, r, p)
	case "smtp":
		return installSMTPPlugin(ctx, sessCtx, w, r, p)
	default:
		return nil, trace.BadParameter("unknown email service type: %q", r.FormValue("service"))
	}
}

func installMailgunPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	sender := r.FormValue("sender")
	if sender == "" {
		return nil, trace.BadParameter("missing Sender")
	}
	fallbackRecipient := r.FormValue("fallbackRecipient")
	if fallbackRecipient == "" {
		return nil, trace.BadParameter("missing Fallback Recipient")
	}
	domain := r.FormValue("domain")
	if domain == "" {
		return nil, trace.BadParameter("missing Mailgun Domain")
	}
	privateKey := r.FormValue("privateKey")
	if privateKey == "" {
		return nil, trace.BadParameter("missing Mailgun Private Key")
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeEmail,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Email{
					Email: &types.PluginEmailSettings{
						Sender:            sender,
						FallbackRecipient: fallbackRecipient,
						Spec: &types.PluginEmailSettings_MailgunSpec{
							MailgunSpec: &types.MailgunSpec{
								Domain: domain,
							},
						},
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"mailgun/domain": domain,
						"mailgun/sender": sender,
					},
					Name: types.PluginTypeEmail,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: privateKey,
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	return ui, trace.Wrap(err)
}

func installSMTPPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	sender := r.FormValue("sender")
	if sender == "" {
		return nil, trace.BadParameter("missing Sender")
	}
	fallbackRecipient := r.FormValue("fallbackRecipient")
	if fallbackRecipient == "" {
		return nil, trace.BadParameter("missing Fallback Recipient")
	}
	host := r.FormValue("host")
	if host == "" {
		return nil, trace.BadParameter("missing SMTP Host")
	}
	port, err := strconv.Atoi(r.FormValue("port"))
	if err != nil {
		return nil, trace.BadParameter("SMTP port must be a valid port value")
	}
	startTLSPolicy := r.FormValue("startTLSPolicy")
	if startTLSPolicy == "" {
		return nil, trace.BadParameter("missing Start TLS Policy")
	}
	username := r.FormValue("username")
	if username == "" {
		return nil, trace.BadParameter("missing SMTP Username")
	}
	password := r.FormValue("password")
	if password == "" {
		return nil, trace.BadParameter("missing SMTP Password")
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeEmail,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Email{
					Email: &types.PluginEmailSettings{
						Sender:            sender,
						FallbackRecipient: fallbackRecipient,
						Spec: &types.PluginEmailSettings_SmtpSpec{
							SmtpSpec: &types.SMTPSpec{
								Host:           host,
								Port:           int32(port),
								StartTlsPolicy: startTLSPolicy,
							},
						},
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"smtp/host":   host,
						"smtp/port":   strconv.Itoa(port),
						"smtp/sender": sender,
					},
					Name: types.PluginTypeEmail,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
					BasicAuth: &types.PluginStaticCredentialsBasicAuth{
						Username: username,
						Password: password,
					},
				},
			},
		},
	}.Build()

	ui, err := installPlugin(ctx, sessCtx, req)
	return ui, trace.Wrap(err)
}

// slackDescriptor defines the custom behavior of the Slack plugin. Contains
// no data, exists only as a thing to hang a custom PluginDescriptor
// implementation on.
type slackDescriptor struct{}

// Static assertion that slackDescriptor implements pluginDescriptor
var _ pluginDescriptor = slackDescriptor{}

func (slackDescriptor) getAuthURL(ctx context.Context, sctx *web.SessionContext, r *http.Request, typ string, state string, p *Plugin) (string, error) {
	meta, err := p.getPluginTypeMeta(ctx, sctx, typ)
	if err != nil {
		return "", trace.Wrap(err)
	}

	callbackURL := p.getPluginCallbackURL(r, typ)
	scopes := []string{
		"chat:write",
		"users:read",
		"users:read.email",
	}

	uri, err := url.Parse(slackAuthBaseURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	uri.RawQuery = url.Values{
		"scope":        {strings.Join(scopes, ",")},
		"client_id":    {meta.GetOauthClientId()},
		"redirect_uri": {callbackURL},
		"state":        {state},
	}.Encode()

	return uri.String(), nil
}

// HandleValidateConfigRequest implements pluginDescriptor for the
// slackDescriptor type. Always returns NotImplemented.
func (slackDescriptor) HandleValidateConfigRequest(context.Context, *web.SessionContext, url.Values, *Plugin) error {
	return trace.NotImplemented("HandleValidateConfigRequest")
}

// HandleInstallRequest kicks off a OAuth2 Code Grant Flow for authorizing
// access to a slack App.
//
// Deprecated: use HandleOAuthStart instead.
func (sd slackDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	return nil, trace.NotImplemented("HandleInstallRequest")
}

// HandleOAuthStart sets required cookie and returns a redirect URL that will start a
// OAuth2 code grant flow for authorizing access to a slack App.
func (sd slackDescriptor) HandleOAuthStart(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.OAuthPluginStartResponse, error) {
	url, err := sd.setCookieAndCreateAuthnURL(ctx, sessCtx, w, r, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.OAuthPluginStartResponse{RedirectURL: url}, nil
}

// TranslateCallbackCookie translates data from the onboarding cookie,
// validates that it is the expected data for a slack installation, and
// and injects the validated values into the plugin spec.
func (slackDescriptor) TranslateCallbackCookie(pluginSpec *types.PluginSpecV1, cookie *pluginOnboardingCookie) error {
	if cookie.Slack == nil {
		return trace.BadParameter("slack info missing")
	}
	pluginSpec.Settings = &types.PluginSpecV1_SlackAccessPlugin{
		SlackAccessPlugin: &types.PluginSlackAccessSettings{
			FallbackChannel: cookie.Slack.FallbackChannel,
		},
	}
	return nil
}

// setCookieAndCreateAuthnURL sets a cookie with insensitive plugin information
// and a state field. Returns the auth providers URL (that starts the OAuth grant flow)
// with the same state field set as a URL query param.
// The state parameter will be used later by TranslateCallbackCookie to validate the request.
func (sd slackDescriptor) setCookieAndCreateAuthnURL(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (string, error) {
	// Set cookie info
	cookie := pluginOnboardingCookie{}
	cookie.Name = r.FormValue("name")
	cookie.Slack = &pluginOnboardingParamsSlack{
		FallbackChannel: r.FormValue("fallback_channel"),
	}
	cookie.EventID = r.FormValue("event_id")
	if err := setPluginOnboardingCookie(&cookie, w); err != nil {
		return "", trace.Wrap(err)
	}

	url, err := sd.getAuthURL(ctx, sessCtx, r, types.PluginTypeSlack, cookie.State, p)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return url, nil
}

func installSCIMPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	// genericSCIMPluginName is the internal name used for the generic SCIM plugin.
	// This name determines the SCIM endpoint path, which will be exposed as /scim/scim-generic.
	// We use a specific name here to avoid confusion with paths like /scim/scim,
	// and to make the plugin name more meaningful when listed via tctl or other tools.
	const genericSCIMPluginName = "scim-generic"

	var scimSettings *types.PluginSCIMSettings

	connectorName := r.FormValue("connectorName")
	if connectorName != "" {
		connectorKind := r.FormValue("connectorKind")
		scimSettings = &types.PluginSCIMSettings{
			ConnectorInfo: &types.PluginSCIMSettings_ConnectorInfo{
				Name: connectorName,
				Type: connectorKind,
			},
		}
	} else {
		// TODO(smallinsky) Remove in v19.
		scimSettings = &types.PluginSCIMSettings{
			SamlConnectorName: r.FormValue("samlConnectorName"),
		}
	}

	clientID, clientSecret, err := generateOAuthCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: genericSCIMPluginName,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Scim{
					Scim: scimSettings,
				},
			},
		},
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			buildOauthCreds(clientID, clientSecret),
		},
	}.Build()
	uiResp, err := installPlugin(ctx, sessCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiResp.Credentials = &ui.Credentials{
		OAuthCreds: &ui.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		},
	}
	return uiResp, nil
}

func buildOauthCreds(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: fmt.Sprintf("%s-%s", types.PluginTypeSCIM, uuid.NewString()),
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     clientID,
				ClientSecret: clientSecret,
			},
		}},
	}
}

func generateOAuthCredentials() (string, string, error) {
	clientID, err := utils.CryptoRandomHex(16)
	if err != nil {
		return "", "", err
	}
	clientSecret, err := utils.CryptoRandomHex(32)
	if err != nil {
		return "", "", err
	}
	return clientID, clientSecret, nil
}
