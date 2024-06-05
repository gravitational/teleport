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

package gcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/compute/metadata"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/defaults"
)

const gcpHostnameTag = "label/" + types.CloudHostnameTag

// contextRoundTripper is a http.RoundTripper that adds a context.Context to
// requests.
type contextRoundTripper struct {
	ctx       context.Context
	transport http.RoundTripper
}

func (rt contextRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.transport.RoundTrip(req.WithContext(rt.ctx))
	return resp, trace.Wrap(err)
}

type metadataGetter func(ctx context.Context, path string) (string, error)

// InstanceMetadataClient is a client for GCP instance metadata.
type InstanceMetadataClient struct {
	instancesClient gcp.InstancesClient
	getMetadata     metadataGetter

	labelPermissionErrorOnce sync.Once
	tagPermissionErrorOnce   sync.Once
}

// NewInstanceMetadataClient creates a new instance metadata client.
func NewInstanceMetadataClient(ctx context.Context) (*InstanceMetadataClient, error) {
	instancesClient, err := gcp.NewInstancesClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InstanceMetadataClient{
		instancesClient: instancesClient,
		getMetadata:     getMetadata,
	}, nil
}

// IsAvailable checks if instance metadata is available.
func (client *InstanceMetadataClient) IsAvailable(ctx context.Context) bool {
	instanceData, err := client.getMetadata(ctx, "instance")
	return err == nil && instanceData != ""
}

// GetTags gets all of the GCP instance's labels (note: these are separate from
// its tags, which we do not use).
func (client *InstanceMetadataClient) GetTags(ctx context.Context) (map[string]string, error) {
	// Get a bunch of info from instance metadata.
	projectID, err := client.GetProjectID(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	zone, err := client.GetZone(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	name, err := client.GetName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	idStr, err := client.GetID(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := &gcp.InstanceRequest{
		ProjectID:       projectID,
		Zone:            zone,
		Name:            name,
		ID:              id,
		WithoutHostKeys: true,
	}
	// Get labels.
	var gcpLabels map[string]string
	inst, err := client.instancesClient.GetInstance(ctx, req)
	if err == nil {
		gcpLabels = inst.Labels
	} else if trace.IsAccessDenied(err) {
		client.labelPermissionErrorOnce.Do(func() {
			slog.WarnContext(ctx, "Access denied to instance labels, does the instance have compute.instances.get permission?")
		})
	} else {
		return nil, trace.Wrap(err)
	}

	// Get tags.
	gcpTags, err := client.instancesClient.GetInstanceTags(ctx, req)
	if trace.IsAccessDenied(err) {
		client.tagPermissionErrorOnce.Do(func() {
			slog.WarnContext(ctx, "Access denied to resource management tags, does the instance have compute.instances.listEffectiveTags permission?")
		})
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	tags := make(map[string]string, len(gcpLabels)+len(gcpTags))
	for k, v := range gcpLabels {
		tags["label/"+k] = v
	}
	for k, v := range gcpTags {
		tags["tag/"+k] = v
	}

	return tags, nil
}

// GetHostname gets the hostname set by the cloud instance that Teleport
// should use, if any.
func (client *InstanceMetadataClient) GetHostname(ctx context.Context) (string, error) {
	tags, err := client.GetTags(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	value, ok := tags[gcpHostnameTag]
	if !ok {
		return "", trace.NotFound("label %q not found", gcpHostnameTag)
	}
	return value, nil
}

// GetType gets the cloud instance type.
func (client *InstanceMetadataClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeGCP
}

// GetID gets the ID of the cloud instance.
func (client *InstanceMetadataClient) GetID(ctx context.Context) (string, error) {
	id, err := client.getMetadata(ctx, "instance/id")
	return id, trace.Wrap(err)
}

// GetProjectID gets the instance's project ID.
func (client *InstanceMetadataClient) GetProjectID(ctx context.Context) (string, error) {
	projectID, err := client.getMetadata(ctx, "project/project-id")
	return projectID, trace.Wrap(err)
}

// GetZone gets the instance's zone.
func (client *InstanceMetadataClient) GetZone(ctx context.Context) (string, error) {
	fullZone, err := client.getMetadata(ctx, "instance/zone")
	if err != nil {
		return "", trace.Wrap(err)
	}
	// zone is formatted as "projects/<project number>/zones/<zone>", we just need the last part
	zoneParts := strings.Split(fullZone, "/")
	return zoneParts[len(zoneParts)-1], nil
}

// GetName gets the instance's name.
func (client *InstanceMetadataClient) GetName(ctx context.Context) (string, error) {
	name, err := client.getMetadata(ctx, "instance/name")
	return name, trace.Wrap(err)
}

// getMetadataClient gets an instance metadata client that will use the
// provided context.
func getMetadataClient(ctx context.Context) (*metadata.Client, error) {
	transport, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return metadata.NewClient(&http.Client{
		Transport: contextRoundTripper{
			ctx:       ctx,
			transport: transport,
		},
	}), nil
}

func convertMetadataError(err error) error {
	var metadataErr *metadata.Error
	if errors.As(err, &metadataErr) {
		return trace.ReadError(metadataErr.Code, []byte(metadataErr.Message))
	}
	return err
}

// get gets GCP instance metadata from an arbitrary path.
func getMetadata(ctx context.Context, suffix string) (string, error) {
	client, err := getMetadataClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	resp, err := client.GetWithContext(ctx, suffix)
	return resp, trace.Wrap(convertMetadataError(err))
}

// GetIDToken gets an ID token from GCP instance metadata.
func GetIDToken(ctx context.Context) (string, error) {
	audience := "teleport.cluster.local"
	resp, err := getMetadata(ctx, fmt.Sprintf(
		"instance/service-accounts/default/identity?audience=%s&format=full&licenses=FALSE",
		url.QueryEscape(audience),
	))
	return resp, trace.Wrap(err)
}
