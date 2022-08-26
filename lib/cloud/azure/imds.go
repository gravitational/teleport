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
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	// metadataReadLimit is the largest number of bytes that will be read from imds responses.
	metadataReadLimit = 1_000_000
	// imdsURL is the default base URL for Azure instance metadata.
	imdsURL = "http://169.254.169.254/metadata"
	// minimumSupportedAPIVersion is the minimum supported version of the Azure instance metadata API.
	minimumSupportedAPIVersion = "2019-06-04"
)

func init() {
	cloud.RegisterIMDSProvider(string(types.InstanceMetadataTypeAzure), func(ctx context.Context) (cloud.InstanceMetadata, error) {
		return NewInstanceMetadataClient(), nil
	})
}

// InstanceMetadataClient is a client for Azure instance metadata.
type InstanceMetadataClient struct {
	baseURL    string
	apiVersion string
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
	client.apiVersion = targetVersion
	return nil
}

// get gets the raw metadata from a specified path.
func (client *InstanceMetadataClient) get(ctx context.Context, route string) ([]byte, error) {
	httpClient := &http.Client{Transport: &http.Transport{Proxy: nil}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+route, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Add("Metadata", "True")
	query := req.URL.Query()
	query.Add("format", "json")
	if client.apiVersion != "" {
		query.Add("version", client.apiVersion)
	}
	req.URL.RawQuery = query.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	body, err := utils.ReadAtMost(resp.Body, metadataReadLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, trace.ReadError(resp.StatusCode, body)
	}
	return body, nil
}

// getVersions gets a list of all API versions supported by this instance.
func (client *InstanceMetadataClient) getVersions(ctx context.Context) ([]string, error) {
	versions := struct {
		APIVersions []string `json:"apiVersions"`
	}{}
	body, err := client.get(ctx, "/versions")
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
	if client.apiVersion != "" {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	err := client.selectVersion(ctx)
	return err == nil
}

func (client *InstanceMetadataClient) GetTags(ctx context.Context) (map[string]string, error) {
	if !client.IsAvailable(ctx) {
		return nil, trace.NotFound("Instance metadata not available")
	}

	rawTags := []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{}
	body, err := client.get(ctx, "/instance/compute/tagsList")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := utils.FastUnmarshal(body, rawTags); err != nil {
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
		return "", trace.NotFound("Tag %q not found", types.CloudHostnameTag)
	}
	return value, nil
}

// selectVersion selects the most recent API version greater than or equal to
// a minimum version. Versions are represented as dates of the form YYYY-MM-DD.
func selectVersion(versions []string, minimumVersion string) (string, error) {
	if len(versions) == 0 {
		return "", trace.BadParameter("No versions provided")
	}
	targetVersion := versions[len(versions)-1]
	if targetVersion < minimumVersion {
		return "", trace.NotImplemented("Tags not supported (requires minimum api version %v)", minimumVersion)
	}
	return targetVersion, nil
}
