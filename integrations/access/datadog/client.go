/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package datadog

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/trace"
)

const (
	datadogMaxConns    = 100
	datadogHTTPTimeout = 10 * time.Second
	statusEmitTimeout  = 10 * time.Second
)

var incidentSummaryTemplate = template.Must(template.New("incident summary").Parse(
	`You have a new Access Request:

ID: {{.ID}}
Cluster: {{.ClusterName}}
User: {{.User}}
Role(s): {{range $index, $element := .Roles}}{{if $index}}, {{end}}{{ . }}{{end}}
{{if .RequestLink}}Link: {{.RequestLink}}{{end}} `,
))
var reviewNoteTemplate = template.Must(template.New("review note").Parse(
	`{{.Author}} reviewed the request.
Resolution: {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))
var resolutionNoteTemplate = template.Must(template.New("resolution note").Parse(
	`Access request has been {{.Resolution}}
{{if .ResolveReason}}Reason: {{.ResolveReason}}{{end}}`,
))

type Datadog struct {
	client *resty.Client
	// TODO: Remove once on-call API is migrated to official API
	clientUnstable *resty.Client

	webProxyURL *url.URL
	severity    string
}

func NewDatadogClient(conf DatadogConfig, webProxyAddr string, statusSink common.StatusSink) (*Datadog, error) {
	var (
		webProxyURL *url.URL
		err         error
	)

	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	client := resty.NewWithClient(&http.Client{
		Timeout: datadogHTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     datadogMaxConns,
			MaxIdleConnsPerHost: datadogMaxConns,
		}}).
		SetBaseURL(conf.APIEndpoint).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("DD-API-KEY", conf.APIKey).
		SetHeader("DD-APPLICATION-KEY", conf.ApplicationKey).
		OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
			req.SetError(&ErrorResult{})
			return nil
		})

	clientUnstable := resty.NewWithClient(&http.Client{
		Timeout: datadogHTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     datadogMaxConns,
			MaxIdleConnsPerHost: datadogMaxConns,
		}}).
		SetBaseURL(APIUnstableEndpointURL).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("DD-API-KEY", conf.APIKey).
		SetHeader("DD-APPLICATION-KEY", conf.ApplicationKey).
		OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
			req.SetError(&ErrorResult{})
			return nil
		})

	if statusSink != nil {
		client.OnAfterResponse(onAfterDatadogResponse(statusSink))
	}

	return &Datadog{
		client:         client,
		clientUnstable: clientUnstable,
		webProxyURL:    webProxyURL,
		severity:       conf.Severity,
	}, nil
}

func onAfterDatadogResponse(sink common.StatusSink) resty.ResponseMiddleware {
	return func(_ *resty.Client, resp *resty.Response) error {
		log := logger.Get(resp.Request.Context())
		status := common.StatusFromStatusCode(resp.StatusCode())

		// No usable context in scope, use background with a reasonable timeout
		ctx, cancel := context.WithTimeout(context.Background(), statusEmitTimeout)
		defer cancel()

		if err := sink.Emit(ctx, status); err != nil {
			log.WithError(err).Errorf("Error while emitting Datadog plugin status: %v", err)
		}

		if resp.IsError() {
			var details string
			switch result := resp.Error().(type) {
			case *ErrorResult:
				// Do we have a formatted Datadog API error response? We set
				// an empty `ErrorResult` in the pre-request hook, and if the
				// HTTP server returns an error, the `resty` middleware will
				// attempt to unmarshal the error response into it.
				details = fmt.Sprintf("http error code=%v, err_code=%v, message=%v, errors=[%v]", resp.StatusCode(), result.Code, result.Message, strings.Join(result.Errors, ", "))
			default:
				details = fmt.Sprintf("unknown error result %#v", result)
			}

			if status.GetCode() == types.PluginStatusCode_UNAUTHORIZED {
				return trace.AccessDenied(details)
			}
			return trace.Errorf(details)
		}
		return nil
	}
}

func (d *Datadog) CheckHealth(ctx context.Context) error {
	var result PermissionsBody
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		Get("permissions")
	if err != nil {
		return trace.Wrap(err)
	}
	for _, permission := range result.Data {
		if permission.Attributes.Name == "incident_write" && !permission.Attributes.Restricted {
			return nil
		}
	}
	return trace.AccessDenied("missing incident_write permissions")
}

func (d *Datadog) CreateIncident(ctx context.Context, clusterName, reqID string, recipients []common.Recipient, reqData pd.AccessRequestData) (IncidentData, error) {
	teams := make([]string, 0, len(recipients))
	services := make([]string, 0, len(recipients))

	for _, recipient := range recipients {
		switch recipient.Kind {
		case common.RecipientKindService:
			services = append(services, recipient.Name)
		case common.RecipientKindTeam:
			teams = append(teams, recipient.Name)
		}
	}

	summary, err := d.buildIncidentSummary(clusterName, reqID, reqData)
	if err != nil {
		return IncidentData{}, trace.Wrap(err)
	}

	body := IncidentBody{
		Data: IncidentData{
			Type: "incidents",
			Attributes: IncidentAttributes{
				Title:            fmt.Sprintf("Access request from %s", reqData.User),
				CustomerImpacted: false,
				Fields: IncidentFields{
					Summary: &StringField{
						Type:  "textbox",
						Value: summary,
					},
					State: &StringField{
						Type:  "dropdown",
						Value: "active",
					},
					DetectionMethod: &StringField{
						Type:  "dropdown",
						Value: "employee",
					},
					Severity: &StringField{
						Type:  "dropdown",
						Value: d.severity,
					},
					RootCause: &StringField{
						Type:  "textbox",
						Value: reqData.RequestReason,
					},
					Teams: &StringSliceField{
						Type:  "multiselect",
						Value: teams,
					},
					Services: &StringSliceField{
						Type:  "autocomplete",
						Value: services,
					},
				},
			},
		},
	}
	var result IncidentBody
	_, err = d.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetResult(&result).
		Post("incidents")
	if err != nil {
		return IncidentData{}, trace.Wrap(err)
	}

	return result.Data, nil
}

// PostReviewNote posts a note once a new request review appears.
func (d *Datadog) PostReviewNote(ctx context.Context, incidentID string, review types.AccessReview) error {
	noteContent, err := d.buildReviewNoteBody(review)
	if err != nil {
		return trace.Wrap(err)
	}
	body := TimelineBody{
		Data: TimelineData{
			Type: "incident_timeline_cells",
			Attributes: TimelineAttributes{
				CellType: "markdown",
				Content: TimelineContent{
					Content: noteContent,
				},
			},
		},
	}

	_, err = d.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParam("incident_id", incidentID).
		Post("incidents/{incident_id}/timeline")
	return trace.Wrap(err)
}

// ResolveIncident resolves an incident and posts a note with resolution details.
func (d *Datadog) ResolveIncident(ctx context.Context, incidentID string, reqData pd.AccessRequestData, reviews []types.AccessReview) error {
	noteContent, err := d.buildResolutionNoteBody(reqData)
	if err != nil {
		return trace.Wrap(err)
	}

	timelineBody := TimelineBody{
		Data: TimelineData{
			Type: "incident_timeline_cells",
			Attributes: TimelineAttributes{
				CellType: "markdown",
				Content: TimelineContent{
					Content: noteContent,
				},
			},
		},
	}

	_, err = d.client.NewRequest().
		SetContext(ctx).
		SetBody(timelineBody).
		SetPathParam("incident_id", incidentID).
		Post("incidents/{incident_id}/timeline")
	if err != nil {
		return trace.Wrap(err)
	}

	incidentBody := IncidentBody{
		Data: IncidentData{
			ID:   incidentID,
			Type: "incidents",
			Attributes: IncidentAttributes{
				Fields: IncidentFields{
					State: &StringField{
						Type:  "dropdown",
						Value: "resolved",
					},
				},
			},
		},
	}
	_, err = d.client.NewRequest().
		SetContext(ctx).
		SetBody(incidentBody).
		SetPathParam("incident_id", incidentID).
		Patch("incidents/{incident_id}")
	return trace.Wrap(err)
}

func (d *Datadog) GetOncallTeams(ctx context.Context) (OncallTeamsBody, error) {
	var result OncallTeamsBody
	resp, err := d.clientUnstable.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		Get("on-call/teams")

	logger.Get(ctx).WithField("result", result).WithField("resp", resp.String()).Info("GetOncallTeams")
	return result, trace.Wrap(err)
}

func (d *Datadog) FindUserByEmail(ctx context.Context, userEmail string) (UserData, error) {
	var result UsersBody
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetQueryParam("filter", userEmail).
		SetResult(&result).
		Get("users")
	if err != nil {
		return UserData{}, trace.Wrap(err)
	}
	for _, user := range result.Data {
		if !user.Attributes.Disabled && user.Attributes.Email == userEmail {
			return user, nil
		}
	}
	return UserData{}, trace.NotFound("could not find user with email %q", userEmail)
}

func (d *Datadog) buildIncidentSummary(clusterName, reqID string, reqData pd.AccessRequestData) (string, error) {
	var requestLink string
	if d.webProxyURL != nil {
		reqURL := *d.webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	err := incidentSummaryTemplate.Execute(&builder, struct {
		ID          string
		ClusterName string
		RequestLink string
		pd.AccessRequestData
	}{
		reqID,
		clusterName,
		requestLink,
		reqData,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func (d *Datadog) buildReviewNoteBody(review types.AccessReview) (string, error) {
	var builder strings.Builder
	err := reviewNoteTemplate.Execute(&builder, struct {
		Author        string
		ProposedState string
		Reason        string
	}{
		review.Author,
		review.ProposedState.String(),
		review.Reason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func (d *Datadog) buildResolutionNoteBody(reqData pd.AccessRequestData) (string, error) {
	var builder strings.Builder
	err := resolutionNoteTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		statusText(reqData.ResolutionTag),
		reqData.ResolutionReason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func statusText(tag pd.ResolutionTag) string {
	var statusEmoji string
	switch tag {
	case pd.Unresolved:
		statusEmoji = "⏳"
	case pd.ResolvedApproved:
		statusEmoji = "✅"
	case pd.ResolvedDenied:
		statusEmoji = "❌"
	case pd.ResolvedExpired:
		statusEmoji = "⌛"
	}
	return fmt.Sprintf("%s %s", statusEmoji, string(tag))
}
