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

package servicecfg

import (
	"context"

	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeConfig specifies configuration for Teleport's Kubernetes service
type KubeConfig struct {
	// Enabled turns kubernetes service role on or off for this process
	Enabled bool

	// ListenAddr is the address to listen on for incoming kubernetes requests.
	// Optional.
	ListenAddr *utils.NetAddr

	// PublicAddrs is a list of the public addresses the Teleport kubernetes
	// service can be reached by the proxy service.
	PublicAddrs []utils.NetAddr

	// KubeClusterName is the name of a kubernetes cluster this proxy is running
	// in. If empty, defaults to the Teleport cluster name.
	KubeClusterName string

	// KubeconfigPath is a path to kubeconfig
	KubeconfigPath string

	// Labels are used for RBAC on clusters.
	StaticLabels  map[string]string
	DynamicLabels services.CommandLabels

	// Limiter limits the connection and request rates.
	Limiter limiter.Config

	// CheckImpersonationPermissions is an optional override to the default
	// impersonation permissions check, for use in testing.
	CheckImpersonationPermissions ImpersonationPermissionsChecker

	// ResourceMatchers match dynamic kube_cluster resources.
	ResourceMatchers []services.ResourceMatcher
}

// ImpersonationPermissionsChecker describes a function that can be used to check
// for the required impersonation permissions on a Kubernetes cluster. Return nil
// to indicate success.
type ImpersonationPermissionsChecker func(ctx context.Context, clusterName string,
	sarClient authztypes.SelfSubjectAccessReviewInterface) error
