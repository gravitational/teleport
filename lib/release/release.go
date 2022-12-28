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
package release

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type ClientConfig struct {
	// TLSConfig is the client TLS configuration
	TLSConfig *tls.Config
}

func (c *ClientConfig) CheckAndSetDefaults() error {
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLS configuration")
	}

	return nil
}

type Client struct {
	// client is the client used to make calls to the release API
	client *roundtrip.Client
}

func NewClient(cfg ClientConfig) (*Client, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: cfg.TLSConfig,
		},
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := roundtrip.NewClient(fmt.Sprintf("https://%s", cfg.TLSConfig.ServerName), "", roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		client: client,
	}, nil
}

func (c *Client) ListReleases(ctx context.Context) ([]*types.Release, error) {
	if c.client == nil {
		return nil, trace.BadParameter("client not initialized")
	}

	resp, err := c.client.Get(ctx, c.client.Endpoint("teleport-ent"), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.Code() == http.StatusUnauthorized {
		return nil, trace.AccessDenied("access denied by the release server")
	}

	var releases []*types.Release
	err = json.Unmarshal(resp.Bytes(), &releases)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// add human-readable display sizes
	for i := range releases {
		for j := range releases[i].Assets {
			releases[i].Assets[j].DisplaySize = byteCount(releases[i].Assets[j].Size_)
		}
	}

	return releases, err
}

// byteCount converts a size in bytes to a human-readable string.
func byteCount(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

// GetServerAddr returns the release server address from the environment
// variables or, i
func GetServerAddr() string {
	addr := os.Getenv(types.ReleaseServerEnvVar)
	if addr == "" {
		addr = types.DefaultReleaseServerAddr
	}
	return addr
}
