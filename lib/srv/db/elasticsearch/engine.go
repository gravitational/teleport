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

package elasticsearch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"

	elastic "github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new elasticsearch engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{EngineConfig: ec}
}

// Engine handles connections from Elasticsearch clients coming from Teleport
// proxy over reverse tunnel.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
}

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
	return nil
}

// SendError sends an error to Elasticsearch client.
func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}

	reason := err.Error()
	cause := elastic.ErrorCause{
		Reason: &reason,
		Type:   "internal_server_error_exception",
	}

	// Assume internal server error HTTP 500 and override if possible.
	statusCode := http.StatusInternalServerError
	if trace.IsAccessDenied(err) {
		statusCode = http.StatusUnauthorized
		cause.Type = "access_denied_exception"
	}

	jsonBody, err := json.Marshal(cause)
	if err != nil {
		e.Log.WithError(err).Error("failed to marshal error response")
		return
	}

	response := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBuffer(jsonBody)),
		Header: map[string][]string{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonBody))},
		},
	}

	if err := response.Write(e.clientConn); err != nil {
		e.Log.Errorf("elasticsearch error: %+v", trace.Unwrap(err))
		return
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target Elasticsearch server and starts proxying requests between client/server.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)

	if err := e.authorizeConnection(ctx); err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	clientConnReader := bufio.NewReader(e.clientConn)

	if sessionCtx.Identity.RouteToDatabase.Username == "" {
		return trace.BadParameter("database username required for Elasticsearch")
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	observe()

	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(e.sessionCtx.Database)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		err = e.process(ctx, sessionCtx, req, client, msgFromClient, msgFromServer)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// process reads request from connected elasticsearch client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, sessionCtx *common.Session, req *http.Request, client *http.Client, msgFromClient prometheus.Counter, msgFromServer prometheus.Counter) error {
	msgFromClient.Inc()

	if req.Body != nil {
		// make sure we close the incoming request's body. ignore any close error.
		defer req.Body.Close()
		req.Body = io.NopCloser(utils.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
	}
	payload, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return trace.Wrap(err)
	}

	copiedReq := req.Clone(ctx)
	copiedReq.RequestURI = ""
	copiedReq.Body = io.NopCloser(bytes.NewReader(payload))

	// force HTTPS, set host URL.
	copiedReq.URL.Scheme = "https"
	copiedReq.URL.Host = sessionCtx.Database.GetURI()
	copiedReq.Host = sessionCtx.Database.GetURI()

	// emit an audit event regardless of failure
	var responseStatusCode uint32
	defer func() {
		e.emitAuditEvent(copiedReq, payload, responseStatusCode, err == nil)
	}()

	// Send the request to elasticsearch API
	resp, err := client.Do(copiedReq)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	responseStatusCode = uint32(resp.StatusCode)

	msgFromServer.Inc()

	return trace.Wrap(e.sendResponse(resp))
}

// emitAuditEvent writes the request and response to audit stream.
func (e *Engine) emitAuditEvent(req *http.Request, body []byte, statusCode uint32, noErr bool) {
	var eventCode string
	if noErr && statusCode != 0 {
		eventCode = events.ElasticsearchRequestCode
	} else {
		eventCode = events.ElasticsearchRequestFailureCode
	}

	// Normally the query is passed as request body, and body content type as a header.
	// Yet it can also be passed as `source` and `source_content_type` URL params, and we handle that here.
	contentType := req.Header.Get("Content-Type")

	source := req.URL.Query().Get("source")
	if len(source) > 0 {
		e.Log.Infof("'source' parameter found, overriding request body.")
		body = []byte(source)
		contentType = req.URL.Query().Get("source_content_type")
	}

	target, category := parsePath(req.URL.Path)

	// Heuristic to calculate the query field.
	// The priority is given to 'q' URL param. If not found, we look at the request body.
	// This is not guaranteed to give us actual query, for example:
	// - we may not support given API
	// - we may not support given content encoding
	query := req.URL.Query().Get("q")
	if query == "" {
		query = GetQueryFromRequestBody(e.EngineConfig, contentType, body)
	}

	ev := &apievents.ElasticsearchRequest{
		Metadata: common.MakeEventMetadata(e.sessionCtx,
			events.DatabaseSessionElasticsearchRequestEvent,
			eventCode),
		UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
		SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
		StatusCode:       statusCode,
		Method:           req.Method,
		Path:             req.URL.Path,
		RawQuery:         req.URL.RawQuery,
		Body:             body,
		Headers:          wrappers.Traits(req.Header),
		Category:         category,
		Target:           target,
		Query:            query,
	}

	e.Audit.EmitEvent(req.Context(), ev)
}

// sendResponse sends the response back to the elasticsearch client.
func (e *Engine) sendResponse(resp *http.Response) error {
	if err := resp.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// authorizeConnection does authorization check for elasticsearch connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := e.sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     e.sessionCtx.Database,
		DatabaseUser: e.sessionCtx.DatabaseUser,
		DatabaseName: e.sessionCtx.DatabaseName,
	})
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)

	return trace.Wrap(err)
}
