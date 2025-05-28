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
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
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
	_, err = h.GetProxyClient().GetIntegration(ctx, integrationName)
	// NotFound error is ignored to prevent disclosure of whether the integration exists in a public/no-auth endpoint.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	authorities, err := client.ExportAllAuthorities(
		ctx,
		h.GetProxyClient(),
		client.ExportAuthoritiesRequest{
			AuthType: string(types.AWSRACA),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(authorities) == 0 {
		return nil, trace.NotFound("no AWS IAM Roles Anywhere CA found")
	}

	var certAuthoritiesData [][]byte
	for _, authority := range authorities {
		certAuthoritiesData = append(certAuthoritiesData, authority.Data)
	}

	awsRACACertB64 := base64.RawStdEncoding.EncodeToString(bytes.Join(certAuthoritiesData, []byte("\n")))

	// The script must execute the following command:
	// teleport integration configure awsra-trust-anchor
	argsList := []string{
		"integration", "configure", "awsra-trust-anchor",
		fmt.Sprintf("--cluster=%s", shsprintf.EscapeDefaultContext(clusterName)),
		fmt.Sprintf("--name=%s", shsprintf.EscapeDefaultContext(integrationName)),
		fmt.Sprintf("--trust-anchor=%s", shsprintf.EscapeDefaultContext(trustAnchorName)),
		fmt.Sprintf("--sync-profile=%s", shsprintf.EscapeDefaultContext(syncProfileName)),
		fmt.Sprintf("--sync-role=%s", shsprintf.EscapeDefaultContext(syncRoleName)),
		fmt.Sprintf("--trust-anchor-cert-b64=%s", awsRACACertB64),
	}

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
