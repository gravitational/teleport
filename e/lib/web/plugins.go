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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
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

	// EventID is the name of the user event ID that will
	// be used to send of a "completed" event once user
	// successfully enrolls this plugin.
	// The ID is used to correlate with its plugin "starting"
	// event.
	EventID string `json:"eventId"`

	pluginOnboardingCookieNonSensitiveData
}

type pluginOnboardingCookieNonSensitiveData struct {
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
	pluginsClt, err := getPluginClientFromSessionContext(ctx)
	if err != nil {
		return nil, err
	}

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

// createPluginHandle expects html form request and
//   - For OAuth plugins: It
//   - Sets a cookie with the plugin information and "state" parameter. This parameter will be used later by pluginCallbackHandle.
//   - Responds with meta redirect. Redirect. Uses "meta" redirect,
//     since our "form-action" CSP prevents a redirect to an external domain on some browsers. See:
//     https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/form-action
//     https://github.com/w3c/webappsec-csp/issues/8
//   - For non-OAuth plugins: it creates plugin and responds with plugin status.
func (p *Plugin) createPluginHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, sessCtx *web.SessionContext) (interface{}, error) {
	pluginType := r.FormValue("type")
	pd, ok := pluginDescriptors[types.PluginType(pluginType)]
	if !ok {
		return nil, trace.BadParameter("unknown plugin type: %q", pluginType)
	}

	return pd.HandleInstallRequest(r.Context(), sessCtx, w, r, p)
}

func (p *Plugin) getPluginsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	const pageSize = apidefaults.DefaultChunkSize
	pluginsClt, err := getPluginClientFromSessionContext(ctx)
	if err != nil {
		return nil, err
	}

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
	pluginName := params.ByName("name")
	if pluginName == "" {
		return nil, trace.BadParameter("name must be specified")
	}

	pluginsClt, err := getPluginClientFromSessionContext(ctx)
	if err != nil {
		return nil, err
	}

	_, err = pluginsClt.DeletePlugin(r.Context(), &pluginspb.DeletePluginRequest{Name: pluginName})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

func (p *Plugin) pluginCallbackHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clearPluginOnboardingCookie(w)
	typ := params.ByName("type")
	if typ == "" {
		return nil, trace.BadParameter("empty type")
	}

	pd, ok := pluginDescriptors[types.PluginType(typ)]
	if !ok {
		return nil, trace.BadParameter("unknown plugin type: %q", typ)
	}

	// Where to finally redirect the user with either the success status,
	// or an error from the 3rd party provider.
	// Internal errors (such as "bad state", or CreatePlugin() failure)
	// are currently not handled with a graceful redirect.
	destURL := url.URL{
		Path: path.Join("/web/integrations/new", typ),
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		destURL.RawQuery = url.Values{
			"error":             {r.URL.Query().Get("error")},
			"error_description": {r.URL.Query().Get("error_description")},
		}.Encode()
		http.Redirect(w, r, destURL.String(), http.StatusFound)
		return nil, nil
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		return nil, trace.AccessDenied("empty state")
	}

	cookie, err := getPluginOnboardingCookie(r)
	if err != nil || subtle.ConstantTimeCompare([]byte(cookie.State), []byte(state)) == 0 {
		return nil, trace.AccessDenied("bad state")
	}

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
					AuthorizationCode: code,
					// Reconstructs the original callback URL
					RedirectUri: p.getPluginCallbackURL(r, typ),
				},
			},
		},
	}

	if err = pd.TranslateCallbackCookie(&req.Plugin.Spec, cookie); err != nil {
		if trace.IsNotImplemented(err) {
			// preserves old behavior
			return nil, trace.BadParameter("unknown plugin type")
		}
		return nil, trace.Wrap(err)
	}

	pluginsClt, err := getPluginClientFromSessionContext(ctx)
	if err != nil {
		return nil, err
	}
	_, err = pluginsClt.CreatePlugin(r.Context(), req)

	if err != nil {
		p.Log.WithError(err).Error("Failed to CreatePlugin() after web flow")
		return nil, trace.Wrap(err)
	}

	successData, err := json.Marshal(cookie.pluginOnboardingCookieNonSensitiveData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	destURL.RawQuery = url.Values{
		"success":  {string(successData)},
		"event_id": {cookie.EventID},
	}.Encode()
	http.Redirect(w, r, destURL.String(), http.StatusFound)
	return nil, nil
}

func (p *Plugin) getPluginTypeMeta(ctx context.Context, sctx *web.SessionContext, typ string) (*pluginspb.PluginType, error) {
	pluginsClt, err := getPluginClientFromSessionContext(sctx)
	if err != nil {
		return nil, err
	}
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

// getPluginClientFromSessionContext will return the plugin client from a given session context.
func getPluginClientFromSessionContext(sessCtx *web.SessionContext) (pluginspb.PluginServiceClient, error) {
	clt, err := sessCtx.GetClient()
	if err != nil {
		return nil, err
	}

	return clt.PluginsClient(), nil
}

func installPlugin(ctx context.Context, sessCtx *web.SessionContext, req *pluginspb.CreatePluginRequest, plugin *Plugin) (*ui.Plugin, error) {

	pluginsClt, err := getPluginClientFromSessionContext(sessCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = pluginsClt.CreatePlugin(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiPlugin, err := ui.NewPlugin(req.Plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return uiPlugin, nil
}
