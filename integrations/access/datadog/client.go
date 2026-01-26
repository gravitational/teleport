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
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const (
	datadogMaxConns    = 100
	datadogHTTPTimeout = 10 * time.Second
	statusEmitTimeout  = 10 * time.Second
)

const (
	// IncidentWritePermissions is a Datadog permission that allows the role to
	// create, view, and manage incidents in Datadog.
	//
	// See documentation for more details:
	// https://docs.datadoghq.com/account_management/rbac/permissions/#case-and-incident-management
	IncidentWritePermissions = "incident_write"

	// TeamsManagePermissions is a Datadog permission that allows the role to
	// create, view, and maage teams in Datadog.
	//
	// See documentation for more details:
	// https://docs.datadoghq.com/account_management/rbac/permissions/#teams
	TeamsManagePermissions = "teams_manage"

	// OnCallReadPermissions is a Datadog permission that allows the role to
	// view on-call teams, schedules, escalation policies and overrides.
	//
	// See documentation for more details:
	// https://docs.datadoghq.com/account_management/rbac/permissions/#on-call
	OnCallReadPermissions = "on_call_read"
)

// Datadog is a wrapper around resty.Client.
type Datadog struct {
	// DatadogConfig specifies datadog client configuration.
	DatadogConfig

	// TODO: Datadog API client implemented using resty because implementation is
	// simpler to integrate with the existing framework. Consider using the official
	// datadog api client package: https://github.com/DataDog/datadog-api-client-go.
	client *resty.Client
}

// NewDatadogClient creates a new Datadog client for managing incidents.
func NewDatadogClient(conf DatadogConfig, webProxyAddr string, statusSink common.StatusSink) (*Datadog, error) {
	apiEndpoint, err := url.JoinPath(conf.APIEndpoint, APIVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := resty.NewWithClient(&http.Client{
		Timeout: datadogHTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     datadogMaxConns,
			MaxIdleConnsPerHost: datadogMaxConns,
			Proxy:               http.ProxyFromEnvironment,
		}}).
		SetBaseURL(apiEndpoint).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("DD-API-KEY", conf.APIKey).
		SetHeader("DD-APPLICATION-KEY", conf.ApplicationKey).
		OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
			req.SetError(&ErrorResult{})
			return nil
		}).
		OnAfterResponse(onAfterDatadogResponse(statusSink))

	return &Datadog{
		DatadogConfig: conf,
		client:        client,
	}, nil
}

func onAfterDatadogResponse(sink common.StatusSink) resty.ResponseMiddleware {
	return func(_ *resty.Client, resp *resty.Response) error {
		log := logger.Get(resp.Request.Context())

		if sink != nil {
			status := common.StatusFromStatusCode(resp.StatusCode())
			// No usable context in scope, use background with a reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), statusEmitTimeout)
			defer cancel()

			if err := sink.Emit(ctx, status); err != nil {
				log.ErrorContext(ctx, "Error while emitting Datadog Incident Management plugin status", "error", err)
			}
		}

		if resp.IsError() {
			switch result := resp.Error().(type) {
			case *ErrorResult:
				return trace.Errorf("http error code=%v, errors=[%v]", resp.StatusCode(), strings.Join(result.Errors, ", "))
			default:
				return trace.Errorf("unknown error result %#v", result)
			}
		}
		return nil
	}
}

// CheckHealth pings Datadog and ensures required permissions.
//
// See: https://docs.datadoghq.com/api/latest/roles/#list-permissions
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
		switch name := permission.Attributes.Name; name {
		case IncidentWritePermissions, TeamsManagePermissions, OnCallReadPermissions:
			if permission.Attributes.Restricted {
				return trace.AccessDenied("missing %s permissions", name)
			}
		}
	}
	return nil
}

// Create Incident creates a new Datadog incident.
//
// See: https://docs.datadoghq.com/api/latest/incidents/#create-an-incident
func (d *Datadog) CreateIncident(ctx context.Context, summary string, recipients []common.Recipient, reqData pd.AccessRequestData) (IncidentsData, error) {
	teams := make([]string, 0, len(recipients))
	emails := make([]NotificationHandle, 0, len(recipients))

	for _, recipient := range recipients {
		switch recipient.Kind {
		case common.RecipientKindTeam:
			teams = append(teams, recipient.Name)
		case common.RecipientKindEmail:
			emails = append(emails, NotificationHandle{Handle: recipient.Name})
		}
	}

	body := IncidentsBody{
		Data: IncidentsData{
			Metadata: Metadata{
				Type: "incidents",
			},
			Attributes: IncidentsAttributes{
				Title: fmt.Sprintf("Access request from %s", reqData.User),
				Fields: IncidentsFields{
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
						Value: d.Severity,
					},
					RootCause: &StringField{
						Type:  "textbox",
						Value: reqData.RequestReason,
					},
					Teams: &StringSliceField{
						Type:  "multiselect",
						Value: teams,
					},
				},
				NotificationHandles: emails,
			},
		},
	}
	var result IncidentsBody
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetResult(&result).
		Post("incidents")
	if err != nil {
		return IncidentsData{}, trace.Wrap(err)
	}
	return result.Data, nil
}

// PostReviewNote posts a note once a new request review appears.
func (d *Datadog) PostReviewNote(ctx context.Context, incidentID, note string) error {
	body := TimelineBody{
		Data: TimelineData{
			Metadata: Metadata{
				Type: "incident_timeline_cells",
			},
			Attributes: TimelineAttributes{
				CellType: "markdown",
				Content: TimelineContent{
					Content: note,
				},
			},
		},
	}
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParam("incident_id", incidentID).
		Post("incidents/{incident_id}/timeline")
	return trace.Wrap(err)
}

// ResolveIncident resolves an incident and posts a note with resolution details.
//
// See: https://docs.datadoghq.com/api/latest/incidents/#update-an-existing-incident
func (d *Datadog) ResolveIncident(ctx context.Context, incidentID, state string) error {
	body := IncidentsBody{
		Data: IncidentsData{
			Metadata: Metadata{
				ID:   incidentID,
				Type: "incidents",
			},
			Attributes: IncidentsAttributes{
				Fields: IncidentsFields{
					State: &StringField{
						Type:  "dropdown",
						Value: state,
					},
				},
			},
		},
	}
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParam("incident_id", incidentID).
		Patch("incidents/{incident_id}")
	return trace.Wrap(err)
}

// ListTeamsPage returns a page of teams.
//
// See: https://docs.datadoghq.com/api/latest/teams/#get-all-teams
func (d *Datadog) ListTeamsPage(ctx context.Context, pageNum int) (ListTeamsBody, error) {
	var result ListTeamsBody
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		SetQueryParam("page[number]", strconv.Itoa(pageNum)).
		Get("team")
	return result, trace.Wrap(err)
}

// GetTeamOncall gets the team's current on-call users.
//
// See: https://docs.datadoghq.com/api/latest/on-call/?s=teams#get-team-on-call-users
func (d *Datadog) GetTeamOncall(ctx context.Context, teamID string) (OncallTeamsBody, error) {
	var result OncallTeamsBody
	_, err := d.client.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		SetPathParam("team_id", teamID).
		SetQueryParam("include", "responders").
		Get("on-call/teams/{team_id}/on-call")
	return result, trace.Wrap(err)
}
