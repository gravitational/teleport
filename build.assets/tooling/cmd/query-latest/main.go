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

// Command query-latest returns the highest semver release for a versionSpec
// query-latest ignores drafts and pre-releases.
//
// For example:
//
//	query-latest v8.1.5     -> v8.1.5
//	query-latest v8.1.3     -> error, no matching release (this is a tag, but not a release)
//	query-latest v8.0.0-rc3 -> error, no matching release (this is a pre-release, in github and in semver)
//	query-latest v7.0       -> v7.0.2
//	query-latest v5         -> v5.2.4
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/tomnomnom/linkheader"
	"golang.org/x/mod/semver" //nolint:depguard // Usage precedes the x/mod/semver rule.

	"github.com/gravitational/teleport/build.assets/tooling/lib/github"
)

const (
	publicECRRegistryEndpoint = "https://public.ecr.aws/"
	publicECRImageName        = "gravitational/teleport-ent-distroless"
)

type lookupConfig struct {
	source      string
	versionSpec string
}

func main() {
	versionSpec, err := parseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags: %v.", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tag, err := getLatest(ctx, versionSpec)
	if err != nil {
		log.Fatalf("Query failed: %v.", err)
	}

	fmt.Println(tag)
}

func parseFlags() (*lookupConfig, error) {
	source := flag.String("source", "github", "source to use for querying the latest release")

	flag.Parse()
	if flag.NArg() == 0 {
		return nil, trace.BadParameter("missing argument: vX.X")
	} else if flag.NArg() > 1 {
		return nil, trace.BadParameter("unexpected positional arguments: %q", flag.Args()[1:])
	}

	versionSpec := flag.Arg(0)
	if !semver.IsValid(versionSpec) {
		return nil, trace.Errorf("version spec %q is not a valid semver", versionSpec)
	}

	return &lookupConfig{
		versionSpec: versionSpec,
		source:      strings.ToLower(*source),
	}, nil
}

func getLatest(ctx context.Context, config *lookupConfig) (string, error) {
	switch config.source {
	case "github":
		return getLatestViaGithubReleases(ctx, config.versionSpec)
	case "ecr-public":
		return getLatestViaECRPublic(ctx, config.versionSpec)
	default:
		return "", trace.Errorf("unsupported source %q", config.source)
	}
}

func getLatestViaGithubReleases(ctx context.Context, versionSpec string) (string, error) {
	gh := github.NewGitHub()

	releases, err := gh.ListReleases(ctx, "gravitational", "teleport")
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(releases) == 0 {
		return "", trace.NotFound("failed to find any releases on GitHub")
	}

	// filter drafts and prereleases, which shouldn't be tracked by latest docker images
	var tags []string
	for _, r := range releases {
		if r.GetDraft() {
			continue
		}
		if r.GetPrerelease() {
			continue
		}
		tag := r.GetTagName()
		if semver.Prerelease(tag) != "" {
			continue
		}
		tags = append(tags, tag)
	}

	semver.Sort(tags)

	// semver.Sort is ascending, so we loop in reverse
	for i := len(tags) - 1; i >= 0; i-- {
		tag := tags[i]
		if strings.HasPrefix(tag, versionSpec) {
			return tag, nil
		}
	}

	return "", trace.NotFound("no releases matched %q", versionSpec)
}

// Gets the latest matching version by querying the images published to the public.ecr.aws/gravitational/teleport-ent-distroless
// PublicECR implements the Docker Registry HTTP API. Use it to get the latest matching image.
// See https://github.com/opencontainers/distribution-spec/blob/main/spec.md#content-discovery
func getLatestViaECRPublic(ctx context.Context, versionSpec string) (string, error) {
	ocidc, err := NewOCIDistributionClient(ctx, publicECRRegistryEndpoint)
	if err != nil {
		return "", trace.Wrap(err, "failed to build ECR public client for %q", publicECRRegistryEndpoint)
	}

	tags, err := ocidc.GetTags(ctx, publicECRImageName)
	if err != nil {
		return "", trace.Wrap(err, "failed to get tags for %q from %q", publicECRImageName, publicECRRegistryEndpoint)
	}

	// Filter the tags and return the latest matching the version spec
	latestMatching := "v0"
	foundMatch := false
	for _, tag := range tags {
		// The semver package doesn't actually support the semver spec, and requires a preceding `v`
		tag = "v" + tag
		if !semver.IsValid(tag) {
			continue
		}

		if !strings.HasPrefix(tag, versionSpec) {
			continue
		}

		// If the tag is newer than the previously recorded newest
		if semver.Compare(tag, latestMatching) > 0 {
			foundMatch = true
			latestMatching = tag
		}
	}

	if !foundMatch {
		return "", trace.NotFound("no releases matched %q", versionSpec)
	}

	return latestMatching, nil
}

// Simple client for the OCI distribution API: https://github.com/opencontainers/distribution-spec/blob/main/spec.md
// This does not currently handle token expiration
type ociDistributionClient struct {
	endpoint string
	token    string
}

// Creates a new OCI distribution API client.
func NewOCIDistributionClient(ctx context.Context, endpoint string) (*ociDistributionClient, error) {
	client := &ociDistributionClient{
		endpoint: endpoint,
	}

	token, err := client.getOCIDistributionToken(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get OCI distribution API token for %q", endpoint)
	}
	client.token = token

	return client, nil
}

