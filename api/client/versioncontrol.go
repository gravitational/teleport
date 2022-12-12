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

package client

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
)

// GetVersionControlInstallers loads version control installers, sorted by type.
func (c *Client) GetVersionControlInstallers(ctx context.Context, filter types.VersionControlInstallerFilter) (types.VersionControlInstallerSet, error) {
	set, err := c.grpc.GetVersionControlInstallers(ctx, &filter, c.callOpts...)
	if err != nil {
		return types.VersionControlInstallerSet{}, trail.FromGRPC(err)
	}

	return *set, nil
}

// UpsertVersionControlInstaller creates or updates a version control installer (nonce safety is
// enforced).
func (c *Client) UpsertVersionControlInstaller(ctx context.Context, installer types.VersionControlInstaller) error {
	var msg proto.VersionControlInstallerOneOf
	switch i := installer.(type) {
	case *types.LocalScriptInstallerV1:
		msg.Installer = &proto.VersionControlInstallerOneOf_LocalScript{
			LocalScript: i,
		}
	default:
		return trace.BadParameter("unexpected installer type %T", installer)
	}
	_, err := c.grpc.UpsertVersionControlInstaller(ctx, &msg, c.callOpts...)
	return trail.FromGRPC(err)
}

// DeleteVersionControlInstaller deletes a single version control installer if it matches the supplied
// filter. Filters that match multiple installers are rejected.
func (c *Client) DeleteVersionControlInstaller(ctx context.Context, filter types.VersionControlInstallerFilter) error {
	_, err := c.grpc.DeleteVersionControlInstaller(ctx, &filter, c.callOpts...)
	return trail.FromGRPC(err)
}

// GetVersionDirectives gets one or more version directives, sorted by state (draft|pending|active).
func (c *Client) GetVersionDirectives(ctx context.Context, filter types.VersionDirectiveFilter) (types.VersionDirectiveSet, error) {
	set, err := c.grpc.GetVersionDirectives(ctx, &filter, c.callOpts...)
	if err != nil {
		return types.VersionDirectiveSet{}, trail.FromGRPC(err)
	}

	return *set, nil
}

// UpsertVersionDirective creates or updates a draft phase version directive.
func (c *Client) UpsertVersionDirective(ctx context.Context, directive types.VersionDirective) error {
	d, ok := directive.(*types.VersionDirectiveV1)
	if !ok {
		return trace.BadParameter("unexpected directive type %T", directive)
	}
	_, err := c.grpc.UpsertVersionDirective(ctx, d, c.callOpts...)
	return trail.FromGRPC(err)
}

// DeleteVersionDirective deletes a single version directive if it matches the supplied
// filter. Filters that match multiple directives are rejected.
func (c *Client) DeleteVersionDirective(ctx context.Context, filter types.VersionDirectiveFilter) error {
	_, err := c.grpc.DeleteVersionDirective(ctx, &filter, c.callOpts...)
	return trail.FromGRPC(err)
}

// PromoteVersionDirective attempts to promote a version directive (allowed phase transitions
// are draft -> pending, and pending -> active).
func (c *Client) PromoteVersionDirective(ctx context.Context, req proto.PromoteVersionDirectiveRequest) (proto.PromoteVersionDirectiveResponse, error) {
	rsp, err := c.grpc.PromoteVersionDirective(ctx, &req, c.callOpts...)
	if err != nil {
		return proto.PromoteVersionDirectiveResponse{}, trail.FromGRPC(err)
	}
	return *rsp, nil
}

// SetVersionDirectiveStatus attempts to update the status of a version directive.
func (c *Client) SetVersionDirectiveStatus(ctx context.Context, req proto.SetVersionDirectiveStatusRequest) (proto.SetVersionDirectiveStatusResponse, error) {
	rsp, err := c.grpc.SetVersionDirectiveStatus(ctx, &req, c.callOpts...)
	if err != nil {
		return proto.SetVersionDirectiveStatusResponse{}, trail.FromGRPC(err)
	}
	return *rsp, nil
}
