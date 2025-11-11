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

package release

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ClientConfig contains configuration for the release client
type ClientConfig struct {
	// TLSConfig is the client TLS configuration
	TLSConfig *tls.Config
	// ReleaseServerAddr is the address of the release server
	ReleaseServerAddr string
}

// CheckAndSetDefaults checks and sets default config values
func (c *ClientConfig) CheckAndSetDefaults() error {
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLS configuration")
	}

	if c.ReleaseServerAddr == "" {
		return trace.BadParameter("missing release server address")
	}

	return nil
}

// Client is a client to make HTTPS requests to the
// release server using the Teleport Enterprise license
// as authentication
type Client struct {
	// client is the client used to make calls to the release API
	client *roundtrip.Client
}

// NewClient returns a new release client with a client
// to make https requests to the release server
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

	client, err := roundtrip.NewClient(fmt.Sprintf("https://%s", cfg.ReleaseServerAddr), "", roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		client: client,
	}, nil
}

// ListReleases calls the release server and returns a list of releases
func (c *Client) ListReleases(ctx context.Context) ([]*types.Release, error) {
	if c.client == nil {
		return nil, trace.BadParameter("client not initialized")
	}

	resp, err := c.client.Get(ctx, c.client.Endpoint(types.EnterpriseReleaseEndpoint), nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve releases from release server", "error", err)
		return nil, trace.Wrap(err)
	}

	if resp.Code() == http.StatusUnauthorized {
		return nil, trace.AccessDenied("access denied by the release server")
	}

	if resp.Code() != http.StatusOK {
		return nil, trace.Errorf("release server responded with status %d", resp.Code())
	}

	var releases []Release
	err = json.Unmarshal(resp.Bytes(), &releases)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var responseReleases []*types.Release
	for _, r := range releases {
		var releaseAssets []*types.Asset
		for _, a := range r.Assets {
			releaseAssets = append(releaseAssets, &types.Asset{
				Arch:        a.Arch,
				Description: a.Description,
				Name:        a.Name,
				OS:          a.OS,
				SHA256:      a.SHA256,
				AssetSize:   a.Size,
				DisplaySize: utils.ByteCount(a.Size),
				ReleaseIDs:  a.ReleaseIDs,
				PublicURL:   a.PublicURL,
			})
		}

		responseReleases = append(responseReleases, &types.Release{
			NotesMD:   r.NotesMD,
			Product:   r.Product,
			ReleaseID: r.ReleaseID,
			Status:    r.Status,
			Version:   r.Version,
			Assets:    releaseAssets,
		})
	}

	return responseReleases, err
}

// GetServerAddr returns the release server address from the environment
// variables or, if not set, the default value
func GetServerAddr() string {
	addr := os.Getenv(types.ReleaseServerEnvVar)
	if addr == "" {
		addr = types.DefaultReleaseServerAddr
	}
	return addr
}

// Release corresponds to a Teleport Enterprise release
// returned by the release service
type Release struct {
	// NotesMD is the notes of the release in markdown
	NotesMD string `json:"notesMd"`
	// Product is the release product, teleport or teleport-ent
	Product string `json:"product"`
	// ReleaseId is the ID of the product
	ReleaseID string `json:"releaseId"`
	// Status is the status of the release
	Status string `json:"status"`
	// Version is the version of the release
	Version string `json:"version"`
	// Assets is a list of assets related to the release
	Assets []*Asset `json:"assets"`
}

// Asset represents a release asset returned by the
// release service
type Asset struct {
	// Arch is the architecture of the asset
	Arch string `json:"arch"`
	// Description is the description of the asset
	Description string `json:"description"`
	// Name is the name of the asset
	Name string `json:"name"`
	// OS is which OS the asset is built for
	OS string `json:"os"`
	// SHA256 is the sha256 of the asset
	SHA256 string `json:"sha256"`
	// Size is the size of the release in bytes
	Size int64 `json:"size"`
	// ReleaseIDs is a list of releases that have the asset included
	ReleaseIDs []string `json:"releaseIds"`
	// PublicURL is the public URL used to download the asset
	PublicURL string `json:"publicUrl"`
}
