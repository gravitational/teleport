/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package githttp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	gitcommon "github.com/gravitational/teleport/lib/srv/git/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

func makeGitMetadata(gitServer types.Server) apievents.GitMetadata {
	meta := apievents.GitMetadata{
		GitServiceType: gitServer.GetSubKind(),
		GitServerName:  gitServer.GetName(),
		GitProtocol:    types.GitProtocolHTTPS,
	}
	if gh := gitServer.GetGitHub(); gh != nil {
		meta.GitOrganization = gh.Organization
		meta.GitIntegration = gh.Integration
	}
	return meta
}

func makeServerMetadata(hostID string) apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerID:        hostID,
		ServerNamespace: apidefaults.Namespace,
	}
}

func makeSessionMetadata(identity *tlsca.Identity) apievents.SessionMetadata {
	return apievents.SessionMetadata{
		SessionID:        identity.RouteToGit.SessionID,
		WithMFA:          identity.MFAVerified,
		PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
	}
}

func makeHTTPRequestEvent(r *http.Request) *apievents.GitHTTPRequest {
	return &apievents.GitHTTPRequest{
		Metadata: apievents.Metadata{
			Type: events.GitHTTPRequestEvent,
			Code: events.GitHTTPRequestCode,
		},
		Status: apievents.Status{
			// Currently success here indicates the request was accepted.
			//
			// TODO(greedy52) set the status from the upstream response once the
			// response is recorded.
			Success: true,
		},
		Method: r.Method,
		Url:    r.URL.String(),
	}
}

func (h *Handler) emitAuditEvent(event apievents.AuditEvent) {
	if err := h.cfg.Emitter.EmitAuditEvent(h.closeContext, event); err != nil {
		h.cfg.Logger.WarnContext(h.closeContext, "Failed to emit audit event",
			"error", err,
			"event_type", event.GetType(),
			"event_code", event.GetCode(),
		)
	}
}

func (h *Handler) makeGitCommandEvent(r *http.Request, identity *tlsca.Identity, gitServer types.Server, cmd gitcommon.Command) *apievents.GitCommand {
	return &apievents.GitCommand{
		Metadata: apievents.Metadata{
			Type:        events.GitCommandEvent,
			Code:        events.GitCommandCode,
			ClusterName: h.cfg.ClusterName,
		},
		UserMetadata:    identity.GetUserMetadata(),
		SessionMetadata: makeSessionMetadata(identity),
		ServerMetadata:  makeServerMetadata(h.cfg.HostID),
		CommandMetadata: apievents.CommandMetadata{
			Command: cmd.RequestLine,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: r.RemoteAddr,
		},
		GitMetadata: makeGitMetadata(gitServer),
		Service:     cmd.Service,
		Path:        string(cmd.Repository),
	}
}

// completeGitCommandEvent sets the result of the git command on the event.
func (h *Handler) completeGitCommandEvent(event *apievents.GitCommand, cmdRecorder gitcommon.CommandRecorder, statusCode int, reqErr error) {
	event.CommandMetadata.ExitCode = "HTTP " + strconv.Itoa(statusCode)
	if reqErr != nil {
		event.Metadata.Code = events.GitCommandFailureCode
		event.CommandMetadata.Error = reqErr.Error()
	} else if statusCode >= http.StatusBadRequest {
		event.Metadata.Code = events.GitCommandFailureCode
		event.CommandMetadata.Error = http.StatusText(statusCode)
	}

	actions, err := cmdRecorder.GetActions()
	if err != nil {
		h.cfg.Logger.WarnContext(h.closeContext, "Failed to get actions from git command recorder", "error", err)
	}
	event.Actions = actions
}

// graphQLQueryForAudit returns the query of a GitHub GraphQL request body for
// audit.
//
// The query is decoded the same way GitHub does, as verified against
// api.github.com:
//   - Field names are case-sensitive, e.g. "QUERY" is ignored.
//   - For duplicate "query" fields, the last one is used.
//
// TODO(greedy52) migrate to encoding/json/v2 once it's out of experimentation.
func graphQLQueryForAudit(body []byte) (string, error) {
	var payload struct {
		Query string `json:"query"`
	}
	if err := caseSensitiveJSON.Unmarshal(body, &payload); err != nil {
		return "", trace.BadParameter("invalid GraphQL request: %v", err)
	}
	if payload.Query == "" {
		return "", trace.BadParameter("invalid GraphQL request: missing query")
	}
	return payload.Query, nil
}

// caseSensitiveJSON is based on jsoniter.ConfigCompatibleWithStandardLibrary
// with CaseSensitive enabled.
var caseSensitiveJSON = jsoniter.Config{
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
	CaseSensitive:          true,
}.Froze()

// smartHTTPRequestDetails holds parsed details of a Git smart HTTP request.
//
// https://git-scm.com/docs/http-protocol
type smartHTTPRequestDetails struct {
	repoPath    gitcommon.Repository
	service     string
	isDiscovery bool
}

func parseSmartHTTPRequest(r *http.Request) smartHTTPRequestDetails {
	urlPath := strings.TrimPrefix(r.URL.Path, "/")

	for _, service := range []string{
		transport.ReceivePackServiceName,
		transport.UploadPackServiceName,
	} {
		if repoPath, ok := strings.CutSuffix(urlPath, "/"+service); ok {
			return smartHTTPRequestDetails{
				repoPath: gitcommon.Repository(repoPath),
				service:  service,
			}
		}
	}

	if repoPath, ok := strings.CutSuffix(urlPath, "/info/refs"); ok {
		return smartHTTPRequestDetails{
			repoPath:    gitcommon.Repository(repoPath),
			service:     r.URL.Query().Get("service"),
			isDiscovery: true,
		}
	}

	return smartHTTPRequestDetails{}
}
