// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oracle

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const defaultIMDSAddr = "http://169.254.169.254/opc/v2"

type instance struct {
	ID           string                       `json:"id"`
	DefinedTags  map[string]map[string]string `json:"definedTags"`
	FreeformTags map[string]string            `json:"freeformTags"`
}

// InstanceMetadataClient is a client for Oracle Cloud instance metadata.
type InstanceMetadataClient struct {
	baseIMDSAddr string
}

// NewInstanceMetadataClient creates a new instance metadata client.
func NewInstanceMetadataClient() *InstanceMetadataClient {
	return &InstanceMetadataClient{
		baseIMDSAddr: defaultIMDSAddr,
	}
}

func (clt *InstanceMetadataClient) getInstance(ctx context.Context) (*instance, error) {
	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	addr, err := url.JoinPath(clt.baseIMDSAddr, "instance")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Authorization", "Bearer Oracle")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, trace.ReadError(resp.StatusCode, body)
	}
	var inst instance
	if err := utils.FastUnmarshal(body, &inst); err != nil {
		return nil, trace.Wrap(err)
	}
	return &inst, nil
}

// IsAvailable checks if instance metadata is available.
func (clt *InstanceMetadataClient) IsAvailable(ctx context.Context) bool {
	inst, err := clt.getInstance(ctx)
	if err != nil {
		return false
	}
	_, err = oracle.ParseRegionFromOCID(inst.ID)
	return err == nil
}

// GetTags gets the instance's defined and freeform tags.
func (clt *InstanceMetadataClient) GetTags(ctx context.Context) (map[string]string, error) {
	inst, err := clt.getInstance(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tags := make(map[string]string, len(inst.FreeformTags))
	for k, v := range inst.FreeformTags {
		tags[k] = v
	}
	for namespace, definedTags := range inst.DefinedTags {
		for k, v := range definedTags {
			tags[namespace+"/"+k] = v
		}
	}
	return tags, nil
}

// GetHostname gets the hostname set by the cloud instance that Teleport
// should use, if any.
func (clt *InstanceMetadataClient) GetHostname(ctx context.Context) (string, error) {
	inst, err := clt.getInstance(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	for k, v := range inst.FreeformTags {
		if strings.EqualFold(k, types.CloudHostnameTag) {
			return v, nil
		}
	}
	return "", trace.NotFound("tag %q not found", types.CloudHostnameTag)
}

// GetType gets the cloud instance type.
func (clt *InstanceMetadataClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeOracle
}

// GetID gets the ID of the cloud instance.
func (clt *InstanceMetadataClient) GetID(ctx context.Context) (string, error) {
	inst, err := clt.getInstance(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return inst.ID, nil
}
