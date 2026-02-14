/**
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

import { generatePath, matchPath, type PathMatch } from 'react-router';

/*
 * Resource URIs
 * These are for identifying a specific resource within a root cluster.
 */

// TODO(gzdunek): These types used to be template literals
// (for example, RootClusterUri = `/clusters/${RootClusterId}`).
// They were replaced with strings here https://github.com/gravitational/teleport/pull/39828,
// because we started using the generated proto types directly
// (so it was not possible to assign these types to plain strings).
// However, I didn't remove the type aliases below, because:
// 1. Ripping them out is too much work.
// 2. They still carry some useful information.
// 3. We might be able to add them back in the future
// (maybe with some sort of TypeScript declaration merging).
export type RootClusterUri = string;
export type RootClusterServerUri = string;
export type RootClusterKubeUri = string;
export type RootClusterKubeResourceNamespaceUri = string;
export type RootClusterDatabaseUri = string;
export type RootClusterAppUri = string;
export type RootClusterWindowsDesktopUri = string;
export type RootClusterResourceUri =
  | RootClusterServerUri
  | RootClusterKubeUri
  | RootClusterDatabaseUri
  | RootClusterAppUri
  | RootClusterWindowsDesktopUri;
export type RootClusterOrResourceUri = RootClusterUri | RootClusterResourceUri;
export type LeafClusterUri = string;
export type LeafClusterServerUri = string;
export type LeafClusterKubeUri = string;
export type LeafClusterKubeResourceNamespaceUri = string;
export type LeafClusterDatabaseUri = string;
export type LeafClusterAppUri = string;
export type LeafClusterWindowsDesktopUri = string;
export type LeafClusterResourceUri =
  | LeafClusterServerUri
  | LeafClusterKubeUri
  | LeafClusterDatabaseUri
  | LeafClusterAppUri
  | LeafClusterWindowsDesktopUri;
export type LeafClusterOrResourceUri = LeafClusterUri | LeafClusterResourceUri;

export type ResourceUri = RootClusterResourceUri | LeafClusterResourceUri;
export type ClusterUri = RootClusterUri | LeafClusterUri;
export type ServerUri = RootClusterServerUri | LeafClusterServerUri;
export type KubeUri = RootClusterKubeUri | LeafClusterKubeUri;
export type KubeResourceNamespaceUri =
  | RootClusterKubeResourceNamespaceUri
  | LeafClusterKubeResourceNamespaceUri;
export type AppUri = RootClusterAppUri | LeafClusterAppUri;
export type DatabaseUri = RootClusterDatabaseUri | LeafClusterDatabaseUri;
export type WindowsDesktopUri =
  | RootClusterWindowsDesktopUri
  | LeafClusterWindowsDesktopUri;
export type ClusterOrResourceUri = ResourceUri | ClusterUri;
export type GatewayTargetUri = DatabaseUri | KubeUri | AppUri;

/** General type for desktop URI. */
export type DesktopUri = WindowsDesktopUri;

/*
 * Document URIs
 * These are for documents (tabs) within the app.
 */

type DocumentId = string;
export type DocumentUri = `/docs/${DocumentId}`;

/*
 * Gateway URIs
 * These are for gateways (proxies) managed by the tsh daemon.
 */

export type GatewayUri = string;

export const paths = {
  // Resources.
  // react-router path patterns do not support regex segments.
  // We use separate paths for root and leaf clusters and try both when parsing.
  rootCluster: '/clusters/:rootClusterId',
  leafCluster: '/clusters/:rootClusterId/leaves/:leafClusterId',
  // Root cluster resource paths (without /leaves/ segment)
  serverRoot: '/clusters/:rootClusterId/servers/:serverId',
  kubeRoot: '/clusters/:rootClusterId/kubes/:kubeId',
  kubeResourceNamespaceRoot:
    '/clusters/:rootClusterId/kubes/:kubeId/namespaces/:kubeNamespaceId',
  dbRoot: '/clusters/:rootClusterId/dbs/:dbId',
  appRoot: '/clusters/:rootClusterId/apps/:appId',
  windowsDesktopRoot:
    '/clusters/:rootClusterId/windows_desktops/:windowsDesktopId',
  // Leaf cluster resource paths (with /leaves/ segment)
  serverLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/servers/:serverId',
  kubeLeaf: '/clusters/:rootClusterId/leaves/:leafClusterId/kubes/:kubeId',
  kubeResourceNamespaceLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/kubes/:kubeId/namespaces/:kubeNamespaceId',
  dbLeaf: '/clusters/:rootClusterId/leaves/:leafClusterId/dbs/:dbId',
  appLeaf: '/clusters/:rootClusterId/leaves/:leafClusterId/apps/:appId',
  windowsDesktopLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/windows_desktops/:windowsDesktopId',
  // Documents.
  docHome: '/docs/home',
  doc: '/docs/:docId',
  // Gateways.
  gateway: '/gateways/:gatewayId',
};

