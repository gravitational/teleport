package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/accessgraph/gitlab"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/integrations/access/servicenow"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/app"
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
	// install a plugin into itself. For integrations with complex
	// installation requirements, it can kick off the appropriate redirect
	// chain. For simpler plugins (e.g. with static auth credentials),
	// implementations can just do the work and return `OK`.
	HandleInstallRequest(context.Context, *web.SessionContext, http.ResponseWriter, *http.Request, *Plugin) (*ui.Plugin, error)

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

// HandleTestConfigRequest implements PluginDescriptor for pluginInstallerFn, always
// returning "Not Implemented".
func (fn pluginInstallerFn) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	return trace.NotImplemented("HandleTestConfigRequest")
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
	types.PluginTypeJira:              pluginInstallerFn(installJiraPlugin),
	types.PluginTypeOkta:              oktaPluginDescriptor{},
	types.PluginTypeOpsgenie:          pluginInstallerFn(installOpsgeniePlugin),
	types.PluginTypePagerDuty:         pluginInstallerFn(installPagerdutyPlugin),
	types.PluginTypeMattermost:        pluginInstallerFn(installMattermostPlugin),
	types.PluginTypeServiceNow:        pluginInstallerFn(installServiceNowPlugin),
	types.PluginTypeSlack:             slackDescriptor{},
	types.PluginTypeGitlab:            pluginInstallerFn(installGitlabPlugin),
	types.PluginTypeEntraID:           entraIDPluginDescriptor{},
	types.PluginTypeDatadog:           pluginInstallerFn(installDatadogPlugin),
	types.PluginTypeAWSIdentityCenter: awsICPluginDescriptor{},
	types.PluginTypeMSTeams:           pluginInstallerFn(installMSTeamsPlugin),
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
	for _, text := range strings.Split(channelsText, ",") {
		text = strings.TrimSpace(text)
		if text != "" {
			channels.ChannelIds = append(channels.ChannelIds, text)
		}
	}

	if len(channels.ChannelIds) == 0 {
		return nil, trace.BadParameter("missing or malformed channels: %q", channelsText)
	}

	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui, nil
}

func installOpsgeniePlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiEndpoint := r.FormValue("apiEndpoint")
	apiKey := r.FormValue("apiKey")
	scheduleName := r.FormValue("scheduleName")

	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
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

	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
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

	pluginReq := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, pluginReq, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
}

func installServiceNowPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	apiEndpoint := r.FormValue("apiEndpoint")
	username := r.FormValue("username")
	password := r.FormValue("password")
	closeCode := r.FormValue("closeCode")

	pluginReq := &pluginspb.CreatePluginRequest{
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
	}

	// Verify ServiceNow credential and API endpoint.
	_, err := servicenow.NewClient(servicenow.ClientConfig{
		APIEndpoint: apiEndpoint,
		Username:    username,
		APIToken:    password,
		CloseCode:   closeCode,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ui, err := installPlugin(ctx, sessCtx, pluginReq, p)
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

	pluginReq := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, pluginReq, p)
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

	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
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

	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
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
	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
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

	req := &pluginspb.CreatePluginRequest{
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
	}

	ui, err := installPlugin(ctx, sessCtx, req, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui, nil
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
		"client_id":    {meta.OauthClientId},
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
func (sd slackDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	// Set cookie info
	cookie := pluginOnboardingCookie{}
	cookie.Name = r.FormValue("name")
	cookie.Slack = &pluginOnboardingParamsSlack{
		FallbackChannel: r.FormValue("fallback_channel"),
	}
	cookie.EventID = r.FormValue("event_id")
	if err := setPluginOnboardingCookie(&cookie, w); err != nil {
		return nil, trace.Wrap(err)
	}

	url, err := sd.getAuthURL(ctx, sessCtx, r, types.PluginTypeSlack, cookie.State, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = app.MetaRedirect(w, url)
	if err != nil {
		p.Logger.WarnContext(ctx, "Failed to issue a redirect", "error", err)
		return nil, trace.Wrap(err)
	}
	return nil, nil
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
