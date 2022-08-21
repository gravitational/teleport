/*
Copyright 2017-2019 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"
)

// ClusterConfiguration stores the cluster configuration in the backend. All
// the resources modified by this interface can only have a single instance
// in the backend.
type ClusterConfiguration interface {
	// SetClusterName gets services.ClusterName from the backend.
	GetClusterName(opts ...MarshalOption) (types.ClusterName, error)
	// SetClusterName sets services.ClusterName on the backend.
	SetClusterName(types.ClusterName) error
	// UpsertClusterName upserts cluster name
	UpsertClusterName(types.ClusterName) error

	// DeleteClusterName deletes cluster name resource
	DeleteClusterName() error

	// GetStaticTokens gets services.StaticTokens from the backend.
	GetStaticTokens() (types.StaticTokens, error)
	// SetStaticTokens sets services.StaticTokens on the backend.
	SetStaticTokens(types.StaticTokens) error
	// DeleteStaticTokens deletes static tokens resource
	DeleteStaticTokens() error

	// GetAuthPreference gets types.AuthPreference from the backend.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
	// SetAuthPreference sets types.AuthPreference from the backend.
	SetAuthPreference(context.Context, types.AuthPreference) error
	// DeleteAuthPreference deletes types.AuthPreference from the backend.
	DeleteAuthPreference(ctx context.Context) error

	// GetSessionRecordingConfig gets SessionRecordingConfig from the backend.
	GetSessionRecordingConfig(context.Context, ...MarshalOption) (types.SessionRecordingConfig, error)
	// SetSessionRecordingConfig sets SessionRecordingConfig from the backend.
	SetSessionRecordingConfig(context.Context, types.SessionRecordingConfig) error
	// DeleteSessionRecordingConfig deletes SessionRecordingConfig from the backend.
	DeleteSessionRecordingConfig(ctx context.Context) error

	// GetClusterAuditConfig gets ClusterAuditConfig from the backend.
	GetClusterAuditConfig(context.Context, ...MarshalOption) (types.ClusterAuditConfig, error)
	// SetClusterAuditConfig sets ClusterAuditConfig from the backend.
	SetClusterAuditConfig(context.Context, types.ClusterAuditConfig) error
	// DeleteClusterAuditConfig deletes ClusterAuditConfig from the backend.
	DeleteClusterAuditConfig(ctx context.Context) error

	// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the backend.
	GetClusterNetworkingConfig(context.Context, ...MarshalOption) (types.ClusterNetworkingConfig, error)
	// SetClusterNetworkingConfig sets ClusterNetworkingConfig from the backend.
	SetClusterNetworkingConfig(context.Context, types.ClusterNetworkingConfig) error
	// DeleteClusterNetworkingConfig deletes ClusterNetworkingConfig from the backend.
	DeleteClusterNetworkingConfig(ctx context.Context) error

	// GetInstaller gets the installer script from the backend
	GetInstaller(context.Context) (types.Installer, error)
	// SetInstaller sets the installer script in the backend
	SetInstaller(context.Context, types.Installer) error
	// DeleteInstaller removes the installer script from the backend
	DeleteInstaller(context.Context) error
}
