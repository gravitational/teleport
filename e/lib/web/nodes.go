package web

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/lib/web/scripts"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (p *Plugin) createNodeJoinTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createNodeJoinToken(r.Context(), clt)
}

func (p *Plugin) getNodeJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	scripts.SetScriptHeaders(w.Header())
	token := params.ByName("token")

	script, err := getNodeJoinScript(token, p.ProxyClient)
	if err != nil {
		log.WithError(err).Info("Failed to return the install node script.")
		http.Error(w, scripts.ErrorBashScript, http.StatusBadRequest)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		log.WithError(err).Debug("Failed to return the install node script.")
		http.Error(w, scripts.ErrorBashScript, http.StatusInternalServerError)
	}

	return nil, nil
}

func createNodeJoinToken(ctx context.Context, m nodeAPIGetter) (*ui.NodeJoinToken, error) {
	req := auth.GenerateTokenRequest{
		Roles: teleport.Roles{teleport.RoleNode},
		TTL:   defaults.NodeJoinTokenTTL,
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

func getNodeJoinScript(token string, m nodeAPIGetter) (string, error) {
	// This token does not need to be validated against the backend because it's not used to
	// reveal any sensitive information. However, we still need to perform a simple input
	// validation check by verifying that the token was auto-generated.
	// Auto-generated tokens must be encoded and must have an expected length.
	decodedToken, err := hex.DecodeString(token)
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
	err = scripts.InstallNodeBashScript.Execute(&buf, map[string]string{
		"token":    token,
		"hostname": hostname,
		"port":     portStr,
		"caPin":    caPin,
		"version":  version,
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
	GetProxies() ([]services.Server, error)
}
