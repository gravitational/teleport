/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package bigquery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	apigcputils "github.com/gravitational/teleport/api/utils/gcp"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
	gcputils "github.com/gravitational/teleport/lib/utils/gcp"
)

const (
	// maxAuditBytes is the number of request body bytes captured for SQL
	// query extraction. 64 KB is large enough to capture complex analytics
	// queries with multiple CTEs while keeping memory usage bounded.
	maxAuditBytes = 64 * 1024
)

// errorResponse is the JSON body sent to the client when SendError is called.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

// errorDetail holds the error code and message within an errorResponse.
type errorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// queryRequest matches the BigQuery jobs.query payload: {"query": "..."}
type queryRequest struct {
	Query string `json:"query"`
}

// jobRequest matches the BigQuery jobs.insert payload:
// {"configuration": {"query": {"query": "..."}}}
type jobRequest struct {
	Configuration struct {
		Query struct {
			Query string `json:"query"`
		} `json:"query"`
	} `json:"configuration"`
}

// Compile-time interface assertions.
var (
	_ common.Engine = (*Engine)(nil)
)

// NewEngine creates a new BigQuery engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements common.Engine for BigQuery.
type Engine struct {
	common.EngineConfig
	clientConn  net.Conn
	sessionCtx  *common.Session
	tokenSource oauth2.TokenSource
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
	return nil
}

// SendError sends an error to the BigQuery client.
func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}
	e.Log.ErrorContext(e.Context, "BigQuery connection error", "error", err)

	code := trace.ErrorToCode(err)
	body, marshalErr := json.Marshal(errorResponse{
		Error: errorDetail{Code: code, Message: err.Error()},
	})
	if marshalErr != nil {
		e.Log.ErrorContext(e.Context, "Failed to marshal error response", "error", marshalErr)
		return
	}

	response := &http.Response{
		Status:        http.StatusText(code),
		StatusCode:    code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		ContentLength: int64(len(body)),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(body)),
	}
	if writeErr := response.Write(e.clientConn); writeErr != nil {
		e.Log.ErrorContext(e.Context, "Failed to send error response to BigQuery client", "error", writeErr)
	}
}

// HandleConnection processes the connection from the proxy.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(e.sessionCtx.Database)

	// Resolve sets sessionCtx.DatabaseUser from GCP credentials if not provided.
	if err := resolveDefaultDatabaseUser(ctx, e.sessionCtx, e.Log); err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}

	err := e.checkAccess(ctx, e.sessionCtx)
	e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
	if err != nil {
		return trace.Wrap(err)
	}
	defer e.Audit.OnSessionEnd(e.Context, e.sessionCtx)

	endpoint := resolveEndpoint(e.sessionCtx.Database.GetURI())
	targetURL, err := url.Parse(endpoint)
	if err != nil {
		return trace.Wrap(err)
	}

	serviceAccount := databaseUserToGCPServiceAccount(e.sessionCtx)
	e.tokenSource, err = e.Auth.GetBigQueryTokenSource(ctx, serviceAccount)
	if err != nil {
		return trace.Wrap(err, "failed to get BigQuery token source")
	}

	observe()

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx.GetExpiry(), e.sessionCtx.Database, e.sessionCtx.DatabaseUser)
	if err != nil {
		return trace.Wrap(err, "failed to get TLS config")
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	httpClient := &http.Client{
		Transport: transport,
	}
	defer transport.CloseIdleConnections()

	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(e.sessionCtx.Database)

	reader := bufio.NewReader(e.clientConn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.ReadRequest(reader)
		if err != nil {
			if err == io.EOF || utils.IsOKNetworkError(err) {
				return nil
			}
			return trace.Wrap(err)
		}

		msgFromClient.Inc()

		if err := e.roundTrip(ctx, httpClient, req, targetURL, msgFromServer); err != nil {
			return trace.Wrap(err)
		}
	}
}

