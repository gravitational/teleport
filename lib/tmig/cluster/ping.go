// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"context"
	"fmt"
)

// PingResult holds the fields from /webapi/ping relevant to tmig.
type PingResult struct {
	ClusterName string
	ClusterID   string
	Version     string
	ScopesAuth  string // "enabled", "disabled", "unknown"
	ScopesProxy bool
	User        string
	Proxy       string
}

// ScopesEnabled returns true if both auth and proxy report scopes as enabled.
func (p *PingResult) ScopesEnabled() bool {
	return p.ScopesAuth == "enabled" && p.ScopesProxy
}

// Ping fetches cluster metadata via /webapi/ping.
// TODO: wire to real Teleport webclient.Ping when cluster connection is live.
func (c *Client) Ping(ctx context.Context) (*PingResult, error) {
	// Placeholder: real implementation will call webclient.Find and parse response
	return nil, fmt.Errorf("not yet connected to cluster %s", c.proxy)
}
