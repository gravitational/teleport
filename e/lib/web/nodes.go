package web

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/gravitational/teleport/api/v7/types"
	"github.com/gravitational/teleport/e/lib/web/scripts"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"k8s.io/apimachinery/pkg/util/validation"
)

// scriptSettings is used to hold values which are passed into the function that
// generates the join script.
type scriptSettings struct {
	token          string
	appInstallMode bool
	appName        string
	appURI         string
}

func (p *Plugin) createScriptJoinTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createScriptJoinToken(r.Context(), clt)
}

func (p *Plugin) getNodeJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	scripts.SetScriptHeaders(w.Header())

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: false,
	}

	script, err := getJoinScript(settings, p.h.GetProxyClient())
	if err != nil {
		p.Log.WithError(err).Info("Failed to return the node install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		p.Log.WithError(err).Info("Failed to return the node install script.")
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (p *Plugin) getAppJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	scripts.SetScriptHeaders(w.Header())
	queryValues := r.URL.Query()

	name, err := url.QueryUnescape(queryValues.Get("name"))
	if err != nil {
		p.Log.WithField("query-param", "name").WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	uri, err := url.QueryUnescape(queryValues.Get("uri"))
	if err != nil {
		p.Log.WithField("query-param", "uri").WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: true,
		appName:        name,
		appURI:         uri,
	}

	script, err := getJoinScript(settings, p.h.GetProxyClient())
	if err != nil {
		p.Log.WithError(err).Info("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		p.Log.WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func createScriptJoinToken(ctx context.Context, m nodeAPIGetter) (*ui.NodeJoinToken, error) {
	req := auth.GenerateTokenRequest{
		Roles: types.SystemRoles{
			types.RoleNode,
			types.RoleApp,
		},
		TTL: defaults.NodeJoinTokenTTL,
	}

	token, err := m.GenerateToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.NodeJoinToken{
		ID:     token,
		Expiry: time.Now().UTC().Add(defaults.NodeJoinTokenTTL),
	}, nil
}

func getJoinScript(settings scriptSettings, m nodeAPIGetter) (string, error) {
	// This token does not need to be validated against the backend because it's not used to
	// reveal any sensitive information. However, we still need to perform a simple input
	// validation check by verifying that the token was auto-generated.
	// Auto-generated tokens must be encoded and must have an expected length.
	decodedToken, err := hex.DecodeString(settings.token)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(decodedToken) != auth.TokenLenBytes {
		return "", trace.BadParameter("invalid token length")
	}

	// Get hostname and port from proxy server address.
	proxyServers, err := m.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(proxyServers) == 0 {
		return "", trace.NotFound("no proxy servers found")
	}

	version := proxyServers[0].GetTeleportVersion()
	hostname, portStr, err := utils.SplitHostPort(proxyServers[0].GetPublicAddr())
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Get the CA pin hash of the cluster to join.
	localCA, err := m.GetClusterCACert()
	if err != nil {
		return "", trace.Wrap(err)
	}

	tlsCA, err := tlsca.ParseCertificatePEM(localCA.TLSCA)
	if err != nil {
		return "", trace.Wrap(err)
	}

	caPin := utils.CalculateSPKI(tlsCA)

	var buf bytes.Buffer
	// If app install mode is requested but parameters are blank for some reason,
	// we need to return an error.
	if settings.appInstallMode == true {
		if errs := validation.IsDNS1035Label(settings.appName); len(errs) > 0 {
			return "", trace.BadParameter("appName %q must be a valid DNS subdomain: https://gravitational.com/teleport/docs/application-access/#application-name", settings.appName)
		}
		if !appURIPattern.MatchString(settings.appURI) {
			return "", trace.BadParameter("appURI %q contains invalid characters", settings.appURI)
		}
	}
	// This section relies on Go's default zero values to make sure that the settings
	// are correct when not installing an app.
	err = scripts.InstallNodeBashScript.Execute(&buf, map[string]string{
		"token":          settings.token,
		"hostname":       hostname,
		"port":           portStr,
		"caPin":          caPin,
		"version":        version,
		"appInstallMode": strconv.FormatBool(settings.appInstallMode),
		"appName":        settings.appName,
		"appURI":         settings.appURI,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buf.String(), nil
}

type nodeAPIGetter interface {
	// GenerateToken creates a special provisioning token for a new SSH server
	// that is valid for ttl period seconds.
	//
	// This token is used by SSH server to authenticate with Auth server
	// and get signed certificate and private key from the auth server.
	//
	// If token is not supplied, it will be auto generated and returned.
	// If TTL is not supplied, token will be valid until removed.
	GenerateToken(ctx context.Context, req auth.GenerateTokenRequest) (string, error)

	// GetClusterCACert returns the CAs for the local cluster without signing keys.
	GetClusterCACert() (*auth.LocalCAResponse, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// appURIPattern is a regexp excluding invalid characters from application URIs.
var appURIPattern = regexp.MustCompile(`^[-\w/:. ]+$`)