// roundTrip forwards a single request to the BigQuery API and writes the response back to the client.
// It tees the request body into an audit buffer as bytes stream to BigQuery, then emits an audit
// event after the response is received.
func (e *Engine) roundTrip(ctx context.Context, client *http.Client, req *http.Request, targetURL *url.URL, msgFromServer prometheus.Counter) error {
	var auditBuf bytes.Buffer
	if req.Body != nil {
		tee := io.TeeReader(io.LimitReader(req.Body, maxAuditBytes), &auditBuf)
		req.Body = io.NopCloser(io.MultiReader(tee, req.Body))
	}

	req.RequestURI = ""
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	req.Host = targetURL.Host

	tok, err := e.tokenSource.Token()
	if err != nil {
		return trace.Wrap(err, "failed to get BigQuery access token")
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Del("X-Forwarded-For")
	req.Header.Del("X-Forwarded-Host")
	req.Header.Del("X-Forwarded-Proto")

	var responseStatusCode uint32
	defer func() {
		e.emitAuditEvent(ctx, req, auditBuf.Bytes(), responseStatusCode)
	}()

	resp, err := client.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	responseStatusCode = uint32(resp.StatusCode)
	msgFromServer.Inc()
	return trace.Wrap(resp.Write(e.clientConn))
}


func (e *Engine) emitAuditEvent(ctx context.Context, req *http.Request, body []byte, statusCode uint32) {
	query := e.extractQuery(req.URL.Path, body)
	if query == "" {
		// Non-SQL requests (list tables, get schema, job status checks, etc.)
		// are not audited as queries.
		return
	}

	var queryErr error
	if statusCode >= http.StatusBadRequest {
		queryErr = trace.Errorf("BigQuery API returned HTTP status %d", statusCode)
	}

	e.Audit.OnQuery(ctx, e.sessionCtx, common.Query{
		Query: query,
		Error: queryErr,
	})
}

// resolveEndpoint determines the BigQuery API endpoint URL from the database URI.
func resolveEndpoint(uri string) string {
	if uri == "" {
		uri = apigcputils.BigQueryEndpoint
	}
	if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
		return "https://" + uri
	}
	return uri
}

// databaseUserToGCPServiceAccount formats a Teleport database user as a GCP service account email.
func databaseUserToGCPServiceAccount(sessionCtx *common.Session) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com",
		sessionCtx.DatabaseUser,
		sessionCtx.Database.GetGCP().ProjectID,
	)
}

// checkAccess does authorization check for BigQuery connection about to be established.
func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     sessionCtx.Database,
		DatabaseUser: sessionCtx.DatabaseUser,
		DatabaseName: sessionCtx.DatabaseName,
	})
	return trace.Wrap(sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	))
}

// extractQuery extracts the SQL query from the request body based on the endpoint.
func (e *Engine) extractQuery(path string, body []byte) string {
	if !strings.Contains(path, "/queries") && !strings.Contains(path, "/jobs") {
		return ""
	}

	var qr queryRequest
	if json.Unmarshal(body, &qr) == nil && qr.Query != "" {
		return qr.Query
	}

	var jr jobRequest
	if json.Unmarshal(body, &jr) == nil && jr.Configuration.Query.Query != "" {
		return jr.Configuration.Query.Query
	}

	return ""
}

func resolveDefaultDatabaseUser(ctx context.Context, sessionCtx *common.Session, log *slog.Logger) error {
	if sessionCtx.DatabaseUser != "" {
		return nil
	}

	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return trace.Wrap(err,
			"no --db-user provided and could not auto-detect from GCP credentials; "+
				"either provide --db-user or ensure GOOGLE_APPLICATION_CREDENTIALS is set on the Teleport agent")
	}

	email, err := gcputils.GetServiceAccountFromCredentials(creds)
	if err != nil {
		return trace.Wrap(err,
			"no --db-user provided and could not extract service account from GCP credentials; "+
				"provide --db-user explicitly")
	}

	// Extract name before '@' because databaseUserToGCPServiceAccount()
	// reconstructs the full email as name@project.iam.gserviceaccount.com.
	name := email
	if idx := strings.Index(email, "@"); idx >= 0 {
		name = email[:idx]
	}

	log.InfoContext(ctx, "No database user specified, using service account from GCP credentials",
		"service_account", email,
	)
	sessionCtx.DatabaseUser = name
	return nil
}
