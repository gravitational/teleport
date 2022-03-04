/*
Copyright 2015-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/scripts"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/util/validation"
)

// nodeJoinToken contains node token fields for the UI.
type nodeJoinToken struct {
	//  ID is token ID.
	ID string `json:"id"`
	// Expiry is token expiration time.
	Expiry time.Time `json:"expiry,omitempty"`
}

// scriptSettings is used to hold values which are passed into the function that
// generates the join script.
type scriptSettings struct {
	token          string
	appInstallMode bool
	appName        string
	appURI         string
}

func (h *Handler) createScriptJoinTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles := types.SystemRoles{
		types.RoleNode,
		types.RoleApp,
	}

	return createScriptJoinToken(r.Context(), clt, roles)
}

func (h *Handler) createDatabaseJoinTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return createScriptJoinToken(r.Context(), clt, types.SystemRoles{
		types.RoleDatabase,
	})
}

func (h *Handler) getNodeJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	scripts.SetScriptHeaders(w.Header())

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: false,
	}

	script, err := getJoinScript(settings, h.GetProxyClient())
	if err != nil {
		log.WithError(err).Info("Failed to return the node install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		log.WithError(err).Info("Failed to return the node install script.")
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (h *Handler) getAppJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	scripts.SetScriptHeaders(w.Header())
	queryValues := r.URL.Query()

	name, err := url.QueryUnescape(queryValues.Get("name"))
	if err != nil {
		log.WithField("query-param", "name").WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	uri, err := url.QueryUnescape(queryValues.Get("uri"))
	if err != nil {
		log.WithField("query-param", "uri").WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: true,
		appName:        name,
		appURI:         uri,
	}

	script, err := getJoinScript(settings, h.GetProxyClient())
	if err != nil {
		log.WithError(err).Info("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		log.WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func createScriptJoinToken(ctx context.Context, m nodeAPIGetter, roles types.SystemRoles) (*nodeJoinToken, error) {
	req := auth.GenerateTokenRequest{
		Roles: roles,
		TTL:   defaults.NodeJoinTokenTTL,
	}

	token, err := m.GenerateToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &nodeJoinToken{
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

	// Get the CA pin hashes of the cluster to join.
	localCAResponse, err := m.GetClusterCACert()
	if err != nil {
		return "", trace.Wrap(err)
	}
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return "", trace.Wrap(err)
	}

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
		"caPins":         strings.Join(caPins, " "),
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
