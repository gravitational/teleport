package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
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
	// HandleInstallRequest handles the http request that asks Teleport to
	// install a plugin into itself. For integrations with complex
	// installation requirements, it can kick off the appropriate redirect
	// chain. For simpler pluigins (e.g. with static auth credentials),
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

// TranslateCallbackCookie implements PluginDescriptor for pluginInstallerFn, always
// returning "Not Implemented".
func (fn pluginInstallerFn) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	return trace.NotImplemented("TranslateCallbackCookie")
}

// pluginDescriptors is the top-level mapping between plugin types and the
// descriptor that provides their customized behavior. For now, this map
// should be considered static, immutable data once the package is
// initialized, but there is nothing stopping us from wrapping it in mutexes
// and making it dynamic data in the future.
var pluginDescriptors map[types.PluginType]pluginDescriptor = map[types.PluginType]pluginDescriptor{
	types.PluginTypeJamf:      pluginInstallerFn(installJamfPlugin),
	types.PluginTypeOkta:      pluginInstallerFn(installOktaPlugin),
	types.PluginTypeOpsgenie:  pluginInstallerFn(installOpsgeniePlugin),
	types.PluginTypePagerDuty: pluginInstallerFn(installPagerdutyPlugin),
	types.PluginTypeSlack:     slackDescriptor{},
}

func installOktaPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	orgURL := r.FormValue("orgURL")
	apiToken := r.FormValue("apiToken")

	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
				Name: types.PluginTypeOkta,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						OrgUrl: orgURL,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"okta/org-url": orgURL,
					},
					Name: types.PluginTypeOkta,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: apiToken,
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

func installJamfPlugin(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
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
							ApiEndpoint: r.FormValue("apiEndpoint"),
						},
					},
				},
			},
		},
	}

	_, err := ui.NewPlugin(pluginReq.Plugin)
	if err != nil {
		return nil, trace.Wrap(err)
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

// slackDescriptor defines the custom behavior of the Slack plugin. Contains
// no data, exists only as a thing to hang a custom PluginDescriptor
// implementation on.
type slackDescriptor struct{}

func (slackDescriptor) getAuthURL(ctx context.Context, sctx *web.SessionContext, r *http.Request, typ string, state string, p *Plugin) (string, error) {
	meta, err := p.getPluginTypeMeta(ctx, sctx, typ)
	if err != nil {
		return "", trace.Wrap(err)
	}

	callbackURL := p.getPluginCallbackURL(r, typ)
	var scopes = []string{
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
		p.Log.WithError(err).Warn("Failed to issue a redirect.")
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
