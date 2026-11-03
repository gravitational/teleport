/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/integrations"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/subca"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
	"github.com/gravitational/teleport/lib/web/ui"
)

// awsRolesAnywhereConfigureTrustAnchor returns a script that configures AWS IAM Roles Anywhere Integration
// by creating:
// - IAM Roles Anywhere Trust Anchor which trusts the Teleport AWS RA CA
// - Roles Anywhere to Apps sync process:
//   - IAM Role which can be assumed by the Trust Anchor and allows the APIs required by the sync process
//   - IAM Roles Anywhere Profile which allows access to the IAM Role above
//
// It requires the following query parameters:
// - integrationName: the name of the AWS IAM Roles Anywhere Integration
// - trustAnchor: the name of the Trust Anchor to be created
// - syncRole: the name of the IAM Role to be created
// - syncProfile: the name of the IAM Roles Anywhere Profile to be created
//
// It accepts the following optional query parameters:
// - integrationScope: the scope of the integration. If set, the script prints the integration's
// URI SAN to require in the trust policy of the roles it grants access to.
func (h *Handler) awsRolesAnywhereConfigureTrustAnchor(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	ctx := r.Context()

	queryParams := r.URL.Query()

	integrationName := queryParams.Get("integrationName")
	if integrationName == "" {
		return nil, trace.BadParameter("missing integrationName param")
	}

	trustAnchorName := queryParams.Get("trustAnchor")
	if trustAnchorName == "" {
		return nil, trace.BadParameter("missing trustAnchor param")
	}
	if err := aws.IsValidIAMRolesAnywhereTrustAnchorName(trustAnchorName); err != nil {
		return nil, trace.BadParameter("invalid trustAnchor %q", trustAnchorName)
	}

	syncRoleName := queryParams.Get("syncRole")
	if syncRoleName == "" {
		return nil, trace.BadParameter("missing syncRole param")
	}
	if err := aws.IsValidIAMRoleName(syncRoleName); err != nil {
		return nil, trace.BadParameter("invalid role %q", syncRoleName)
	}

	integrationScope := queryParams.Get("integrationScope")
	if integrationScope != "" {
		if err := apiscopes.WeakValidate(integrationScope); err != nil {
			return nil, trace.BadParameter("invalid integrationScope %q", integrationScope)
		}
	}

	syncProfileName := queryParams.Get("syncProfile")
	if syncProfileName == "" {
		return nil, trace.BadParameter("missing syncProfile param")
	}
	if err := aws.IsValidIAMRolesAnywhereProfileName(syncProfileName); err != nil {
		return nil, trace.BadParameter("invalid syncProfile %q", syncProfileName)
	}

	clusterName, err := h.GetProxyClient().GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure the IntegrationName is valid.
	_, err = h.GetProxyClient().GetIntegration(ctx, integrationv1.GetIntegrationRequest_builder{Name: integrationName, Scope: integrationScope}.Build())
	// NotFound error is ignored to prevent disclosure of whether the integration exists in a public/no-auth endpoint.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	certAuthoritiesData, err := awsRATrustAnchorCertPEMs(ctx, h.GetProxyClient(), clusterName, h.cfg.Modules.IsEnterpriseBuild())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsRACACertB64 := base64.RawStdEncoding.EncodeToString(bytes.Join(certAuthoritiesData, []byte("\n")))

	// The script must execute the following command:
	// teleport integration configure awsra-trust-anchor
	argsList := []string{
		"integration", "configure", "awsra-trust-anchor",
		fmt.Sprintf("--cluster=%s", shsprintf.EscapeDefaultContext(clusterName)),
		fmt.Sprintf("--name=%s", shsprintf.EscapeDefaultContext(integrationName)),
	}

	if integrationScope != "" {
		argsList = append(argsList, fmt.Sprintf("--scope=%s", shsprintf.EscapeDefaultContext(integrationScope)))
	}

	argsList = append(argsList,
		fmt.Sprintf("--trust-anchor=%s", shsprintf.EscapeDefaultContext(trustAnchorName)),
		fmt.Sprintf("--sync-profile=%s", shsprintf.EscapeDefaultContext(syncProfileName)),
		fmt.Sprintf("--sync-role=%s", shsprintf.EscapeDefaultContext(syncRoleName)),
		fmt.Sprintf("--trust-anchor-cert-b64=%s", awsRACACertB64),
	)

	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		EntrypointArgs: strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to continue the setup.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = w.Write([]byte(script))

	return nil, trace.Wrap(err)
}

