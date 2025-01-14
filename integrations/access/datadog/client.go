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
)

// Datadog is a wrapper around resty.Client.
type Datadog struct {
	// DatadogConfig specifies datadog client configuration.
	DatadogConfig

	// TODO: Datadog API client implemented using resty because implementation is
	// simpler to integrate with the existing framework. Consider using the official
	// datadog api client package: https://github.com/DataDog/datadog-api-client-go.
	client *resty.Client

	// TODO: Remove clientUnstable once on-call API is merged into official API.
	// See: https://docs.datadoghq.com/api/latest/
	clientUnstable *resty.Client
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

	apiEndpointUnstable, err := url.JoinPath(conf.APIEndpoint, APIUnstable)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientUnstable := resty.NewWithClient(&http.Client{
		Timeout: datadogHTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     datadogMaxConns,
			MaxIdleConnsPerHost: datadogMaxConns,
		}}).
		SetBaseURL(apiEndpointUnstable).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("DD-API-KEY", conf.APIKey).
		SetHeader("DD-APPLICATION-KEY", conf.ApplicationKey).
		OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
			req.SetError(&ErrorResult{})
			return nil
		}).OnAfterResponse(onAfterDatadogResponse(statusSink))

	return &Datadog{
		DatadogConfig:  conf,
		client:         client,
		clientUnstable: clientUnstable,
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
			var details string
			switch result := resp.Error().(type) {
			case *ErrorResult:
				details = fmt.Sprintf("http error code=%v, errors=[%v]", resp.StatusCode(), strings.Join(result.Errors, ", "))
			default:
				details = fmt.Sprintf("unknown error result %#v", result)
			}
			return trace.Errorf(details)
		}
		return nil
	}
}

// CheckHealth pings Datadog and ensures required permissions.
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
		// TODO: Verify on-call/teams permissions once required permissions have
		// been published.
		if permission.Attributes.Name == IncidentWritePermissions {
			if permission.Attributes.Restricted {
				return trace.AccessDenied("missing incident_write permissions")
			}
			return nil
		}
	}
	return nil
}

// Create Incident creates a new Datadog incident.
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

// GetOncallTeams gets current on call teams.
func (d *Datadog) GetOncallTeams(ctx context.Context) (OncallTeamsBody, error) {
	var result OncallTeamsBody
	_, err := d.clientUnstable.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		Get("on-call/teams")
	return result, trace.Wrap(err)
}
