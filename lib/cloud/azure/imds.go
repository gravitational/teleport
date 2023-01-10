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
package azure

import (
	"context"
	"net/http"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// imdsURL is the default base URL for Azure instance metadata.
	imdsURL = "http://169.254.169.254/metadata"
	// minimumSupportedAPIVersion is the minimum supported version of the Azure instance metadata API.
	minimumSupportedAPIVersion = "2019-06-04"
)

// InstanceMetadataClient is a client for Azure instance metadata.
type InstanceMetadataClient struct {
	baseURL    string
	apiVersion string

	mu sync.RWMutex
}

// InstanceMetadataClientOption allows setting options as functional arguments to an InstanceMetadataClient.
type InstanceMetadataClientOption func(client *InstanceMetadataClient)

// WithBaseURL sets the base URL for the metadata client. Used in tests.
func WithBaseURL(url string) InstanceMetadataClientOption {
	return func(client *InstanceMetadataClient) {
		client.baseURL = url
	}
}

// NewInstanceMetadataClient creates a new instance metadata client.
func NewInstanceMetadataClient(opts ...InstanceMetadataClientOption) *InstanceMetadataClient {
	client := &InstanceMetadataClient{}
	for _, opt := range opts {
		opt(client)
	}

	if client.baseURL == "" {
		client.baseURL = imdsURL
	}
	return client
}

// GetAPIVersion gets the Azure instance metadata API version this client
// is using.
func (client *InstanceMetadataClient) GetAPIVersion() string {
	client.mu.RLock()
	defer client.mu.RUnlock()
	return client.apiVersion
}

// GetType gets the cloud instance type.
func (client *InstanceMetadataClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeAzure
}

// selectVersion sets the Azure instance metadata API version.
func (client *InstanceMetadataClient) selectVersion(ctx context.Context) error {
	versions, err := client.getVersions(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// Use the most recent supported API version.
	targetVersion, err := selectVersion(versions, minimumSupportedAPIVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	client.apiVersion = targetVersion
	return nil
}

// getRawMetadata gets the raw metadata from a specified path.
func (client *InstanceMetadataClient) getRawMetadata(ctx context.Context, route string) ([]byte, error) {
	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+route, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Add("Metadata", "True")
	query := req.URL.Query()
	query.Add("format", "json")
	apiVersion := client.GetAPIVersion()
	if apiVersion != "" {
		query.Add("api-version", apiVersion)
	}
	req.URL.RawQuery = query.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseMetadataClientError(resp.StatusCode, body)
	}
	return body, nil
}

// getVersions gets a list of all API versions supported by this instance.
func (client *InstanceMetadataClient) getVersions(ctx context.Context) ([]string, error) {
	versions := struct {
		APIVersions []string `json:"apiVersions"`
	}{}
	body, err := client.getRawMetadata(ctx, "/versions")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := utils.FastUnmarshal(body, &versions); err != nil {
		return nil, trace.Wrap(err)
	}
	return versions.APIVersions, nil
}

// IsAvailable checks if instance metadata is available.
func (client *InstanceMetadataClient) IsAvailable(ctx context.Context) bool {
	if client.GetAPIVersion() != "" {
		return true
	}

	err := client.selectVersion(ctx)
	return err == nil
}

// GetTags gets all of the Azure instance's tags.
func (client *InstanceMetadataClient) GetTags(ctx context.Context) (map[string]string, error) {
	if !client.IsAvailable(ctx) {
		return nil, trace.NotFound("Instance metadata is not available")
	}

	rawTags := []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{}
	body, err := client.getRawMetadata(ctx, "/instance/compute/tagsList")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := utils.FastUnmarshal(body, &rawTags); err != nil {
		return nil, trace.Wrap(err)
	}

	tags := make(map[string]string, len(rawTags))
	for _, tag := range rawTags {
		tags[tag.Name] = tag.Value
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
	value, ok := tags[types.CloudHostnameTag]
	if !ok {
		return "", trace.NotFound("tag %q not found", types.CloudHostnameTag)
	}
	return value, nil
}

// GetID gets the Azure resource ID of the cloud instance.
func (client *InstanceMetadataClient) GetID(ctx context.Context) (string, error) {
	compute := struct {
		ResourceID string `json:"resourceId"`
	}{}
	body, err := client.getRawMetadata(ctx, "/instance/compute")
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := utils.FastUnmarshal(body, &compute); err != nil {
		return "", trace.Wrap(err)
	}

	if compute.ResourceID == "" {
		return "", trace.NotFound("instance resource ID not available")
	}

	return compute.ResourceID, nil
}

// GetInstanceMetadata returns an empty InstanceMetadata.
// Azure InstanceMetadata lookup is not yet implemented.
func (client *InstanceMetadataClient) GetInstanceMetadata(ctx context.Context) (*types.InstanceMetadata, error) {
	return &types.InstanceMetadata{}, nil
}

// selectVersion selects the most recent API version greater than or equal to
// a minimum version. Versions are represented as dates of the form YYYY-MM-DD.
func selectVersion(versions []string, minimumVersion string) (string, error) {
	if len(versions) == 0 {
		return "", trace.BadParameter("azure did not provide any versions to select from")
	}
	// Versions are in ascending order.
	targetVersion := versions[len(versions)-1]
	if targetVersion < minimumVersion {
		return "", trace.NotImplemented("tags not supported (requires minimum API version %v, current version is %v)", minimumVersion, targetVersion)
	}
	return targetVersion, nil
}