// Gets a new anonymous authentication token for the OCI distribution API.
func (ocidc *ociDistributionClient) getOCIDistributionToken(ctx context.Context) (string, error) {
	requestURL, err := url.JoinPath(ocidc.endpoint, "token")
	if err != nil {
		return "", trace.Wrap(err, "failed to build token URL")
	}

	tokenResponse := &struct {
		Token string
	}{}

	if _, err := ocidc.doRequest(ctx, requestURL, http.MethodGet, tokenResponse); err != nil {
		return "", trace.Wrap(err, "request to %q failed", requestURL)
	}

	if tokenResponse.Token == "" {
		return "", trace.Errorf("token API endpoint did not include a token in the API response")
	}

	return tokenResponse.Token, nil
}

// Gets a list of all tags for the provided image.
func (ocidc *ociDistributionClient) GetTags(ctx context.Context, image string) ([]string, error) {
	baseRequestURL, err := url.JoinPath(ocidc.endpoint, "v2", image, "tags", "list")
	if err != nil {
		return nil, trace.Wrap(err, "failed to build tags URL")
	}

	// AWS ECR public does not conform to the OCI distribution spec. Responses are limited to 1000
	// tags despite the spec explicitly requiring that `n` results must always be returned if `n`
	// tags are available. Tags are also not in lexical order as explicitly defined by the spec,
	// so all tags must be retrieved by pagination every time.
	tags := []string{}
	requestCountLimit := 100
	tagsPerRequest := 1000
	baseRequestURL = fmt.Sprintf("%s?n=%d", baseRequestURL, tagsPerRequest)
	requestURL := baseRequestURL

	for requestCount := 0; requestCount < requestCountLimit; requestCount++ {
		tagsResponse := &struct {
			Tags []string
		}{}

		response, err := ocidc.doRequest(ctx, requestURL, http.MethodGet, tagsResponse)
		if err != nil {
			return nil, trace.Wrap(err, "request to %q failed", requestURL)
		}

		tags = append(tags, tagsResponse.Tags...)

		if len(tagsResponse.Tags) < tagsPerRequest {
			break
		}

		// Note: If there are _exactly_ multiples of tagsPerRequest in the registry,
		// this may fail on public ECR because public ECR does not properly implement
		// the spec. I don't have an easy easy way to test this. In this case, the
		// response may not include a Link header of type `next` (because there are
		// no more images in the last batch), and the public ECR implementation does
		// not support the `last` query parameter with the tag as the value.

		if linkHeaders, ok := response.Header["Link"]; ok {
			links := linkheader.ParseMultiple(linkHeaders)
			nextLinks := links.FilterByRel("next")
			if len(nextLinks) > 0 {
				requestURL = ocidc.endpoint + nextLinks[0].URL
				continue
			}
		}

		requestURL = fmt.Sprintf(baseRequestURL, "&last=%s", url.QueryEscape(tags[len(tags)-1]))
	}

	return tags, nil
}

// Sends and HTTP request to the given URL and parses the JSON response into the outVal, which is expected to be a pointer.
// The returned response body read will be closed by this function.
func (ocidc *ociDistributionClient) doRequest(ctx context.Context, requestURL string, method string, outVal any) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return nil, trace.Wrap(err, "failed to build request")
	}

	if ocidc.token != "" {
		request.Header.Add("Authorization", "Bearer "+ocidc.token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return response, trace.Wrap(err, "request failed")
	}

	// Read the body. This is needed even when a non-2xx response code is returned.
	// Errors will be aggregated below.
	responseBody, readBodyErr := readLimitedSizeResponse(response.Body)
	closeErr := response.Body.Close()

	// Check the response status code, and error if the request was unsuccessful
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		statusCodeErr := trace.Errorf("received response code %d", response.StatusCode)
		if readBodyErr == nil {
			statusCodeErr = trace.Errorf("received response code %d with body %q ", response.StatusCode, string(responseBody))
		}

		return response, trace.NewAggregate(
			statusCodeErr,
			trace.Wrap(readBodyErr, "failed to read response body for error response"),
			trace.Wrap(closeErr, "failed to close response body reader"),
		)
	}

	if err = json.Unmarshal(responseBody, outVal); err != nil {
		return response, trace.Wrap(err, "failed to unmarshal response body")
	}

	return response, nil
}

// Read up to a megabyte of response body
func readLimitedSizeResponse(response io.Reader) ([]byte, error) {
	sizeLimit := int64(1024 * 1024)
	responseBody := make([]byte, sizeLimit)

	readBytes, err := io.ReadFull(io.LimitReader(response, sizeLimit), responseBody)

	// Truncate the body for return values
	responseBody = responseBody[0:readBytes]

	// EOF errors will _always_ occur when 0 < readBytes < sizeLimit
	if readBytes > 0 && int64(readBytes) < sizeLimit && errors.Is(err, io.EOF) {
		return responseBody, nil
	}

	// Check if at least one more byte can be read. If so, the response was too large and and error
	// should be returned.
	extraBytes := make([]byte, 1)
	extraByteCount, extraReadErr := response.Read(extraBytes)
	if extraByteCount == 0 {
		return responseBody, nil
	}
	if extraReadErr != nil {
		return nil, trace.Wrap(err, "failed to check if there were more than %d bytes in the response", sizeLimit)
	}

	// Normal error case when there are <= sizeLimit bytes in the body, but an error occurred while reading it
	return nil, trace.Wrap(err, "failed to read response body")
}
