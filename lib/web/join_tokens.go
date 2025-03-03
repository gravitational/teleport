/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

	"github.com/google/safetext/shsprintf"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/ui"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/scripts"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

const (
	stableCloudChannelRepo = "stable/cloud"
	HeaderTokenName        = "X-Teleport-TokenName"
)

// nodeJoinToken contains node token fields for the UI.
type nodeJoinToken struct {
	//  ID is token ID.
	ID string `json:"id"`
	// Expiry is token expiration time.
	Expiry time.Time `json:"expiry,omitempty"`
	// Method is the join method that the token supports
	Method types.JoinMethod `json:"method"`
	// SuggestedLabels contains the set of labels we expect the node to set when using this token
	SuggestedLabels []ui.Label `json:"suggestedLabels,omitempty"`
}

// scriptSettings is used to hold values which are passed into the function that
// generates the join script.
type scriptSettings struct {
	token               string
	appInstallMode      bool
	appName             string
	appURI              string
	joinMethod          string
	databaseInstallMode bool
	installUpdater      bool

	discoveryInstallMode bool
	discoveryGroup       string

	// automaticUpgradesVersion is the target automatic upgrades version.
	// The version must be valid semver, with the leading 'v'. e.g. v15.0.0-dev
	// Required when installUpdater is true.
	automaticUpgradesVersion string
}

// automaticUpgrades returns whether automaticUpgrades should be enabled.
func automaticUpgrades(features proto.Features) bool {
	return features.AutomaticUpgrades && features.Cloud
}

// Currently we aren't paginating this endpoint as we don't
// expect many tokens to exist at a time. I'm leaving it in a "paginated" form
// without a nextKey for now so implementing pagination won't change the response shape
// TODO (avatus) implement pagination

// GetTokensResponse returns a list of JoinTokens.
type GetTokensResponse struct {
	Items []webui.JoinToken `json:"items"`
}