export const routing = {
  parseClusterUri(uri: string) {
    // Use end: false to match resource URIs that extend beyond the cluster path
    // e.g., /clusters/test/servers/123 should still match and extract rootClusterId
    // Try leaf first (more specific), then root
    const leafMatch = matchPath({ path: paths.leafCluster, end: false }, uri);
    const rootMatch = matchPath({ path: paths.rootCluster, end: false }, uri);
    return leafMatch || rootMatch;
  },

  // Pass either a root or a leaf cluster URI to get back a root cluster URI.
  ensureRootClusterUri(uri: ClusterOrResourceUri) {
    // parseClusterUri returns null if the URI doesn't match a cluster pattern.
    // Keep legacy behavior where invalid input throws on .params access.
    const { rootClusterId } = routing.parseClusterUri(uri)!.params;
    return routing.getClusterUri({ rootClusterId }) as RootClusterUri;
  },

  // Pass any resource URI to get back a cluster URI.
  ensureClusterUri(uri: ClusterOrResourceUri) {
    const { rootClusterId, leafClusterId } =
      routing.parseClusterUri(uri)!.params;
    return routing.getClusterUri({ rootClusterId, leafClusterId });
  },

  ensureKubeUri(uri: KubeResourceNamespaceUri) {
    const { kubeId, rootClusterId, leafClusterId } =
      routing.parseKubeResourceNamespaceUri(uri)!.params;
    return routing.getKubeUri({ kubeId, rootClusterId, leafClusterId });
  },

  parseKubeUri(uri: string) {
    // Try leaf path first (more specific), then root path
    return (
      routing.parseUri(uri, paths.kubeLeaf) ||
      routing.parseUri(uri, paths.kubeRoot)
    );
  },

  parseWindowsDesktopUri(uri: string) {
    return (
      routing.parseUri(uri, paths.windowsDesktopLeaf) ||
      routing.parseUri(uri, paths.windowsDesktopRoot)
    );
  },

  parseKubeResourceNamespaceUri(uri: string) {
    return (
      routing.parseUri(uri, paths.kubeResourceNamespaceLeaf) ||
      routing.parseUri(uri, paths.kubeResourceNamespaceRoot)
    );
  },

  parseAppUri(uri: string) {
    return (
      routing.parseUri(uri, paths.appLeaf) ||
      routing.parseUri(uri, paths.appRoot)
    );
  },

  parseServerUri(uri: string) {
    return (
      routing.parseUri(uri, paths.serverLeaf) ||
      routing.parseUri(uri, paths.serverRoot)
    );
  },

  parseDbUri(uri: string) {
    return (
      routing.parseUri(uri, paths.dbLeaf) || routing.parseUri(uri, paths.dbRoot)
    );
  },

  // matchPath signature is (pattern, pathname).
  parseUri(path: string, route: string): PathMatch<string> | null {
    return matchPath(route, path);
  },

  /**
   * Returns the profile name for root clusters and the cluster name for leaf clusters.
   *
   * In the URI, `rootClusterId` may not be the root cluster's name but the hostname
   * of its proxy (these may differ).
   * `leafClusterId`, on the other hand, always matches the leaf cluster's name.
   *
   * TODO(gzdunek): Split this function into `parseProfileName` and `parseLeafClusterName`.
   */
  parseClusterName(clusterUri: string) {
    const parsed = routing.parseClusterUri(clusterUri);
    if (!parsed) {
      return '';
    }

    if (parsed.params.leafClusterId) {
      return parsed.params.leafClusterId;
    }

    if (parsed.params.rootClusterId) {
      return parsed.params.rootClusterId;
    }

    return '';
  },

  getDocUri(params: Params) {
    return generatePath(paths.doc, params as any) as DocumentUri;
  },

  getClusterUri(params: Params): ClusterUri {
    if (params.leafClusterId) {
      return generatePath(paths.leafCluster, params as any) as LeafClusterUri;
    }

    return generatePath(paths.rootCluster, params as any) as RootClusterUri;
  },

  getServerUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(
        paths.serverLeaf,
        params as any
      ) as LeafClusterServerUri;
    } else {
      return generatePath(
        paths.serverRoot,
        params as any
      ) as RootClusterServerUri;
    }
  },

  getAppUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(paths.appLeaf, params as any) as LeafClusterAppUri;
    } else {
      return generatePath(paths.appRoot, params as any) as RootClusterAppUri;
    }
  },

  getDbUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(
        paths.dbLeaf,
        params as any
      ) as LeafClusterDatabaseUri;
    } else {
      return generatePath(
        paths.dbRoot,
        params as any
      ) as RootClusterDatabaseUri;
    }
  },

  getKubeUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(paths.kubeLeaf, params as any) as LeafClusterKubeUri;
    } else {
      return generatePath(paths.kubeRoot, params as any) as RootClusterKubeUri;
    }
  },

  getWindowsDesktopUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(
        paths.windowsDesktopLeaf,
        params as any
      ) as LeafClusterWindowsDesktopUri;
    } else {
      return generatePath(
        paths.windowsDesktopRoot,
        params as any
      ) as RootClusterWindowsDesktopUri;
    }
  },

  getKubeResourceNamespaceUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(
        paths.kubeResourceNamespaceLeaf,
        params as any
      ) as LeafClusterKubeResourceNamespaceUri;
    } else {
      return generatePath(
        paths.kubeResourceNamespaceRoot,
        params as any
      ) as RootClusterKubeResourceNamespaceUri;
    }
  },

  isClusterServer(clusterUri: ClusterUri, serverUri: ServerUri) {
    return serverUri.startsWith(`${clusterUri}/servers/`);
  },

  isClusterKube(clusterUri: ClusterUri, kubeUri: KubeUri) {
    return kubeUri.startsWith(`${clusterUri}/kubes/`);
  },

  isClusterDb(clusterUri: ClusterUri, dbUri: DatabaseUri) {
    return dbUri.startsWith(`${clusterUri}/dbs/`);
  },

  isClusterApp(clusterUri: ClusterUri, appUri: string) {
    return appUri.startsWith(`${clusterUri}/apps/`);
  },

  isLeafCluster(clusterUri: ClusterUri) {
    const match = routing.parseClusterUri(clusterUri);
    return match && Boolean(match.params.leafClusterId);
  },

  isRootCluster(clusterUri: ClusterUri) {
    return !routing.isLeafCluster(clusterUri);
  },

  belongsToProfile(
    clusterUri: ClusterOrResourceUri,
    resourceUri: ClusterOrResourceUri
  ) {
    const rootClusterUri = routing.ensureRootClusterUri(clusterUri);
    const resourceRootClusterUri = routing.ensureRootClusterUri(resourceUri);

    return resourceRootClusterUri === rootClusterUri;
  },

  belongsToKube(
    kubeClusterUri: KubeUri,
    namespaceUri: KubeResourceNamespaceUri
  ) {
    const kubeUri = routing.ensureKubeUri(namespaceUri);
    return kubeUri === kubeClusterUri;
  },
};

export function isAppUri(uri: string): uri is AppUri {
  return !!routing.parseAppUri(uri);
}

export function isDatabaseUri(uri: string): uri is DatabaseUri {
  return !!routing.parseDbUri(uri);
}

export function isServerUri(uri: string): uri is ServerUri {
  return !!routing.parseServerUri(uri);
}

export function isKubeUri(uri: string): uri is KubeUri {
  return !!routing.parseKubeUri(uri);
}

export function isWindowsDesktopUri(uri: string): uri is WindowsDesktopUri {
  return !!routing.parseWindowsDesktopUri(uri);
}

export type Params = {
  rootClusterId?: string;
  leafClusterId?: string;
  serverId?: string;
  kubeId?: string;
  kubeNamespaceId?: string;
  dbId?: string;
  gatewayId?: string;
  tabId?: string;
  sid?: string;
  docId?: string;
  appId?: string;
  windowsDesktopId?: string;
};
