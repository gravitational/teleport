/*

 Copyright 2022 Gravitational, Inc.

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
	"strings"

	elastic "github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

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
	if err := e.authorizeConnection(ctx); err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

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

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		err = e.process(ctx, sessionCtx, req, client)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func copyRequest(ctx context.Context, req *http.Request, body io.Reader) (*http.Request, error) {
	reqCopy, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy.Header = req.Header.Clone()

	return reqCopy, nil
}

// process reads request from connected elasticsearch client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, sessionCtx *common.Session, req *http.Request, client *http.Client) error {
	body, err := io.ReadAll(io.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		return trace.Wrap(err)
	}

	reqCopy, err := copyRequest(ctx, req, bytes.NewReader(body))
	if err != nil {
		return trace.Wrap(err)
	}
	defer req.Body.Close()

	e.emitAuditEvent(reqCopy, body)

	// force HTTPS, set host URL.
	reqCopy.URL.Scheme = "https"
	reqCopy.URL.Host = sessionCtx.Database.GetURI()

	// Send the request to elasticsearch API
	resp, err := client.Do(reqCopy)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	return trace.Wrap(e.sendResponse(resp))
}

// parsePath returns (optional) target of query as well as the event category.
func parsePath(path string) (string, apievents.ElasticsearchCategory) {
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
	}

	// first term starts with _
	switch parts[1] {
	case "_security", "_ssl":
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SECURITY
	case
		"_search",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-search.html
		"_async_search", // https://www.elastic.co/guide/en/elasticsearch/reference/master/async-search.html
		"_pit",          // https://www.elastic.co/guide/en/elasticsearch/reference/master/point-in-time-api.html
		"_msearch",      // https://www.elastic.co/guide/en/elasticsearch/reference/master/multi-search-template.html, https://www.elastic.co/guide/en/elasticsearch/reference/master/search-multi-search.html
		"_render",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/render-search-template-api.html
		"_field_caps":   // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-field-caps.html
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH
	case "_sql":
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SQL
	}

	// starts with _, but we don't handle it explicitly
	if strings.HasPrefix("_", parts[1]) {
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
	}

	if len(parts) < 3 {
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
	}

	// a number of APIs are invoked by providing a target first, e.g. /<target>/_search, where <target> is an index or expression matching a group of indices.
	switch parts[2] {
	case
		"_search",        // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-search.html
		"_async_search",  // https://www.elastic.co/guide/en/elasticsearch/reference/master/async-search.html
		"_pit",           // https://www.elastic.co/guide/en/elasticsearch/reference/master/point-in-time-api.html
		"_knn_search",    // https://www.elastic.co/guide/en/elasticsearch/reference/master/knn-search-api.html
		"_msearch",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/multi-search-template.html, https://www.elastic.co/guide/en/elasticsearch/reference/master/search-multi-search.html
		"_search_shards", // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-shards.html
		"_count",         // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-count.html
		"_validate",      // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-validate.html
		"_terms_enum",    // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-terms-enum.html
		"_explain",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-explain.html
		"_field_caps",    // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-field-caps.html
		"_rank_eval",     // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-rank-eval.html
		"_mvt":           // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-vector-tile-api.html
		return parts[1], apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH
	}

	return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
}

// getQueryFromRequestBody attempts to find the actual query from the request body, to be shown to the interested user.
func (e *Engine) getQueryFromRequestBody(contentType string, body []byte) string {
	// Elasticsearch APIs have no shared schema, but the ones we support have the query either
	// as 'query' or as 'knn'.
	// We will attempt to deserialize the query as 'q' to discover these fields.
	// The type for those is 'any': both strings and objects can be found.
	var q struct {
		Query any `json:"query" yaml:"query"`
		Knn   any `json:"knn" yaml:"knn"`
	}

	switch contentType {
	// CBOR and Smile are officially supported by Elasticsearch:
	// https://www.elastic.co/guide/en/elasticsearch/reference/master/api-conventions.html#_content_type_requirements
	// We don't support introspection of these content types, at least for now.
	case "application/cbor":
		e.Log.Warnf("Content type not supported: %q.", contentType)
		return ""

	case "application/smile":
		e.Log.Warnf("Content type not supported: %q.", contentType)
		return ""

	case "application/yaml":
		if len(body) == 0 {
			e.Log.WithField("content-type", contentType).Infof("Empty request body.")
			return ""
		}
		err := yaml.Unmarshal(body, &q)
		if err != nil {
			e.Log.WithError(err).Warnf("Error decoding request body as %q.", contentType)
			return ""
		}

	case "application/json":
		if len(body) == 0 {
			e.Log.WithField("content-type", contentType).Infof("Empty request body.")
			return ""
		}
		err := json.Unmarshal(body, &q)
		if err != nil {
			e.Log.WithError(err).Warnf("Error decoding request body as %q.", contentType)
			return ""
		}

	default:
		e.Log.Warnf("Unknown or missing 'Content-Type': %q, assuming 'application/json'.", contentType)
		if len(body) == 0 {
			e.Log.WithField("content-type", contentType).Infof("Empty request body.")
			return ""
		}

		err := json.Unmarshal(body, &q)
		if err != nil {
			e.Log.WithError(err).Warnf("Error decoding request body as %q.", contentType)
			return ""
		}
	}

	result := q.Query
	if result == nil {
		result = q.Knn
	}

	if result == nil {
		return ""
	}

	switch qt := result.(type) {
	case string:
		return qt
	default:
		marshal, err := json.Marshal(result)
		if err != nil {
			e.Log.WithError(err).Warnf("Error encoding query to json; body: %x, content type: %v.", body, contentType)
			return ""
		}
		return string(marshal)
	}
}

// emitAuditEvent writes the request and response to audit stream.
func (e *Engine) emitAuditEvent(req *http.Request, body []byte) {
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
		query = e.getQueryFromRequestBody(contentType, body)
	}

	ev := &apievents.ElasticsearchRequest{
		Metadata: common.MakeEventMetadata(e.sessionCtx,
			events.DatabaseSessionElasticsearchRequestEvent,
			events.ElasticsearchRequestCode),
		UserMetadata:     common.MakeUserMetadata(e.sessionCtx),
		SessionMetadata:  common.MakeSessionMetadata(e.sessionCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(e.sessionCtx),
		Method:           req.Method,
		Path:             req.URL.Path,
		RawQuery:         req.URL.RawQuery,
		Body:             body,
		Headers:          wrappers.Traits(req.Header),

		Category: category,
		Target:   target,
		Query:    query,
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
	dbRoleMatchers := role.DatabaseRoleMatchers(
		e.sessionCtx.Database,
		e.sessionCtx.DatabaseUser,
		e.sessionCtx.DatabaseName,
	)
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}
