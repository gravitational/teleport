/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package k8s

import (
	"go.opentelemetry.io/otel"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/services/k8s")
	log    = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)
)

// WithDefaultCredentialLifetime sets the service's default credential lifetime.
func WithDefaultCredentialLifetime(lifetime bot.CredentialLifetime) DefaultCredentialLifetimeOption {
	return DefaultCredentialLifetimeOption{lifetime}
}

// DefaultCredentialLifetimeOption is returned from WithDefaultCredentialLifetime.
type DefaultCredentialLifetimeOption struct{ lifetime bot.CredentialLifetime }

// WithKubernetesClient sets the service's Kubernetes client. It's used in tests.
func WithKubernetesClient(k8s kubernetes.Interface) KubernetesClientOption {
	return KubernetesClientOption{k8s}
}

// KubernetesClientOption is returned from WithKubernetesClient.
type KubernetesClientOption struct{ client kubernetes.Interface }

// WithInsecure controls whether the service will verify proxy certificates.
func WithInsecure(insecure bool) InsecureOption {
	return InsecureOption{insecure}
}

// InsecureOption is returned from WithInsecure.
type InsecureOption struct{ insecure bool }

// WithALPNUpgradeCache sets the service's ALPN upgrade cache so that it can be
// shared with other services.
func WithALPNUpgradeCache(cache *internal.ALPNUpgradeCache) ALPNUpgradeCacheOption {
	return ALPNUpgradeCacheOption{cache}
}

// ALPNUpgradeCacheOption is returned from WithALPNUpgradeCache.
type ALPNUpgradeCacheOption struct{ cache *internal.ALPNUpgradeCache }
