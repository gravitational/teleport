// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linuxdesktop

import (
	"context"

	"github.com/gravitational/trace"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
)

// Client is a client for the Linux Desktop API.
type Client struct {
	grpcClient linuxdesktopv1.LinuxDesktopServiceClient
}

// NewClient creates a new Linux Desktop client.
func NewClient(grpcClient linuxdesktopv1.LinuxDesktopServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListLinuxDesktops returns a list of Linux Desktops.
func (c *Client) ListLinuxDesktops(ctx context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error) {
	req := &linuxdesktopv1.ListLinuxDesktopsRequest{}
	req.SetPageSize(int32(pageSize))
	req.SetPageToken(nextToken)
	resp, err := c.grpcClient.ListLinuxDesktops(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.GetLinuxDesktops(), resp.GetNextPageToken(), nil
}

// CreateLinuxDesktop creates a new Linux Desktop.
func (c *Client) CreateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r := &linuxdesktopv1.CreateLinuxDesktopRequest{}
	r.SetLinuxDesktop(req)
	rsp, err := c.grpcClient.CreateLinuxDesktop(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// GetLinuxDesktop returns a Linux Desktop by name.
func (c *Client) GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error) {
	req := &linuxdesktopv1.GetLinuxDesktopRequest{}
	req.SetName(name)
	rsp, err := c.grpcClient.GetLinuxDesktop(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpdateLinuxDesktop updates an existing Linux Desktop.
func (c *Client) UpdateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r := &linuxdesktopv1.UpdateLinuxDesktopRequest{}
	r.SetLinuxDesktop(req)
	rsp, err := c.grpcClient.UpdateLinuxDesktop(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpsertLinuxDesktop updates an existing Linux Desktop or creates one.
func (c *Client) UpsertLinuxDesktop(ctx context.Context, req *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	r := &linuxdesktopv1.UpsertLinuxDesktopRequest{}
	r.SetLinuxDesktop(req)
	rsp, err := c.grpcClient.UpsertLinuxDesktop(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// DeleteLinuxDesktop deletes a Linux Desktop.
func (c *Client) DeleteLinuxDesktop(ctx context.Context, name string) error {
	req := &linuxdesktopv1.DeleteLinuxDesktopRequest{}
	req.SetName(name)
	_, err := c.grpcClient.DeleteLinuxDesktop(ctx, req)
	return trace.Wrap(err)
}
