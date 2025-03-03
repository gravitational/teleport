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

	"cloud.google.com/go/compute/metadata"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
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

// MetadataGetter defines a function that is used to get InstanceMetadata.
// Receives the metadata path and returns the value or an error.
type MetadataGetter func(ctx context.Context, path string) (string, error)

// InstanceRequest contains parameters for making a request to a specific instance.
type InstanceRequest struct {
	// ProjectID is the ID of the VM's project.
	ProjectID string
	// Zone is the instance's zone.
	Zone string
	// Name is the instance's name.
	Name string
	// ID is the instance's ID.
	ID uint64
	// WithoutHostKeys indicates that the client should not request the instance's
	// host keys.
	WithoutHostKeys bool
}

func (req *InstanceRequest) CheckAndSetDefaults() error {
	if req.ProjectID == "" {
		return trace.BadParameter("projectID must be set")
	}
	if req.Zone == "" {
		return trace.BadParameter("zone must be set")
	}
	if req.Name == "" {
		return trace.BadParameter("name must be set")
	}
	return nil
}

// Instance represents a GCP VM.
type Instance struct {
	// Name is the instance's name.
	Name string
	// Zone is the instance's zone.
	Zone string
	// ProjectID is the ID of the project the VM is in.
	ProjectID string
	// ServiceAccount is the email address of the VM's service account, if any.
	ServiceAccount string
	// Labels is the instance's labels.
	Labels map[string]string
	// InternalIPAddress is the instance's private ip.
	InternalIPAddress string
	// ExternalIPAddress is the instance's public ip.
	ExternalIPAddress string
	// HostKeys contain any public keys associated with the instance.
	HostKeys []ssh.PublicKey
	// Fingerprint is generated server-side and used to enforce optimistic
	// locking. It must be unaltered and provided to any update requests to
	// prevent requests being rejected.
	Fingerprint string
	// MetadataItems are key value pairs associated with the instance.
	MetadataItems map[string]string
}

// InstanceGetter provides a mechanism to retrieve information about a
// particular instannce.
type InstanceGetter interface {
	// GetInstance gets a GCP VM.
	GetInstance(ctx context.Context, req *InstanceRequest) (*Instance, error)
	// GetInstanceTags gets the GCP tags for the instance.
	GetInstanceTags(ctx context.Context, req *InstanceRequest) (map[string]string, error)
}

// InstanceMetadataClient is a client for GCP instance metadata.
type InstanceMetadataClient struct {
	getMetadata    MetadataGetter
	instanceGetter InstanceGetter
}

// NewInstanceMetadataClient creates a new instance metadata client.
func NewInstanceMetadataClient(getter InstanceGetter, opts ...ClientOption) (*InstanceMetadataClient, error) {
	ret := &InstanceMetadataClient{
		getMetadata:    getMetadata,
		instanceGetter: getter,
	}

	for _, opt := range opts {
		opt(ret)
	}

	return ret, nil
}

// ClientOption is used to customize the InstanceMetadataClient.
type ClientOption func(*InstanceMetadataClient)

// WithMetadataClient replaces the metadata getter with a custom one.
func WithMetadataClient(getter MetadataGetter) func(*InstanceMetadataClient) {
	return func(imc *InstanceMetadataClient) {
		imc.getMetadata = getter
	}
}

// IsAvailable checks if instance metadata is available.
func (client *InstanceMetadataClient) IsAvailable(ctx context.Context) bool {
	_, err := client.getNumericID(ctx)
	return err == nil
}

func (client *InstanceMetadataClient) getNumericID(ctx context.Context) (uint64, error) {
	idStr, err := client.GetID(ctx)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return 0, trace.BadParameter("Invalid instance ID %q", idStr)
	}
	return id, nil
}

// GetTags gets all the GCP instance's labels and tags.
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
	id, err := client.getNumericID(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := &InstanceRequest{
		ProjectID:       projectID,
		Zone:            zone,
		Name:            name,
		ID:              id,
		WithoutHostKeys: true,
	}
	// Get labels.
	var gcpLabels map[string]string
	inst, err := client.instanceGetter.GetInstance(ctx, req)
	if err == nil {
		gcpLabels = inst.Labels
	} else if trace.IsAccessDenied(err) {
		slog.WarnContext(ctx, "Access denied to instance labels, does the instance have compute.instances.get permission?")

	} else {
		return nil, trace.Wrap(err)
	}

	// Get tags.
	gcpTags, err := client.instanceGetter.GetInstanceTags(ctx, req)
	if trace.IsAccessDenied(err) {
		slog.WarnContext(ctx, "Access denied to resource management tags, does the instance have compute.instances.listEffectiveTags permission?")
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
