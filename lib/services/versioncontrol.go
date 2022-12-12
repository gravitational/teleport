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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// VersionControl describes the methods of the version control system, which is used to
// configure and enforce installation of specific teleport versions across a cluster.
type VersionControl interface {
	// GetVersionControlInstallers loads version control installers, sorted by type.
	GetVersionControlInstallers(ctx context.Context, filter types.VersionControlInstallerFilter) (types.VersionControlInstallerSet, error)

	// UpsertVersionControlInstaller creates or updates a version control installer (nonce safety is
	// enforced).
	UpsertVersionControlInstaller(ctx context.Context, installer types.VersionControlInstaller) error

	// DeleteVersionControlInstaller deletes a single version control installer if it matches the supplied
	// filter. Filters that match multiple installers are rejected.
	DeleteVersionControlInstaller(ctx context.Context, filter types.VersionControlInstallerFilter) error

	// GetVersionDirectives gets one or more version directives, sorted by state (draft|pending|active).
	GetVersionDirectives(ctx context.Context, filter types.VersionDirectiveFilter) (types.VersionDirectiveSet, error)

	// UpsertVersionDirective creates or updates a draft phase version directive.
	UpsertVersionDirective(ctx context.Context, directive types.VersionDirective) error

	// DeleteVersionDirective deletes a single version directive if it matches the supplied
	// filter. Filters that match multiple directives are rejected.
	DeleteVersionDirective(ctx context.Context, filter types.VersionDirectiveFilter) error

	// PromoteVersionDirective attempts to promote a version directive (allowed phase transitions
	// are draft -> pending, and pending -> active).
	PromoteVersionDirective(ctx context.Context, req proto.PromoteVersionDirectiveRequest) (proto.PromoteVersionDirectiveResponse, error)

	// SetVersionDirectiveStatus attempts to update the status of a version directive.
	SetVersionDirectiveStatus(ctx context.Context, req proto.SetVersionDirectiveStatusRequest) (proto.SetVersionDirectiveStatusResponse, error)
}
