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

package opsgenie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// alertKeyPrefix is the prefix for Alert's alias field used when creating an Alert.
	alertKeyPrefix        = "teleport-access-request"
	heartbeatName         = "teleport-access-heartbeat"
	ResponderTypeSchedule = "schedule"
	ResponderTypeUser     = "user"
	ResponderTypeTeam     = "team"

	ResolveAlertRequestRetryInterval = time.Second * 10
	ResolveAlertRequestRetryTimeout  = time.Minute * 2
)

var alertBodyTemplate = template.Must(template.New("alert body").Parse(
	`{{.User}} requested permissions for roles {{range $index, $element := .Roles}}{{if $index}}, {{end}}{{ . }}{{end}} on Teleport at {{.Created.Format .TimeFormat}}.
{{if .RequestReason}}Reason: {{.RequestReason}}{{end}}
{{if .RequestLink}}To approve or deny the request, proceed to {{.RequestLink}}{{end}}
`,
))
var reviewNoteTemplate = template.Must(template.New("review note").Parse(
	`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))
var resolutionNoteTemplate = template.Must(template.New("resolution note").Parse(
	`Access request has been {{.Resolution}}
{{if .ResolveReason}}Reason: {{.ResolveReason}}{{end}}`,
))

// Client is a wrapper around resty.Client.
type Client struct {
	ClientConfig

	client *resty.Client
}

// ClientConfig is the config for the opsgenie client.
type ClientConfig struct {
	// APIKey is the API key for Opsgenie
	APIKey string
	// APIEndpoint is the endpoint for the Opsgenie API
	// api url of the form https://api.opsgenie.com/v2/ with optional trailing '/'
	APIEndpoint string
	// DefaultSchedules are the default on-call schedules to check for auto approval
	DefaultSchedules []string
	// DefaultTeams are the default Opsgenie Teams to add as responders
	DefaultTeams []string
	// Priority is the priority alerts are to be created with
	Priority string

	// WebProxyURL is the Teleport address used when building the bodies of the alerts
	// allowing links to the access requests to be built
	WebProxyURL *url.URL
	// ClusterName is the name of the Teleport cluster
	ClusterName string

	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink
}

func (cfg *ClientConfig) CheckAndSetDefaults() error {
	if cfg.APIKey == "" {
		return trace.BadParameter("missing required value APIKey")
	}
	if cfg.APIEndpoint == "" {
		return trace.BadParameter("missing required value APIEndpoint")
	}
	if cfg.WebProxyURL == nil {
		return trace.BadParameter("missing required value WebProxyURL")
	}
	return nil
}

// NewClient creates a new Opsgenie client for managing alerts.
func NewClient(conf ClientConfig) (*Client, error) {
	client := resty.NewWithClient(defaults.Config().HTTPClient)
	client.SetTransport(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
	})
	client.SetHeader("Authorization", "GenieKey "+conf.APIKey)
	client.SetBaseURL(conf.APIEndpoint)
	return &Client{
		client:       client,
		ClientConfig: conf,
	}, nil
}

func errWrapper(statusCode int, body string) error {
	switch statusCode {
	case http.StatusForbidden:
		return trace.AccessDenied("opsgenie API access denied: status code %v: %q", statusCode, body)
	case http.StatusRequestTimeout:
		return trace.ConnectionProblem(trace.Errorf("status code %v: %q", statusCode, body),
			"connecting to opsgenie API")
	}
	return trace.Errorf("connecting to opsgenie API status code %v: %q", statusCode, body)
}

// CreateAlert creates an opsgenie alert.
func (og Client) CreateAlert(ctx context.Context, reqID string, reqData RequestData) (OpsgenieData, error) {
	bodyDetails, err := buildAlertBody(og.WebProxyURL, reqID, reqData)
	if err != nil {
		return OpsgenieData{}, trace.Wrap(err)
	}

	body := AlertBody{
		Message:     fmt.Sprintf("Access request from %s", reqData.User),
		Alias:       fmt.Sprintf("%s/%s", alertKeyPrefix, reqID),
		Description: bodyDetails,
		Responders:  og.getResponders(reqData),
		Priority:    og.Priority,
	}

	var result CreateAlertResult
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetResult(&result).
		Post("v2/alerts")

	if err != nil {
		return OpsgenieData{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return OpsgenieData{}, errWrapper(resp.StatusCode(), string(resp.Body()))
	}

	// If this fails, Teleport request approval and auto-approval will still work,
	// but incident in Opsgenie won't be auto-closed or updated as the alertID won't be available.
	alertRequestResult, err := og.tryGetAlertRequestResult(ctx, result.RequestID)
	if err != nil {
		return OpsgenieData{}, trace.Wrap(err)
	}

	return OpsgenieData{
		AlertID: alertRequestResult.Data.AlertID,
	}, nil
}

func (og Client) tryGetAlertRequestResult(ctx context.Context, reqID string) (GetAlertRequestResult, error) {
	backoff := backoff.NewDecorr(ResolveAlertRequestRetryInterval, ResolveAlertRequestRetryTimeout, clockwork.NewRealClock())
	for {
		alertRequestResult, err := og.getAlertRequestResult(ctx, reqID)
		if err == nil {
			logger.Get(ctx).DebugContext(ctx, "Got alert request result", "alert_id", alertRequestResult.Data.AlertID)
			return alertRequestResult, nil
		}
		logger.Get(ctx).DebugContext(ctx, "Failed to get alert request result", "error", err)
		if err := backoff.Do(ctx); err != nil {
			return GetAlertRequestResult{}, trace.Wrap(err)
		}
	}
}

func (og Client) getAlertRequestResult(ctx context.Context, reqID string) (GetAlertRequestResult, error) {
	var result GetAlertRequestResult
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		SetPathParams(map[string]string{"requestID": reqID}).
		Get("v2/alerts/requests/{requestID}")
	if err != nil {
		return GetAlertRequestResult{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return GetAlertRequestResult{}, errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return result, nil
}

func (og Client) getResponders(reqData RequestData) []Responder {
	schedules := og.DefaultSchedules
	if reqSchedules, ok := reqData.SystemAnnotations[types.TeleportNamespace+types.ReqAnnotationNotifySchedulesLabel]; ok {
		schedules = reqSchedules
	}
	teams := og.DefaultTeams
	if reqTeams, ok := reqData.SystemAnnotations[types.TeleportNamespace+types.ReqAnnotationTeamsLabel]; ok {
		teams = reqTeams
	}
	responders := make([]Responder, 0, len(schedules)+len(teams))
	for _, s := range schedules {
		responders = append(responders, createResponder(ResponderTypeSchedule, s))
	}
	for _, t := range teams {
		responders = append(responders, createResponder(ResponderTypeTeam, t))
	}
	return responders
}

// Check if the responder is a UUID. If it is, then it is an ID; otherwise, it is a name.
func createResponder(responderType string, value string) Responder {
	if _, err := uuid.Parse(value); err == nil {
		return Responder{
			Type: responderType,
			ID:   value,
		}
	}
	return Responder{
		Type: responderType,
		Name: value,
	}
}

// PostReviewNote posts a note once a new request review appears.
func (og Client) PostReviewNote(ctx context.Context, alertID string, review types.AccessReview) error {
	note, err := buildReviewNoteBody(review)
	if err != nil {
		return trace.Wrap(err)
	}
	body := AlertNote{
		Note: note,
	}
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParams(map[string]string{"alertID": alertID}).
		SetQueryParams(map[string]string{"identifierType": "id"}).
		Post("v2/alerts/{alertID}/notes")

	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return nil
}

// ResolveAlert resolves an alert and posts a note with resolution details.
func (og Client) ResolveAlert(ctx context.Context, alertID string, resolution Resolution) error {
	note, err := buildResolutionNoteBody(resolution)
	if err != nil {
		return trace.Wrap(err)
	}
	body := AlertNote{
		Note: note,
	}
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParams(map[string]string{"alertID": alertID}).
		SetQueryParams(map[string]string{"identifierType": "id"}).
		Post("v2/alerts/{alertID}/close")
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return nil
}

// GetOnCall returns the list of responders on-call for a schedule.
func (og Client) GetOnCall(ctx context.Context, scheduleName string) (RespondersResult, error) {
	var result RespondersResult
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"scheduleName": scheduleName}).
		SetQueryParams(map[string]string{
			// This is required to lookup schedules by name (as opposed to lookup by ID)
			"scheduleIdentifierType": "name",
			// When flat is enabled it returns the email addresses of on-call participants.
			"flat": "true",
		}).
		SetResult(&result).
		Get("v2/schedules/{scheduleName}/on-calls")
	if err != nil {
		return RespondersResult{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return RespondersResult{}, errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return result, nil
}

// CheckHealth pings opsgenie.
func (og Client) CheckHealth(ctx context.Context) error {
	// The heartbeat pings will respond even if the heartbeat does not exist.
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"heartbeat": heartbeatName}).
		Get("v2/heartbeats/teleport-access-heartbeat/ping")

	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()

	if og.StatusSink != nil {
		var code types.PluginStatusCode
		switch {
		case resp.StatusCode() == http.StatusUnauthorized:
			code = types.PluginStatusCode_UNAUTHORIZED
		case resp.StatusCode() >= 200 && resp.StatusCode() < 400:
			code = types.PluginStatusCode_RUNNING
		default:
			code = types.PluginStatusCode_OTHER_ERROR
		}
		if err := og.StatusSink.Emit(ctx, &types.PluginStatusV1{Code: code}); err != nil {
			logger.Get(resp.Request.Context()).ErrorContext(ctx, "Error while emitting servicenow plugin status",
				"error", err,
				"code", resp.StatusCode(),
			)
		}
	}

	if resp.IsError() {
		return errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return nil
}

func buildAlertBody(webProxyURL *url.URL, reqID string, reqData RequestData) (string, error) {
	var requestLink string
	if webProxyURL != nil {
		reqURL := *webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	err := alertBodyTemplate.Execute(&builder, struct {
		ID          string
		TimeFormat  string
		RequestLink string
		RequestData
	}{
		ID:          reqID,
		TimeFormat:  time.RFC822,
		RequestLink: requestLink,
		RequestData: reqData,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func buildReviewNoteBody(review types.AccessReview) (string, error) {
	var builder strings.Builder
	err := reviewNoteTemplate.Execute(&builder, struct {
		types.AccessReview
		ProposedState string
		TimeFormat    string
	}{
		review,
		review.ProposedState.String(),
		time.RFC822,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func buildResolutionNoteBody(resolution Resolution) (string, error) {
	var builder strings.Builder
	err := resolutionNoteTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		Resolution:    string(resolution.Tag),
		ResolveReason: resolution.Reason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}
