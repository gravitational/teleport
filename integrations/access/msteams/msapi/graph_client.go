// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gravitational/trace"
)

const (
	graphAuthScope = "https://graph.microsoft.com/.default"
	graphBaseURL   = "https://graph.microsoft.com/v1.0"
)

// GraphClient represents client to MS Graph API
type GraphClient struct {
	Client
}

// graphError represents MS Graph error
type graphError struct {
	E struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// genericGraphResponse represents the utility struct for parsing MS Graph API response
type genericGraphResponse struct {
	Context string          `json:"@odata.context"`
	Count   int             `json:"@odata.count"`
	Value   json.RawMessage `json:"value"`
}

// TeamsApp represents teamsApp resource
type TeamsApp struct {
	ID                 string `json:"id"`
	ExternalID         string `json:"externalId"`
	DisplayName        string `json:"displayName"`
	DistributionMethod string `json:"distributionMethod"`
}

// InstalledApp represents teamsAppInstallation resource
type InstalledApp struct {
	ID       string   `json:"id"`
	TeamsApp TeamsApp `json:"teamsApp"`
}

// Chat represents chat resource
type Chat struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
	WebURL   string `json:"webUrl"`
}

// User represents user resource
type User struct {
	ID       string `json:"id"`
	Name     string `json:"displayName"`
	Mail     string `json:"mail"`
	JobTitle string `json:"jobTitle"`
}

// NewGraphClient creates MS Graph API client
func NewGraphClient(config Config) *GraphClient {
	baseURL := config.url.graphBaseURL
	if baseURL == "" {
		baseURL = graphBaseURL
	}

	return &GraphClient{
		Client: Client{
			token:   tokenWithTTL{scope: graphAuthScope, baseURL: config.url.tokenBaseURL},
			baseURL: baseURL,
			config:  config,
		},
	}
}

// Error returns error string
func (e graphError) Error() string {
	return e.E.Code + " " + e.E.Message
}

// GetErrorCode returns the
func GetErrorCode(err error) string {
	var graphErr graphError
	ok := errors.As(err, &graphErr)
	if !ok {
		return ""
	}
	return graphErr.E.Code
}

// GetTeamsApp returns the list of installed team apps
func (c *GraphClient) GetTeamsApp(ctx context.Context, teamsAppID string) (*TeamsApp, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "appCatalogs/teamsApps",
		Filter:   "externalId eq '" + teamsAppID + "'",
		Response: g,
		Err:      &graphError{},
	}

	err := c.request(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps []TeamsApp
	err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(apps) == 0 {
		return nil, trace.NotFound("App %v not found", teamsAppID)
	}

	if len(apps) > 1 {
		return nil, trace.Errorf("There is more than one app having externalID eq %v", teamsAppID)
	}

	return &apps[0], nil
}

// GetAppForUser returns installedApp for a given app and user
func (c *GraphClient) GetAppForUser(ctx context.Context, app *TeamsApp, userID string) (*InstalledApp, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "users/" + userID + "/teamWork/installedApps",
		Expand:   []string{"teamsApp"},
		Filter:   "teamsApp/id eq '" + app.ID + "'",
		Response: g,
		Err:      &graphError{},
	}

	err := c.request(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps []InstalledApp
	err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(apps) == 0 {
		return nil, trace.NotFound("App %v for user %v not found", app.ID, userID)
	}

	if len(apps) > 1 {
		return nil, trace.Errorf("There is more than one app having id eq %v", app.ID)
	}

	return &apps[0], nil
}

// GetAppForTeam returns installedApp for a given app and user
// This call requires the permission `TeamsAppInstallation.ReadWriteSelfForTeam.All`. This is overkill as we're only
// reading. Resource Specific Consent is not enabled by default (still in preview) and we cannot rely on it.
// https://docs.microsoft.com/en-us/microsoftteams/platform/graph-api/rsc/resource-specific-consent
func (c *GraphClient) GetAppForTeam(ctx context.Context, app *TeamsApp, teamID string) (*InstalledApp, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "teams/" + teamID + "/installedApps",
		Expand:   []string{"teamsApp"},
		Filter:   "teamsApp/id eq '" + app.ID + "'",
		Response: g,
		Err:      &graphError{},
	}

	err := c.request(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps []InstalledApp
	err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(apps) == 0 {
		return nil, trace.NotFound("App %v for team %v not found", app.ID, teamID)
	}

	if len(apps) > 1 {
		return nil, trace.Errorf("There is more than one app having id eq %v", app.ID)
	}

	return &apps[0], nil
}

// InstallAppForUser returns installed apps for user
func (c *GraphClient) InstallAppForUser(ctx context.Context, userID, teamAppID string) error {
	body := `
		{
			"teamsApp@odata.bind": "https://graph.microsoft.com/v1.0/appCatalogs/teamsApps/` + teamAppID + `"
		}	
	`

	request := request{
		Method:      http.MethodPost,
		Path:        "users/" + userID + "/teamWork/installedApps",
		Body:        body,
		Err:         &graphError{},
		SuccessCode: http.StatusCreated,
	}

	return trace.Wrap(c.request(ctx, request))
}

// UninstallAppForUser returns installed apps for user
func (c *GraphClient) UninstallAppForUser(ctx context.Context, userID, teamAppID string) error {
	body := `
		{
			"teamsApp@odata.bind": "https://graph.microsoft.com/v1.0/appCatalogs/teamsApps/` + teamAppID + `"
		}
	`

	request := request{
		Method:      http.MethodDelete,
		Path:        "users/" + userID + "/teamWork/installedApps/" + teamAppID,
		Body:        body,
		Err:         &graphError{},
		SuccessCode: http.StatusNoContent,
	}

	return trace.Wrap(c.request(ctx, request))
}

// GetChatForInstalledApp returns a chat between user and installed app
func (c *GraphClient) GetChatForInstalledApp(ctx context.Context, userID, installationID string) (Chat, error) {
	var chat Chat

	request := request{
		Method:   http.MethodGet,
		Path:     "users/" + userID + "/teamwork/installedApps/" + installationID + "/chat",
		Response: &chat,
		Err:      &graphError{},
	}

	err := c.request(ctx, request)
	if err != nil {
		return chat, trace.Wrap(err)
	}

	return chat, nil
}

// GetUserByEmail searches a user by email
func (c *GraphClient) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "users",
		Filter:   "mail eq '" + email + "'",
		Response: &g,
		Err:      &graphError{},
	}

	err := c.request(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var users []User
	err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&users)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(users) == 0 {
		return nil, trace.NotFound("User by email %v not found", email)
	}

	if len(users) > 1 {
		return nil, trace.Errorf("There is more than one user with email eq %v", email)
	}

	return &users[0], nil
}

// GetUserByID returns a user by ID
func (c *GraphClient) GetUserByID(ctx context.Context, id string) (*User, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "users",
		Filter:   "id eq '" + id + "'",
		Response: &g,
		Err:      &graphError{},
	}

	err := c.request(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var users []User
	err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&users)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(users) == 0 {
		return nil, trace.NotFound("User %v not found", id)
	}

	if len(users) > 1 {
		return nil, trace.Errorf("There is more than one user with id %v", id)
	}

	return &users[0], nil
}
