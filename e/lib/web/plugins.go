package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slices"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/app"
)

var slackAuthBaseURL = "https://slack.com/oauth/v2/authorize"

const pluginOnboardingCookieName = "__Host-plugin-params"
const pluginOnboardingCookieMaxAge = 1 * time.Hour

var basePluginOnboardingCookie = http.Cookie{
	Name:     pluginOnboardingCookieName,
	Path:     "/",
	HttpOnly: true,
	MaxAge:   int(pluginOnboardingCookieMaxAge / time.Second),
	Secure:   true,
}

type pluginOnboardingParamsSlack struct {
	FallbackChannel string `json:"fallback_channel"`
}

// pluginOnboardingCookie is used at the start of plugin onboarding process
// to temporarily save information about the plugin.
// Upon successful authentication with the API provider,
// the user is redirected back to our callback
// and this information is used to finish the process.
type pluginOnboardingCookie struct {
	// OAuth2 "state" parameter
	State string `json:"state"`

	Name string `json:"name"`

	// "sum type": provider-specific settings required for onboarding
	// JSON tags match types.PluginTypeXXX constants
	Slack *pluginOnboardingParamsSlack `json:"slack"`
}

func (c *pluginOnboardingCookie) CheckAndSetDefaults() error {
	if c.State == "" {
		randBytes := make([]byte, 16)
		_, err := io.ReadFull(rand.Reader, randBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		c.State = hex.EncodeToString(randBytes)
	}

	if c.Name == "" {
		return trace.BadParameter("name must be set")
	}

	switch {
	case c.Slack != nil:
		if c.Slack.FallbackChannel == "" {
			return trace.BadParameter("fallback_channel must be set")
		}
		c.Slack.FallbackChannel = "#" + strings.TrimLeft(c.Slack.FallbackChannel, "#")
	}

	return nil
}

func clearPluginOnboardingCookie(w http.ResponseWriter) {
	httpCookie := basePluginOnboardingCookie
	httpCookie.MaxAge = -1
	http.SetCookie(w, &httpCookie)
}

func setPluginOnboardingCookie(cookie *pluginOnboardingCookie, w http.ResponseWriter) error {
	if err := cookie.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	payload, err := json.Marshal(cookie)
	if err != nil {
		return trace.Wrap(err)
	}
	value := base64.URLEncoding.EncodeToString(payload)
	httpCookie := basePluginOnboardingCookie
	httpCookie.Value = value
	http.SetCookie(w, &httpCookie)
	return nil
}

func getPluginOnboardingCookie(r *http.Request) (*pluginOnboardingCookie, error) {
	reqCookie, err := r.Cookie(pluginOnboardingCookieName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	decoded, err := base64.URLEncoding.DecodeString(reqCookie.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cookie pluginOnboardingCookie
	if err = json.Unmarshal(decoded, &cookie); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cookie, nil
}

func (p *Plugin) getAvailablePluginTypesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginsClt := clt.PluginsClient()

	resp, err := pluginsClt.GetAvailablePluginTypes(r.Context(), &pluginspb.GetAvailablePluginTypesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	availableTypes := make([]types.PluginType, 0, len(resp.PluginTypes))
	for _, typ := range resp.PluginTypes {
		availableTypes = append(availableTypes, types.PluginType(typ.Type))
	}

	return availableTypes, nil
}

// createPluginHandle accepts the initial request to create the plugin
// It then:
//   - Sets a cookie with the plugin information and "state" parameter
//     for later use by pluginCallbackHandle
//   - Redirects to the API provider to authorize
func (p *Plugin) createPluginHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	pluginType := r.FormValue("type")

	// Set cookie info
	cookie := pluginOnboardingCookie{
		Name: r.FormValue("name"),
	}
	switch pluginType {
	case types.PluginTypeSlack:
		cookie.Slack = &pluginOnboardingParamsSlack{
			FallbackChannel: r.FormValue("fallback_channel"),
		}
	default:
		return nil, trace.BadParameter("unknown plugin type")
	}
	if err := setPluginOnboardingCookie(&cookie, w); err != nil {
		return nil, trace.Wrap(err)
	}

	// Redirect. Uses "meta" redirect,
	// since our "form-action" CSP prevents a redirect to an external domain on some browsers. See:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/form-action
	// https://github.com/w3c/webappsec-csp/issues/8

	url, err := p.getPluginProviderAuthURL(r.Context(), ctx, r, pluginType, cookie.State)
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

func (p *Plugin) getPluginsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	const pageSize = apidefaults.DefaultChunkSize
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginsClt := clt.PluginsClient()

	// TODO(justinas): actually paginate
	results, err := pluginsClt.ListPlugins(r.Context(), &pluginspb.ListPluginsRequest{PageSize: pageSize})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Avoid returning nil/null to the UI when there are no plugins, make an empty slice.
	plugins := make([]*ui.Plugin, 0)

	for _, p := range results.Plugins {
		plugin, err := ui.NewPlugin(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

func (p *Plugin) deletePluginHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginName := params.ByName("name")
	if pluginName == "" {
		return nil, trace.BadParameter("name must be specified")
	}

	pluginsClt := clt.PluginsClient()

	_, err = pluginsClt.DeletePlugin(r.Context(), &pluginspb.DeletePluginRequest{Name: pluginName})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

func (p *Plugin) pluginCallbackHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clearPluginOnboardingCookie(w)

	state := r.URL.Query().Get("state")
	if state == "" {
		return nil, trace.AccessDenied("empty state")
	}

	cookie, err := getPluginOnboardingCookie(r)
	if err != nil || subtle.ConstantTimeCompare([]byte(cookie.State), []byte(state)) == 0 {
		return nil, trace.AccessDenied("bad state")
	}

	typ := params.ByName("type")

	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: cookie.Name,
				Labels: map[string]string{
					plugins.HostedPluginLabel: "true",
				},
			},
			Spec: types.PluginSpecV1{},
		},
		BootstrapCredentials: &types.PluginBootstrapCredentialsV1{
			Credentials: &types.PluginBootstrapCredentialsV1_Oauth2AuthorizationCode{
				Oauth2AuthorizationCode: &types.PluginOAuth2AuthorizationCodeCredentials{
					AuthorizationCode: r.URL.Query().Get("code"),
					// Reconstructs the original callback URL
					RedirectUri: p.getPluginCallbackURL(r, typ),
				},
			},
		},
	}

	switch typ {
	case types.PluginTypeSlack:
		if cookie.Slack == nil {
			return nil, trace.BadParameter("slack info missing")
		}
		req.Plugin.Spec.Settings = &types.PluginSpecV1_SlackAccessPlugin{
			SlackAccessPlugin: &types.PluginSlackAccessSettings{
				FallbackChannel: cookie.Slack.FallbackChannel,
			},
		}
	default:
		return nil, trace.BadParameter("unknown plugin type")
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginsClt := clt.PluginsClient()
	_, err = pluginsClt.CreatePlugin(r.Context(), req)
	if err != nil {
		p.Log.WithError(err).Error("Failed to CreatePlugin() after web flow")
		return nil, trace.Wrap(err)
	}

	// TODO(justinas): redirect to a dedicated "success" page instead,
	// once it exists on the frontend.
	http.Redirect(w, r, "/web/integrations", http.StatusFound)
	return nil, nil
}

func (p *Plugin) getPluginProviderAuthURL(ctx context.Context, sctx *web.SessionContext, r *http.Request, typ string, state string) (string, error) {
	meta, err := p.getPluginTypeMeta(ctx, sctx, typ)
	if err != nil {
		return "", trace.Wrap(err)
	}

	callbackURL := p.getPluginCallbackURL(r, typ)

	switch typ {
	case types.PluginTypeSlack:
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
	default:
		return "", trace.BadParameter("unknown plugin type")
	}
}

func (p *Plugin) getPluginTypeMeta(ctx context.Context, sctx *web.SessionContext, typ string) (*pluginspb.PluginType, error) {
	clt, err := sctx.GetClient()
	if err != nil {
		return nil, err
	}

	pluginsClt := clt.PluginsClient()
	resp, err := pluginsClt.GetAvailablePluginTypes(ctx, &pluginspb.GetAvailablePluginTypesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	foundTypeIdx := slices.IndexFunc(resp.PluginTypes, func(t *pluginspb.PluginType) bool { return t.Type == typ })
	if foundTypeIdx == -1 {
		return nil, trace.NotFound("plugin type %q not supported by the server", typ)
	}
	return resp.PluginTypes[foundTypeIdx], nil
}

// getPluginCallbackURL returns the URL to use as OAuth `redirect_uri` .
func (p *Plugin) getPluginCallbackURL(r *http.Request, typ string) string {
	callbackPath := path.Join("callback", typ)

	// When plugin shim is used, redirect to it.
	if p.PluginShimURL != nil {
		// This relies on the Cloud FQDN scheme: <tenant>.teleport.sh
		addr := p.h.PublicProxyAddr()
		tenant := strings.Split(addr, ".")[0]

		uri := *p.PluginShimURL
		uri.Path = callbackPath
		uri.RawQuery = url.Values{
			"tenant": {tenant},
		}.Encode()
		return uri.String()
	}

	// When plugin shim is not used (e.g. in debug, or in future self-hosted Enterprise),
	// redirect directly back to the proxy.
	uri := url.URL{
		Scheme: "https",
		Host:   r.Host,
		Path:   path.Join("/v1/enterprise/plugins", callbackPath),
	}
	return uri.String()
}