func awsRATrustAnchorCertPEMs(ctx context.Context, clt authclient.ClientI, clusterName string, isEnterpriseBuild bool) ([][]byte, error) {
	const loadKeys = false
	ca, err := clt.GetCertAuthority(ctx, types.CertAuthID{
		Type:        types.AWSRACA,
		ClusterName: clusterName,
	}, loadKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resolver, err := subca.LoadCAOverrideResolver(
		ctx,
		clt,
		isEnterpriseBuild,
		types.CertAuthorityOverrideID{
			ClusterName: clusterName,
			CAType:      string(types.AWSRACA),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPairs := append(
		ca.GetActiveKeys().TLS,
		ca.GetAdditionalTrustedKeys().TLS...,
	)

	// Active keys and additional trusted keys can repeat the same cert (e.g.
	// during a CA rotation). Deduplicate so the bundle contains each one once.
	var certPEMs [][]byte
	seen := make(map[string]struct{}, len(keyPairs))
	for _, keyPair := range keyPairs {
		certPEM := keyPair.Cert

		result, err := resolver.CalculateOverride(subca.Certificate{PEM: certPEM})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPEM = result.CACertificate.PEM

		if _, ok := seen[string(certPEM)]; ok {
			continue
		}

		seen[string(certPEM)] = struct{}{}
		certPEMs = append(certPEMs, certPEM)
	}
	if len(certPEMs) == 0 {
		return nil, trace.BadParameter("no AWS IAM Roles Anywhere CA found (this is a bug)")
	}

	return certPEMs, nil
}

// validateAWSRolesAnywhereIntegration performs a validation for the AWS Roles Anywhere Integration name.
// This ensures the integration name is not yet being used and that it is a valid name.
func (h *Handler) validateAWSRolesAnywhereIntegration(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	ctx := r.Context()

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("integration name is required")
	}

	// validate integration name.
	if err := integrations.ValidName(integrationName); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.GetIntegration(ctx, integrationv1.GetIntegrationRequest_builder{Name: integrationName}.Build())
	switch {
	case err == nil:
		return nil, trace.AlreadyExists("integration named %q already exists", integrationName)

	case trace.IsNotFound(err):

	default:
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// awsRolesAnywherePing performs an health check for the integration.
// It returns the caller identity and the number of AWS Roles Anywhere Profiles that are active.
// If a trust anchor is provided in the body, it will be used to check the connection ignoring the integration.
// Otherwise, the integration is used to check the connection.
func (h *Handler) awsRolesAnywherePing(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	ctx := r.Context()

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("integration name is required")
	}

	var req ui.AWSRolesAnywherePingRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pingRequest := &integrationv1.AWSRolesAnywherePingRequest{}

	// When creating an integration, the Ping is called with an empty integration, but Trust Anchor, Profile and Role ARNs must be provided.
	// This allow us to check if the integration is properly configured before creating it.
	switch {
	case req.TrustAnchorARN != "":
		if req.SyncRoleARN == "" || req.SyncProfileARN == "" {
			return nil, trace.BadParameter("sync role and sync profile ARNs must be provided when trust anchor ARN is provided")
		}

		pingRequest.SetCustom(integrationv1.AWSRolesAnywherePingRequestWithoutIntegration_builder{
			TrustAnchorArn: req.TrustAnchorARN,
			RoleArn:        req.SyncRoleARN,
			ProfileArn:     req.SyncProfileARN,
		}.Build())

	default:
		pingRequest.SetIntegration(integrationName)
	}

	pingResp, err := clt.IntegrationAWSRolesAnywhereClient().AWSRolesAnywherePing(ctx, pingRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSRolesAnywherePingResponse{
		ProfileCount: int(pingResp.GetProfileCount()),
		AccountID:    pingResp.GetAccountId(),
		ARN:          pingResp.GetArn(),
		UserID:       pingResp.GetUserId(),
	}, nil
}

// awsRolesAnywhereListProfiles lists profiles Roles Anywhere Profiles accessible by the integration.
func (h *Handler) awsRolesAnywhereListProfiles(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	ctx := r.Context()

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("integration name is required")
	}

	var req ui.AWSRolesAnywhereListProfilesRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allProfiles := &integrationv1.ListRolesAnywhereProfilesResponse{}
	var startKey string
	for {
		listResp, err := clt.IntegrationAWSRolesAnywhereClient().ListRolesAnywhereProfiles(ctx, integrationv1.ListRolesAnywhereProfilesRequest_builder{
			Integration:        integrationName,
			NextPageToken:      startKey,
			ProfileNameFilters: req.Filters,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allProfiles.SetProfiles(append(allProfiles.GetProfiles(), listResp.GetProfiles()...))

		startKey = listResp.GetNextPageToken()
		if startKey == "" {
			break
		}
	}

	return allProfiles, nil
}
