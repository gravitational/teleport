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
	"hash/fnv"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
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
	// Method is the join method that the token supports
	Method types.JoinMethod `json:"method"`
}

// scriptSettings is used to hold values which are passed into the function that
// generates the join script.
type scriptSettings struct {
	token          string
	appInstallMode bool
	appName        string
	appURI         string
	joinMethod     string
	nodeLabels     string
}

func (h *Handler) createTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req types.ProvisionTokenSpecV2
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var expires time.Time
	var tokenName string
	switch req.JoinMethod {
	case types.JoinMethodIAM:
		// to prevent generation of redundant IAM tokens
		// we generate a deterministic name for them
		tokenName, err = generateIAMTokenName(req.Allow)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// if a token with this name is found and it has indeed the same rule set,
		// return it. Otherwise, go ahead and create it
		t, err := clt.GetToken(r.Context(), tokenName)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}

		if err == nil {
			// check if the token found has the right rules
			if t.GetJoinMethod() != types.JoinMethodIAM || !isSameRuleSet(req.Allow, t.GetAllowRules()) {
				return nil, trace.BadParameter("failed to create token: token with name %q already exists and does not have the expected allow rules", tokenName)
			}

			return &nodeJoinToken{
				ID:     t.GetName(),
				Expiry: *t.GetMetadata().Expires,
				Method: t.GetJoinMethod(),
			}, nil
		}

		// IAM tokens should 'never' expire
		expires = time.Now().UTC().AddDate(1000, 0, 0)
	default:
		tokenName, err = utils.CryptoRandomHex(auth.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		expires = time.Now().UTC().Add(defaults.NodeJoinTokenTTL)
	}

	provisionToken, err := types.NewProvisionTokenFromSpec(tokenName, expires, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.UpsertToken(r.Context(), provisionToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &nodeJoinToken{
		ID:     tokenName,
		Expiry: expires,
		Method: provisionToken.GetJoinMethod(),
	}, nil
}

func (h *Handler) createNodeTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles := types.SystemRoles{
		types.RoleNode,
		types.RoleApp,
	}

	return createJoinToken(r.Context(), clt, roles)
}

func (h *Handler) getNodeJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	scripts.SetScriptHeaders(w.Header())
	queryValues := r.URL.Query()

	nodeLabels, err := url.QueryUnescape(queryValues.Get("node-labels"))
	if err != nil {
		log.WithField("query-param", "node-labels").WithError(err).Debug("Failed to return the app install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: false,
		joinMethod:     r.URL.Query().Get("method"),
		nodeLabels:     nodeLabels,
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

func createJoinToken(ctx context.Context, m nodeAPIGetter, roles types.SystemRoles) (*nodeJoinToken, error) {
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
	// Skip decoding validation for IAM tokens since they are generated with a different method
	if settings.joinMethod != string(types.JoinMethodIAM) {
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
	localCAResponse, err := m.GetClusterCACert(context.TODO())
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
		"token":    settings.token,
		"hostname": hostname,
		"port":     portStr,
		// The install.sh script has some manually generated configs and some
		// generated by the `teleport <service> config` commands. The old bash
		// version used space delimited values whereas the teleport command uses
		// a comma delimeter. The Old version can be removed when the install.sh
		// file has been completely converted over.
		"caPinsOld":      strings.Join(caPins, " "),
		"caPins":         strings.Join(caPins, ","),
		"version":        version,
		"appInstallMode": strconv.FormatBool(settings.appInstallMode),
		"appName":        settings.appName,
		"appURI":         settings.appURI,
		"joinMethod":     settings.joinMethod,
		"nodeLabels":     settings.nodeLabels,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buf.String(), nil
}

// generateIAMTokenName makes a deterministic name for a iam join token
// based on its rule set
func generateIAMTokenName(rules []*types.TokenRule) (string, error) {
	// sort the rules by (account ID, arn)
	// to make sure a set of rules will produce the same hash,
	// no matter the order they are in the slice
	orderedRules := make([]*types.TokenRule, len(rules))
	copy(orderedRules, rules)
	sortRules(orderedRules)

	h := fnv.New32a()
	for _, r := range orderedRules {
		s := fmt.Sprintf("%s%s", r.AWSAccount, r.AWSARN)
		_, err := h.Write([]byte(s))
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return fmt.Sprintf("teleport-ui-iam-%d", h.Sum32()), nil
}

// sortRules sorts a slice of rules based on their AWS Account ID and ARN
func sortRules(rules []*types.TokenRule) {
	sort.Slice(rules, func(i, j int) bool {
		iAcct, jAcct := rules[i].AWSAccount, rules[j].AWSAccount
		// if accountID is the same, sort based on arn
		if iAcct == jAcct {
			arn1, arn2 := rules[i].AWSARN, rules[j].AWSARN
			return arn1 < arn2
		}

		return iAcct < jAcct
	})
}

// isSameRuleSet check if r1 and r2 are the same rules, ignoring the order
func isSameRuleSet(r1 []*types.TokenRule, r2 []*types.TokenRule) bool {
	sortRules(r1)
	sortRules(r2)
	return reflect.DeepEqual(r1, r2)
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
	GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// appURIPattern is a regexp excluding invalid characters from application URIs.
var appURIPattern = regexp.MustCompile(`^[-\w/:. ]+$`)
