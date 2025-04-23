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

import "time"

const (
	backoffBase = time.Millisecond
	backoffMax  = time.Second * 5
	httpTimeout = time.Second * 30
)

// Config represents MS Graph API and Bot API config
type Config struct {
	// AppID application id (uuid, for bots must be underlying app id, not bot's id)
	AppID string `toml:"app_id"`
	// AppSecret application secret token
	AppSecret string `toml:"app_secret"`
	// TenantID ms tenant id
	TenantID string `toml:"tenant_id"`
	// Region to be used by the Microsoft Graph API client
	Region string `toml:"region"`
	// TeamsAppID represents Teams App ID
	TeamsAppID string `toml:"teams_app_id"`

	// url represents url configuration for testing
	url struct {
		tokenBaseURL        string
		graphBaseURL        string
		botFrameworkBaseURL string
	} `toml:"-"`
}

// SetBaseURLs is used to point MS Graph API to test servers
func (c *Config) SetBaseURLs(token, graph, bot string) {
	c.url.tokenBaseURL = token
	c.url.graphBaseURL = graph
	c.url.botFrameworkBaseURL = bot
}