func (h *Handler) getTokens(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokens, err := clt.GetTokens(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiTokens, err := webui.MakeJoinTokens(tokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return GetTokensResponse{
		Items: uiTokens,
	}, nil
}

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	token := r.Header.Get(HeaderTokenName)
	if token == "" {
		return nil, trace.BadParameter("requires a token to delete")
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.DeleteToken(r.Context(), token); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

type CreateTokenRequest struct {
	Content string `json:"content"`
}

func (h *Handler) updateTokenYAML(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (interface{}, error) {
	tokenId := r.Header.Get(HeaderTokenName)
	if tokenId == "" {
		return nil, trace.BadParameter("requires a token name to edit")
	}

	var yaml CreateTokenRequest
	if err := httplib.ReadJSON(r, &yaml); err != nil {
		return nil, trace.Wrap(err)
	}

	extractedRes, err := ExtractResourceAndValidate(yaml.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tokenId != extractedRes.Metadata.Name {
		return nil, trace.BadParameter("renaming tokens is not supported")
	}

	token, err := services.UnmarshalProvisionToken(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.UpsertToken(r.Context(), token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiToken, err := webui.MakeJoinToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return uiToken, trace.Wrap(err)

}

type upsertTokenHandleRequest struct {
	types.ProvisionTokenSpecV2
	Name string `json:"name"`
}

func (h *Handler) upsertTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	// if using the PUT route, tokenId will be present
	// in the X-Teleport-TokenName header
	editing := r.Method == "PUT"
	tokenId := r.Header.Get(HeaderTokenName)
	if editing && tokenId == "" {
		return nil, trace.BadParameter("requires a token name to edit")
	}

	var req upsertTokenHandleRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if editing && tokenId != req.Name {
		return nil, trace.BadParameter("renaming tokens is not supported")
	}

	// set expires time to default node join token TTL
	expires := time.Now().UTC().Add(defaults.NodeJoinTokenTTL)
	// IAM and GCP tokens should never expire
	if req.JoinMethod == types.JoinMethodGCP || req.JoinMethod == types.JoinMethodIAM {
		expires = time.Now().UTC().AddDate(1000, 0, 0)
	}

	name := req.Name
	if name == "" {
		randName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		name = randName
	}

	token, err := types.NewProvisionTokenFromSpec(name, expires, req.ProvisionTokenSpecV2)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.UpsertToken(r.Context(), token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiToken, err := webui.MakeJoinToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return uiToken, nil
}

func (h *Handler) createTokenForDiscoveryHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req types.ProvisionTokenSpecV2
	if err := httplib.ReadJSON(r, &req); err != nil {
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
				Expiry: t.Expiry(),
				Method: t.GetJoinMethod(),
			}, nil
		}

		// IAM tokens should 'never' expire
		expires = time.Now().UTC().AddDate(1000, 0, 0)
	case types.JoinMethodAzure:
		tokenName, err := generateAzureTokenName(req.Azure.Allow)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		t, err := clt.GetToken(r.Context(), tokenName)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}

		v2token, ok := t.(*types.ProvisionTokenV2)
		if !ok {
			return nil, trace.BadParameter("Azure join requires v2 token")
		}

		if err == nil {
			if t.GetJoinMethod() != types.JoinMethodAzure || !isSameAzureRuleSet(req.Azure.Allow, v2token.Spec.Azure.Allow) {
				return nil, trace.BadParameter("failed to create token: token with name %q already exists and does not have the expected allow rules", tokenName)
			}

			return &nodeJoinToken{
				ID:     t.GetName(),
				Expiry: t.Expiry(),
				Method: t.GetJoinMethod(),
			}, nil
		}
	default:
		tokenName, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		expires = time.Now().UTC().Add(defaults.NodeJoinTokenTTL)
	}

	// If using the automatic method to add a Node, the `install.sh` will add the token's suggested labels
	//   as part of the initial Labels configuration for that Node
	// Script install-node.sh:
	//   ...
	//   $ teleport configure ... --labels <suggested_label=value>,<suggested_label=value> ...
	//   ...
	//
	// We create an ID and return it as part of the Token, so the UI can use this ID to query the Node that joined using this token
	// WebUI can then query the resources by this id and answer the question:
	//   - Which Node joined the cluster from this token Y?
	req.SuggestedLabels = types.Labels{
		types.InternalResourceIDLabel: apiutils.Strings{uuid.NewString()},
	}

	provisionToken, err := types.NewProvisionTokenFromSpec(tokenName, expires, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.CreateToken(r.Context(), provisionToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	suggestedLabels := make([]ui.Label, 0, len(req.SuggestedLabels))

	for labelKey, labelValues := range req.SuggestedLabels {
		suggestedLabels = append(suggestedLabels, ui.Label{
			Name:  labelKey,
			Value: strings.Join(labelValues, " "),
		})
	}

	return &nodeJoinToken{
		ID:              tokenName,
		Expiry:          expires,
		Method:          provisionToken.GetJoinMethod(),
		SuggestedLabels: suggestedLabels,
	}, nil
}

// getAutoUpgrades checks if automaticUpgrades are enabled and returns the
// version that should be used according to auto upgrades default channel.
func (h *Handler) getAutoUpgrades(ctx context.Context) (bool, string, error) {
	var autoUpgradesVersion string
	var err error
	autoUpgrades := automaticUpgrades(h.GetClusterFeatures())
	if autoUpgrades {
		autoUpgradesVersion, err = h.cfg.AutomaticUpgradesChannels.DefaultVersion(ctx)
		if err != nil {
			log.WithError(err).Info("Failed to get auto upgrades version.")
			return false, "", trace.Wrap(err)
		}
	}
	return autoUpgrades, autoUpgradesVersion, nil

}

func (h *Handler) getNodeJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	httplib.SetScriptHeaders(w.Header())

	autoUpgrades, autoUpgradesVersion, err := h.getAutoUpgrades(r.Context())
	if err != nil {
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:                    params.ByName("token"),
		appInstallMode:           false,
		joinMethod:               r.URL.Query().Get("method"),
		installUpdater:           autoUpgrades,
		automaticUpgradesVersion: autoUpgradesVersion,
	}

	script, err := getJoinScript(r.Context(), settings, h.GetProxyClient())
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
	httplib.SetScriptHeaders(w.Header())
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

	autoUpgrades, autoUpgradesVersion, err := h.getAutoUpgrades(r.Context())
	if err != nil {
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:                    params.ByName("token"),
		appInstallMode:           true,
		appName:                  name,
		appURI:                   uri,
		installUpdater:           autoUpgrades,
		automaticUpgradesVersion: autoUpgradesVersion,
	}

	script, err := getJoinScript(r.Context(), settings, h.GetProxyClient())
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

func (h *Handler) getDatabaseJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	httplib.SetScriptHeaders(w.Header())

	autoUpgrades, autoUpgradesVersion, err := h.getAutoUpgrades(r.Context())
	if err != nil {
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:                    params.ByName("token"),
		databaseInstallMode:      true,
		installUpdater:           autoUpgrades,
		automaticUpgradesVersion: autoUpgradesVersion,
	}

	script, err := getJoinScript(r.Context(), settings, h.GetProxyClient())
	if err != nil {
		log.WithError(err).Info("Failed to return the database install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		log.WithError(err).Debug("Failed to return the database install script.")
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (h *Handler) getDiscoveryJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	httplib.SetScriptHeaders(w.Header())
	queryValues := r.URL.Query()
	const discoveryGroupQueryParam = "discoveryGroup"

	autoUpgrades, autoUpgradesVersion, err := h.getAutoUpgrades(r.Context())
	if err != nil {
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	discoveryGroup, err := url.QueryUnescape(queryValues.Get(discoveryGroupQueryParam))
	if err != nil {
		log.WithField("query-param", discoveryGroupQueryParam).WithError(err).Debug("Failed to return the discovery install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}
	if discoveryGroup == "" {
		log.WithField("query-param", discoveryGroupQueryParam).Debug("Failed to return the discovery install script. Missing required fields.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:                    params.ByName("token"),
		discoveryInstallMode:     true,
		discoveryGroup:           discoveryGroup,
		installUpdater:           autoUpgrades,
		automaticUpgradesVersion: autoUpgradesVersion,
	}

	script, err := getJoinScript(r.Context(), settings, h.GetProxyClient())
	if err != nil {
		log.WithError(err).Info("Failed to return the discovery install script.")
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		log.WithError(err).Debug("Failed to return the discovery install script.")
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func getJoinScript(ctx context.Context, settings scriptSettings, m nodeAPIGetter) (string, error) {
	switch types.JoinMethod(settings.joinMethod) {
	case types.JoinMethodUnspecified, types.JoinMethodToken:
		if err := validateJoinToken(settings.token); err != nil {
			return "", trace.Wrap(err)
		}
	case types.JoinMethodIAM:
	default:
		return "", trace.BadParameter("join method %q is not supported via script", settings.joinMethod)
	}

	// The provided token can be attacker controlled, so we must validate
	// it with the backend before using it to generate the script.
	token, err := m.GetToken(ctx, settings.token)
	if err != nil {
		return "", trace.BadParameter("invalid token")
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

	publicAddr := proxyServers[0].GetPublicAddr()
	if publicAddr == "" {
		return "", trace.Errorf("proxy public_addr is not set, you must set proxy_service.public_addr to the publicly reachable address of the proxy before you can generate a node join script")
	}

	hostname, portStr, err := utils.SplitHostPort(publicAddr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Get the CA pin hashes of the cluster to join.
	localCAResponse, err := m.GetClusterCACert(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return "", trace.Wrap(err)
	}

	labelsList := []string{}
	for labelKey, labelValues := range token.GetSuggestedLabels() {
		labelKey = shsprintf.EscapeDefaultContext(labelKey)
		for i := range labelValues {
			labelValues[i] = shsprintf.EscapeDefaultContext(labelValues[i])
		}
		labels := strings.Join(labelValues, " ")
		labelsList = append(labelsList, fmt.Sprintf("%s=%s", labelKey, labels))
	}

	var dbServiceResourceLabels []string
	if settings.databaseInstallMode {
		suggestedAgentMatcherLabels := token.GetSuggestedAgentMatcherLabels()
		dbServiceResourceLabels, err = scripts.MarshalLabelsYAML(suggestedAgentMatcherLabels, 6)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	var buf bytes.Buffer
	// If app install mode is requested but parameters are blank for some reason,
	// we need to return an error.
	if settings.appInstallMode {
		if errs := validation.IsDNS1035Label(settings.appName); len(errs) > 0 {
			return "", trace.BadParameter("appName %q must be a valid DNS subdomain: https://goteleport.com/docs/enroll-resources/application-access/guides/connecting-apps/#application-name", settings.appName)
		}
		if !appURIPattern.MatchString(settings.appURI) {
			return "", trace.BadParameter("appURI %q contains invalid characters", settings.appURI)
		}
	}

	if settings.discoveryInstallMode {
		if settings.discoveryGroup == "" {
			return "", trace.BadParameter("discovery group is required")
		}
	}

	packageName := types.PackageNameOSS
	if modules.GetModules().BuildType() == modules.BuildEnterprise {
		packageName = types.PackageNameEnt
	}

	// By default, it will use `stable/v<majorVersion>`, eg stable/v12
	repoChannel := ""

	// The install script will install the updater (teleport-ent-updater) for Cloud customers enrolled in Automatic Upgrades.
	// The repo channel used must be `stable/cloud` which has the available packages for the Cloud Customer's agents.
	// It pins the teleport version to the one specified by the default version channel
	// This ensures the initial installed version is the same as the `teleport-ent-updater` would install.
	if settings.installUpdater {
		if settings.automaticUpgradesVersion == "" {
			return "", trace.Wrap(err, "automatic upgrades version must be set when installUpdater is true")
		}

		repoChannel = stableCloudChannelRepo
		// automaticUpgradesVersion has vX.Y.Z format, however the script
		// expects the version to not include the `v` so we strip it
		version = strings.TrimPrefix(settings.automaticUpgradesVersion, "v")
	}

	// This section relies on Go's default zero values to make sure that the settings
	// are correct when not installing an app.
	err = scripts.InstallNodeBashScript.Execute(&buf, map[string]interface{}{
		"token":    shsprintf.EscapeDefaultContext(settings.token),
		"hostname": hostname,
		"port":     portStr,
		// The install.sh script has some manually generated configs and some
		// generated by the `teleport <service> config` commands. The old bash
		// version used space delimited values whereas the teleport command uses
		// a comma delimeter. The Old version can be removed when the install.sh
		// file has been completely converted over.
		"caPinsOld":                  strings.Join(caPins, " "),
		"caPins":                     strings.Join(caPins, ","),
		"packageName":                packageName,
		"repoChannel":                repoChannel,
		"installUpdater":             strconv.FormatBool(settings.installUpdater),
		"version":                    shsprintf.EscapeDefaultContext(version),
		"appInstallMode":             strconv.FormatBool(settings.appInstallMode),
		"appName":                    shsprintf.EscapeDefaultContext(settings.appName),
		"appURI":                     shsprintf.EscapeDefaultContext(settings.appURI),
		"joinMethod":                 shsprintf.EscapeDefaultContext(settings.joinMethod),
		"labels":                     strings.Join(labelsList, ","),
		"databaseInstallMode":        strconv.FormatBool(settings.databaseInstallMode),
		"db_service_resource_labels": dbServiceResourceLabels,
		"discoveryInstallMode":       settings.discoveryInstallMode,
		"discoveryGroup":             shsprintf.EscapeDefaultContext(settings.discoveryGroup),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buf.String(), nil
}

// validateJoinToken validate a join token.
func validateJoinToken(token string) error {
	decodedToken, err := hex.DecodeString(token)
	if err != nil {
		return trace.BadParameter("invalid token %q", token)
	}
	if len(decodedToken) != defaults.TokenLenBytes {
		return trace.BadParameter("invalid token %q", decodedToken)
	}

	return nil
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

// generateAzureTokenName makes a deterministic name for an azure join token
// based on its rule set.
func generateAzureTokenName(rules []*types.ProvisionTokenSpecV2Azure_Rule) (string, error) {
	orderedRules := make([]*types.ProvisionTokenSpecV2Azure_Rule, len(rules))
	copy(orderedRules, rules)
	sortAzureRules(orderedRules)

	h := fnv.New32a()
	for _, r := range orderedRules {
		_, err := h.Write([]byte(r.Subscription))
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return fmt.Sprintf("teleport-ui-azure-%d", h.Sum32()), nil
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

// sortAzureRules sorts a slice of Azure rules based on their subscription.
func sortAzureRules(rules []*types.ProvisionTokenSpecV2Azure_Rule) {
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Subscription < rules[j].Subscription
	})
}

// isSameRuleSet check if r1 and r2 are the same rules, ignoring the order
func isSameRuleSet(r1 []*types.TokenRule, r2 []*types.TokenRule) bool {
	sortRules(r1)
	sortRules(r2)
	return reflect.DeepEqual(r1, r2)
}

// isSameAzureRuleSet checks if r1 and r2 are the same rules, ignoring order.
func isSameAzureRuleSet(r1, r2 []*types.ProvisionTokenSpecV2Azure_Rule) bool {
	sortAzureRules(r1)
	sortAzureRules(r2)
	return reflect.DeepEqual(r1, r2)
}

type nodeAPIGetter interface {
	// GetToken looks up a provisioning token.
	GetToken(ctx context.Context, token string) (types.ProvisionToken, error)

	// GetClusterCACert returns the CAs for the local cluster without signing keys.
	GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// appURIPattern is a regexp excluding invalid characters from application URIs.
var appURIPattern = regexp.MustCompile(`^[-\w/:. ]+$`)
