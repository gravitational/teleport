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

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// as per
	// https://developer.atlassian.com/cloud/jira/platform/webhooks/#known-issues,
	// the webhook payload size is limited to 25MB
	jiraWebhookPayloadLimit = 25 * 1024 * 1024

	DefaultDir = "/var/lib/teleport/plugins/jira"
)

type WebhookIssue struct {
	ID     string      `json:"id"`
	Self   string      `json:"self"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

type Webhook struct {
	Timestamp          int    `json:"timestamp"`
	WebhookEvent       string `json:"webhookEvent"`
	IssueEventTypeName string `json:"issue_event_type_name"`
	User               *struct {
		Self        string `json:"self"`
		AccountID   string `json:"accountId"`
		AccountType string `json:"accountType"`
		DisplayName string `json:"displayName"`
		Active      bool   `json:"active"`
	} `json:"user"`
	Issue *WebhookIssue `json:"issue"`
}
type WebhookFunc func(ctx context.Context, webhook Webhook) error

// WebhookServer is a wrapper around http.Server that processes Jira webhook events.
// It verifies incoming requests and calls onWebhook for valid ones
type WebhookServer struct {
	http      *lib.HTTP
	onWebhook WebhookFunc
	counter   uint64
}

func NewWebhookServer(conf lib.HTTPConfig, onWebhook WebhookFunc) (*WebhookServer, error) {
	httpSrv, err := lib.NewHTTP(conf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv := &WebhookServer{
		http:      httpSrv,
		onWebhook: onWebhook,
	}
	httpSrv.POST("/", srv.processWebhook)
	httpSrv.GET("/status", srv.processStatus)
	return srv, nil
}

func (s *WebhookServer) ServiceJob() lib.ServiceJob {
	return s.http.ServiceJob()
}

func (s *WebhookServer) BaseURL() *url.URL {
	return s.http.BaseURL()
}

func (s *WebhookServer) EnsureCert() error {
	return s.http.EnsureCert(DefaultDir + "/server")
}

func (s *WebhookServer) processWebhook(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Millisecond*2500)
	defer cancel()

	httpRequestID := fmt.Sprintf("%v-%v", time.Now().Unix(), atomic.AddUint64(&s.counter, 1))
	ctx, log := logger.With(ctx, "jira_http_id", httpRequestID)

	var webhook Webhook

	body, err := io.ReadAll(io.LimitReader(r.Body, jiraWebhookPayloadLimit+1))
	if err != nil {
		log.ErrorContext(ctx, "Failed to read webhook payload", "error", err)
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
	if len(body) > jiraWebhookPayloadLimit {
		log.ErrorContext(ctx, "Received a webhook with a payload that exceeded the limit",
			"payload_size", len(body),
			"payload_size_limit", jiraWebhookPayloadLimit,
		)
		http.Error(rw, "", http.StatusRequestEntityTooLarge)
	}
	if err = json.Unmarshal(body, &webhook); err != nil {
		log.ErrorContext(ctx, "Failed to parse webhook payload", "error", err)
		http.Error(rw, "", http.StatusBadRequest)
		return
	}

	if err = s.onWebhook(ctx, webhook); err != nil {
		log.ErrorContext(ctx, "Failed to process webhook", "error", err)
		var code int
		switch {
		case lib.IsCanceled(err) || lib.IsDeadline(err):
			code = http.StatusServiceUnavailable
		default:
			code = http.StatusInternalServerError
		}
		http.Error(rw, "", code)
	} else {
		rw.WriteHeader(http.StatusOK)
	}
}

func (s *WebhookServer) processStatus(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	rw.WriteHeader(http.StatusOK)
}
