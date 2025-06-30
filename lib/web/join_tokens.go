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
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/ui"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/scripts"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

const (
	HeaderTokenName = "X-Teleport-TokenName"
)

// nodeJoinToken contains node token fields for the UI.
type nodeJoinToken struct {
	//  ID is token ID.
	ID string `json:"id"`
	// Expiry is token expiration time.
	Expiry time.Time `json:"expiry"`
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

	discoveryInstallMode bool
	discoveryGroup       string
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

func (h *Handler) getTokens(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (any, error) {
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

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (any, error) {
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

func (h *Handler) updateTokenYAML(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	tokenId := r.Header.Get(HeaderTokenName)
	if tokenId == "" {
		return nil, trace.BadParameter("requires a token name to edit")
	}

	var yaml CreateTokenRequest
	if err := httplib.ReadResourceJSON(r, &yaml); err != nil {
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

func (h *Handler) upsertTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (any, error) {
	var req upsertTokenHandleRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var existingToken types.ProvisionToken
	if req.Name != "" {
		existingToken, err = clt.GetToken(r.Context(), req.Name)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	var expires time.Time
	switch req.JoinMethod {
	case types.JoinMethodGCP, types.JoinMethodIAM, types.JoinMethodOracle, types.JoinMethodGitHub:
		// IAM, GCP, Oracle and GitHub tokens should never expire.
		expires = time.Time{}
	default:
		// Set expires time to default node join token TTL.
		expires = time.Now().UTC().Add(defaults.NodeJoinTokenTTL)
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

	// If this is an edit, then overwrite the metadata to retain the existing fields
	if existingToken != nil {
		token.SetMetadata(existingToken.GetMetadata())
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

// createTokenForDiscoveryHandle creates tokens used during guided discover flows.
// V2 endpoint processes "suggestedLabels" field.
func (h *Handler) createTokenForDiscoveryHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req types.ProvisionTokenSpecV2
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
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
	if req.SuggestedLabels == nil {
		req.SuggestedLabels = make(types.Labels)
	}
	req.SuggestedLabels[types.InternalResourceIDLabel] = apiutils.Strings{uuid.NewString()}

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

func (h *Handler) getNodeJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	httplib.SetScriptHeaders(w.Header())

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: false,
		joinMethod:     r.URL.Query().Get("method"),
	}

	script, err := h.getJoinScript(r.Context(), settings)
	if err != nil {
		h.logger.InfoContext(r.Context(), "Failed to return the node install script", "error", err)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		h.logger.InfoContext(r.Context(), "Failed to return the node install script", "error", err)
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (h *Handler) getAppJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	httplib.SetScriptHeaders(w.Header())
	queryValues := r.URL.Query()

	name, err := url.QueryUnescape(queryValues.Get("name"))
	if err != nil {
		h.logger.DebugContext(r.Context(), "Failed to return the app install script",
			"query_param", "name",
			"error", err,
		)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	uri, err := url.QueryUnescape(queryValues.Get("uri"))
	if err != nil {
		h.logger.DebugContext(r.Context(), "Failed to return the app install script",
			"query_param", "uri",
			"error", err,
		)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:          params.ByName("token"),
		appInstallMode: true,
		appName:        name,
		appURI:         uri,
	}

	script, err := h.getJoinScript(r.Context(), settings)
	if err != nil {
		h.logger.InfoContext(r.Context(), "Failed to return the app install script", "error", err)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		h.logger.DebugContext(r.Context(), "Failed to return the app install script", "error", err)
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (h *Handler) getDatabaseJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	httplib.SetScriptHeaders(w.Header())

	settings := scriptSettings{
		token:               params.ByName("token"),
		databaseInstallMode: true,
	}

	script, err := h.getJoinScript(r.Context(), settings)
	if err != nil {
		h.logger.InfoContext(r.Context(), "Failed to return the database install script", "error", err)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		h.logger.DebugContext(r.Context(), "Failed to return the database install script", "error", err)
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (h *Handler) getDiscoveryJoinScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	httplib.SetScriptHeaders(w.Header())
	queryValues := r.URL.Query()
	const discoveryGroupQueryParam = "discoveryGroup"

	discoveryGroup, err := url.QueryUnescape(queryValues.Get(discoveryGroupQueryParam))
	if err != nil {
		h.logger.DebugContext(r.Context(), "Failed to return the discovery install script",
			"error", err,
			"query_param", discoveryGroupQueryParam,
		)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}
	if discoveryGroup == "" {
		h.logger.DebugContext(r.Context(), "Failed to return the discovery install script. Missing required fields",
			"query_param", discoveryGroupQueryParam,
		)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	settings := scriptSettings{
		token:                params.ByName("token"),
		discoveryInstallMode: true,
		discoveryGroup:       discoveryGroup,
	}

	script, err := h.getJoinScript(r.Context(), settings)
	if err != nil {
		h.logger.InfoContext(r.Context(), "Failed to return the discovery install script", "error", err)
		w.Write(scripts.ErrorBashScript)
		return nil, nil
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		h.logger.DebugContext(r.Context(), "Failed to return the discovery install script", "error", err)
		w.Write(scripts.ErrorBashScript)
	}

	return nil, nil
}

func (h *Handler) getJoinScript(ctx context.Context, settings scriptSettings) (string, error) {
	joinMethod := types.JoinMethod(settings.joinMethod)
	switch joinMethod {
	case types.JoinMethodUnspecified, types.JoinMethodToken:
		if err := validateJoinToken(settings.token); err != nil {
			return "", trace.Wrap(err)
		}
	case types.JoinMethodIAM:
	default:
		return "", trace.BadParameter("join method %q is not supported via script", settings.joinMethod)
	}

	clt := h.GetProxyClient()

	// The provided token can be attacker controlled, so we must validate
	// it with the backend before using it to generate the script.
	token, err := clt.GetToken(ctx, settings.token)
	if err != nil {
		return "", trace.BadParameter("invalid token")
	}

	// TODO(hugoShaka): hit the local accesspoint which has a cache instead of asking the auth every time.

	// Get the CA pin hashes of the cluster to join.
	localCAResponse, err := clt.GetClusterCACert(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return "", trace.Wrap(err)
	}

	installOpts, err := h.installScriptOptions(ctx)
	if err != nil {
		return "", trace.Wrap(err, "Building install script options")
	}

	nodeInstallOpts := scripts.InstallNodeScriptOptions{
		InstallOptions: installOpts,
		Token:          token.GetName(),
		CAPins:         caPins,
		// We are using the joinMethod from the script settings instead of the one from the token
		// to reproduce the previous script behavior. I'm also afraid that using the
		// join method from the token would provide an oracle for an attacker wanting to discover
		// the join method.
		// We might want to change this in the future to lookup the join method from the token
		// to avoid potential mismatch and allow the caller to not care about the join method.
		JoinMethod:              joinMethod,
		Labels:                  token.GetSuggestedLabels(),
		LabelMatchers:           token.GetSuggestedAgentMatcherLabels(),
		AppServiceEnabled:       settings.appInstallMode,
		AppName:                 settings.appName,
		AppURI:                  settings.appURI,
		DatabaseServiceEnabled:  settings.databaseInstallMode,
		DiscoveryServiceEnabled: settings.discoveryInstallMode,
		DiscoveryGroup:          settings.discoveryGroup,
	}

	return scripts.GetNodeInstallScript(ctx, nodeInstallOpts)
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
