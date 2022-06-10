/*
Copyright 2021 Gravitational, Inc.

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

package auth

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// GetDomainName returns local auth domain of the current auth server
// DELETE IN 11.0.0
func (c *Client) GetDomainName(ctx context.Context) (string, error) {
	if resp, err := c.APIClient.GetDomainName(ctx); err != nil {
		if !trace.IsNotImplemented(err) {
			return "", trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.Get(ctx, c.Endpoint("domain"), url.Values{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	var domain string
	if err := json.Unmarshal(out.Bytes(), &domain); err != nil {
		return "", trace.Wrap(err)
	}
	return domain, nil
}

// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster. If
// the cluster has multiple TLS certs, they will all be concatenated.
// DELETE IN 11.0.0
func (c *Client) GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error) {
	if resp, err := c.APIClient.GetClusterCACert(ctx); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}
	out, err := c.Get(context.TODO(), c.Endpoint("cacert"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var localCA deprecatedLocalCAResponse
	if err := json.Unmarshal(out.Bytes(), &localCA); err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.GetClusterCACertResponse{
		TLSCA: localCA.TLSCA,
	}, nil
}

// CreateOIDCAuthRequest creates OIDCAuthRequest.
// DELETE IN 11.0.0
func (c *Client) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	if resp, err := c.APIClient.CreateOIDCAuthRequest(ctx, req); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.PostJSON(ctx, c.Endpoint("oidc", "requests", "create"), createOIDCAuthRequestReq{
		Req: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response *types.OIDCAuthRequest
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// CreateSAMLAuthRequest creates SAMLAuthRequest.
// DELETE IN 11.0.0
func (c *Client) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	if resp, err := c.APIClient.CreateSAMLAuthRequest(ctx, req); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.PostJSON(ctx, c.Endpoint("saml", "requests", "create"), createSAMLAuthRequestReq{
		Req: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response *types.SAMLAuthRequest
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// CreateGithubAuthRequest creates GithubAuthRequest.
// DELETE IN 11.0.0
func (c *Client) CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error) {
	if resp, err := c.APIClient.CreateGithubAuthRequest(ctx, req); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	out, err := c.PostJSON(ctx, c.Endpoint("github", "requests", "create"),
		createGithubAuthRequestReq{Req: req})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response types.GithubAuthRequest
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return &response, nil
}
