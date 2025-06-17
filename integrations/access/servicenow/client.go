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

package servicenow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
)

const (
	// DateTimeFormat is the time format used by servicenow
	DateTimeFormat = "2006-01-02 15:04:05"
)

type ServiceNowClient interface {
	// CreateIncident creates an servicenow incident.
	CreateIncident(ctx context.Context, reqID string, reqData RequestData) (Incident, error)
	// PostReviewNote posts a note once a new request review appears.
	PostReviewNote(ctx context.Context, incidentID string, review types.AccessReview) error
	// ResolveIncident resolves an incident and posts a note with resolution details.
	ResolveIncident(ctx context.Context, incidentID string, resolution Resolution) error
	// GetOnCall returns the current users on-call for the given rota ID.
	GetOnCall(ctx context.Context, rotaID string) ([]string, error)
	// GetUserName returns the name for the given user ID
	GetUserName(ctx context.Context, userID string) (string, error)
	// CheckHealth pings servicenow to check if it is reachable.
	CheckHealth(ctx context.Context) error
}

// Client is a wrapper around resty.Client that implements a few ServiceNow
// incident methods to create incidents, update them, and check who is on-call.
//
// The on_call_rota API is not publicly documented, but you can access
// swagger-like interface by registering a dev SNow account and requesting a dev
// instance.
// When the dev instance is created, you can open the "ALL" tab and search for
// the REST API explorer.
type Client struct {
	ClientConfig

	client *resty.Client
}

// ClientConfig is the config for the servicenow client.
type ClientConfig struct {
	// APIEndpoint is the endpoint for the Servicenow API
	// api url of the form  https://instance.service-now.com/ with optional trailing '/'
	APIEndpoint string

	// WebProxyURL is the Teleport address used when building the bodies of the incidents
	// allowing links to the access requests to be built
	WebProxyURL *url.URL

	// ClusterName is the name of the Teleport cluster.
	ClusterName string

	// Username is the username used by the client for basic auth.
	Username string
	// APIToken is the token used for basic auth.
	APIToken string
	// CloseCode is the ServiceNow close code that incidents will be closed with.
	CloseCode string

	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink
}

// NewClient creates a new ServiceNow client for managing incidents.
func NewClient(conf ClientConfig) (*Client, error) {
	if err := conf.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	apiURL, err := url.Parse(conf.APIEndpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if apiURL.Scheme == "http" && apiURL.Hostname() != "127.0.0.1" {
		return nil, trace.BadParameter("http scheme is only permitted for localhost: %v", apiURL.Host)
	}
	if apiURL.Hostname() != "127.0.0.1" {
		apiURL.Scheme = "https"
	}

	const (
		maxConns      = 100
		clientTimeout = 10 * time.Second
	)

	client := resty.NewWithClient(&http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     maxConns,
			MaxIdleConnsPerHost: maxConns,
			Proxy:               http.ProxyFromEnvironment,
		}}).
		SetBaseURL(apiURL.String()).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBasicAuth(conf.Username, conf.APIToken)
	client.OnAfterResponse(common.OnAfterResponse(types.PluginTypeServiceNow, errWrapper, conf.StatusSink))
	return &Client{
		client:       client,
		ClientConfig: conf,
	}, nil
}

func (conf ClientConfig) checkAndSetDefaults() error {
	if conf.APIEndpoint == "" {
		return trace.BadParameter("missing required field: APIEndpoint")
	}
	return nil
}

func errWrapper(statusCode int, body []byte) error {
	defaultMessage := string(body)
	errResponse := errorResult{}
	if err := json.Unmarshal(body, &errResponse); err == nil {
		defaultMessage = errResponse.Error.Message
	}

	switch statusCode {
	case http.StatusForbidden:
		return trace.AccessDenied("servicenow API access denied: status code %v: %q", statusCode, defaultMessage)
	case http.StatusRequestTimeout:
		return trace.ConnectionProblem(nil, "request to servicenow API failed: status code %v: %q", statusCode, defaultMessage)
	}
	return trace.Errorf("request to servicenow API failed: status code %d: %q", statusCode, defaultMessage)
}

