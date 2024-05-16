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

package auth

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// Announcer specifies interface responsible for announcing presence
type Announcer = authclient.Announcer

// ReadNodeAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentNode.
//
// NOTE: This interface must match the resources replicated in cache.ForNode.
type ReadNodeAccessPoint = authclient.ReadNodeAccessPoint

// NodeAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by teleport.ComponentNode.
type NodeAccessPoint = authclient.NodeAccessPoint

// ReadProxyAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
//
// NOTE: This interface must match the resources replicated in cache.ForProxy.
type ReadProxyAccessPoint = authclient.ReadProxyAccessPoint

// SnowflakeSessionWatcher is watcher interface used by Snowflake web session watcher.
type SnowflakeSessionWatcher = authclient.SnowflakeSessionWatcher

// ProxyAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
type ProxyAccessPoint = authclient.ProxyAccessPoint

// ReadRemoteProxyAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
//
// NOTE: This interface must match the resources replicated in cache.ForRemoteProxy.
type ReadRemoteProxyAccessPoint = authclient.ReadRemoteProxyAccessPoint

// RemoteProxyAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentProxy.
type RemoteProxyAccessPoint = authclient.RemoteProxyAccessPoint

// ReadKubernetesAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentKube.
//
// NOTE: This interface must match the resources replicated in cache.ForKubernetes.
type ReadKubernetesAccessPoint = authclient.ReadKubernetesAccessPoint

// KubernetesAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentKube.
type KubernetesAccessPoint = authclient.KubernetesAccessPoint

// ReadAppsAccessPoint is a read only API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentApp.
//
// NOTE: This interface must match the resources replicated in cache.ForApps.
type ReadAppsAccessPoint = authclient.ReadAppsAccessPoint

// AppsAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentApp.
type AppsAccessPoint = authclient.AppsAccessPoint

// ReadDatabaseAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentDatabase.
//
// NOTE: This interface must match the resources replicated in cache.ForDatabases.
type ReadDatabaseAccessPoint = authclient.ReadDatabaseAccessPoint

// DatabaseAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentDatabase.
type DatabaseAccessPoint = authclient.DatabaseAccessPoint

// ReadWindowsDesktopAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentWindowsDesktop.
//
// NOTE: This interface must match the resources replicated in cache.ForWindowsDesktop.
type ReadWindowsDesktopAccessPoint = authclient.ReadWindowsDesktopAccessPoint

// WindowsDesktopAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentWindowsDesktop.
type WindowsDesktopAccessPoint = authclient.WindowsDesktopAccessPoint

// ReadDiscoveryAccessPoint is a read only API interface to be
// used by a teleport.ComponentDiscovery.
//
// NOTE: This interface must match the resources replicated in cache.ForDiscovery.
type ReadDiscoveryAccessPoint = authclient.ReadDiscoveryAccessPoint

// DiscoveryAccessPoint is an API interface implemented by a certificate authority (CA) to be
// used by a teleport.ComponentDiscovery
type DiscoveryAccessPoint = authclient.DiscoveryAccessPoint

// ReadOktaAccessPoint is a read only API interface to be
// used by an Okta component.
//
// NOTE: This interface must provide read interfaces for the [types.WatchKind] registered in [cache.ForOkta].
type ReadOktaAccessPoint = authclient.ReadOktaAccessPoint

// OktaAccessPoint is a read caching interface used by an Okta component.
type OktaAccessPoint = authclient.OktaAccessPoint

// AccessCache is a subset of the interface working on the certificate authorities
type AccessCache = authclient.AccessCache

// AccessCacheWithEvents extends the AccessCache interface with events. Useful for trust-related components
// that need to watch for changes.
type AccessCacheWithEvents interface {
	AccessCache
	types.Events
}

// Cache is a subset of the auth interface handling
// access to the discovery API and static tokens
type Cache = authclient.Cache

type NodeWrapper = authclient.NodeWrapper

func NewNodeWrapper(base NodeAccessPoint, cache ReadNodeAccessPoint) NodeAccessPoint {
	return authclient.NewNodeWrapper(base, cache)
}

type ProxyWrapper = authclient.ProxyWrapper

func NewProxyWrapper(base ProxyAccessPoint, cache ReadProxyAccessPoint) ProxyAccessPoint {
	return authclient.NewProxyWrapper(base, cache)
}

type RemoteProxyWrapper = authclient.RemoteProxyWrapper

func NewRemoteProxyWrapper(base RemoteProxyAccessPoint, cache ReadRemoteProxyAccessPoint) RemoteProxyAccessPoint {
	return authclient.NewRemoteProxyWrapper(base, cache)
}

type KubernetesWrapper = authclient.KubernetesWrapper

func NewKubernetesWrapper(base KubernetesAccessPoint, cache ReadKubernetesAccessPoint) KubernetesAccessPoint {
	return authclient.NewKubernetesWrapper(base, cache)
}

type DatabaseWrapper = authclient.DatabaseWrapper

func NewDatabaseWrapper(base DatabaseAccessPoint, cache ReadDatabaseAccessPoint) DatabaseAccessPoint {
	return authclient.NewDatabaseWrapper(base, cache)
}

type AppsWrapper = authclient.AppsWrapper

func NewAppsWrapper(base AppsAccessPoint, cache ReadAppsAccessPoint) AppsAccessPoint {
	return authclient.NewAppsWrapper(base, cache)
}

type WindowsDesktopWrapper = authclient.WindowsDesktopWrapper

func NewWindowsDesktopWrapper(base WindowsDesktopAccessPoint, cache ReadWindowsDesktopAccessPoint) WindowsDesktopAccessPoint {
	return authclient.NewWindowsDesktopWrapper(base, cache)
}

type DiscoveryWrapper = authclient.DiscoveryWrapper

func NewDiscoveryWrapper(base DiscoveryAccessPoint, cache ReadDiscoveryAccessPoint) DiscoveryAccessPoint {
	return authclient.NewDiscoveryWrapper(base, cache)
}

type OktaWrapper = authclient.OktaWrapper

func NewOktaWrapper(base OktaAccessPoint, cache ReadOktaAccessPoint) OktaAccessPoint {
	return authclient.NewOktaWrapper(base, cache)
}

// NewRemoteProxyCachingAccessPoint returns new caching access point using
// access point policy
type NewRemoteProxyCachingAccessPoint = authclient.NewRemoteProxyCachingAccessPoint