// CreateIncident creates an servicenow incident.
func (snc *Client) CreateIncident(ctx context.Context, reqID string, reqData RequestData) (Incident, error) {
	bodyDetails, err := buildIncidentBody(snc.WebProxyURL, reqID, reqData, snc.ClusterName)
	if err != nil {
		return Incident{}, trace.Wrap(err)
	}

	body := Incident{
		ShortDescription: fmt.Sprintf("Teleport access request from user %s", reqData.User),
		Description:      bodyDetails,
		Caller:           reqData.User,
	}

	if len(reqData.SuggestedReviewers) != 0 {
		// Only one assignee per incident allowed so just grab the first.
		body.AssignedTo = reqData.SuggestedReviewers[0]
	}

	var result IncidentResult
	resp, err := snc.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetResult(&result).
		Post("/api/now/v1/table/incident")
	if err != nil {
		return Incident{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()

	return Incident{IncidentID: result.Result.IncidentID}, nil
}

// PostReviewNote posts a note once a new request review appears.
func (snc *Client) PostReviewNote(ctx context.Context, incidentID string, review types.AccessReview) error {
	note, err := buildReviewNoteBody(review)
	if err != nil {
		return trace.Wrap(err)
	}
	body := Incident{
		WorkNotes: note,
	}
	resp, err := snc.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParams(map[string]string{"sys_id": incidentID}).
		Patch("/api/now/v1/table/incident/{sys_id}")
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	return nil
}

// ResolveIncident resolves an incident and posts a note with resolution details.
func (snc *Client) ResolveIncident(ctx context.Context, incidentID string, resolution Resolution) error {
	note, err := buildResolutionNoteBody(resolution, snc.CloseCode)
	if err != nil {
		return trace.Wrap(err)
	}
	body := Incident{
		CloseCode:     snc.CloseCode,
		IncidentState: resolution.State,
		CloseNotes:    note,
	}
	resp, err := snc.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParams(map[string]string{"sys_id": incidentID}).
		Patch("/api/now/v1/table/incident/{sys_id}")
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	return nil
}

// GetOnCall returns the current users on-call for the given rota ID.
func (snc *Client) GetOnCall(ctx context.Context, rotaID string) ([]string, error) {
	formattedTime := time.Now().Format(DateTimeFormat)
	var result OnCallResult
	resp, err := snc.client.NewRequest().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"rota_ids":  rotaID,
			"date_time": formattedTime,
		}).
		SetResult(&result).
		Get("/api/now/on_call_rota/whoisoncall")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if len(result.Result) == 0 {
		return nil, trace.NotFound("no user found for given rota: %q", rotaID)
	}
	var userNames []string
	for _, result := range result.Result {
		userName, err := snc.GetUserName(ctx, result.UserID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userNames = append(userNames, userName)
	}
	return userNames, nil
}

// CheckHealth pings servicenow to check if it is reachable.
func (snc *Client) CheckHealth(ctx context.Context) error {
	resp, err := snc.client.NewRequest().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"sysparm_limit": "1",
		}).
		Get("/api/now/table/incident")
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	return nil
}

// GetUserName returns the name for the given user ID
func (snc *Client) GetUserName(ctx context.Context, userID string) (string, error) {
	var result UserResult
	resp, err := snc.client.NewRequest().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"sysparm_fields": "user_name",
		}).
		SetPathParams(map[string]string{"user_id": userID}).
		SetResult(&result).
		Get("/api/now/table/sys_user/{user_id}")
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if result.Result.UserName == "" {
		return "", trace.NotFound("no username found for given id: %v", userID)
	}
	return result.Result.UserName, nil
}

var (
	incidentWithRolesBodyTemplate = template.Must(template.New("incident body").Parse(
		`Teleport user {{.User}} submitted access request for roles {{range $index, $element := .Roles}}{{if $index}}, {{end}}{{ . }}{{end}} on Teleport cluster {{.ClusterName}}.
{{if .RequestReason}}Reason: {{.RequestReason}}{{end}}
{{if .RequestLink}}Click this link to review the request in Teleport: {{.RequestLink}}{{end}}
`,
	))
	incidentBodyTemplate = template.Must(template.New("incident body").Parse(
		`Teleport user {{.User}} submitted access request on Teleport cluster {{.ClusterName}}.
{{if .RequestReason}}Reason: {{.RequestReason}}{{end}}
{{if .RequestLink}}Click this link to review the request in Teleport: {{.RequestLink}}{{end}}
`,
	))
	reviewNoteTemplate = template.Must(template.New("review note").Parse(
		`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
	))
	resolutionNoteTemplate = template.Must(template.New("resolution note").Parse(
		`Access request has been {{.Resolution}}
{{if .ResolveReason}}Reason: {{.ResolveReason}}{{end}}`,
	))
)

func buildIncidentBody(webProxyURL *url.URL, reqID string, reqData RequestData, clusterName string) (string, error) {
	var requestLink string
	if webProxyURL != nil {
		reqURL := *webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	template := incidentBodyTemplate
	if reqData.Resources == nil {
		template = incidentWithRolesBodyTemplate
	}
	err := template.Execute(&builder, struct {
		ID          string
		TimeFormat  string
		RequestLink string
		ClusterName string
		RequestData
	}{
		ID:          reqID,
		TimeFormat:  time.RFC822,
		RequestLink: requestLink,
		ClusterName: clusterName,
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

func buildResolutionNoteBody(resolution Resolution, closeCode string) (string, error) {
	var builder strings.Builder
	err := resolutionNoteTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		Resolution:    closeCode,
		ResolveReason: resolution.Reason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}
