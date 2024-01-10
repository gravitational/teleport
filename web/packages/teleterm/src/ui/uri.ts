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

import { matchPath, generatePath } from 'react-router';

import type { RouteProps } from 'react-router';

/*
 * Resource URIs
 * These are for identifying a specific resource within a root cluster.
 */

type RootClusterId = string;
type LeafClusterId = string;
type ServerId = string;
type KubeId = string;
type DbId = string;
type AppId = string;
export type RootClusterUri = `/clusters/${RootClusterId}`;
export type RootClusterServerUri =
  `/clusters/${RootClusterId}/servers/${ServerId}`;
export type RootClusterKubeUri = `/clusters/${RootClusterId}/kubes/${KubeId}`;
export type RootClusterDatabaseUri = `/clusters/${RootClusterId}/dbs/${DbId}`;
export type RootClusterAppUri = `/clusters/${RootClusterId}/apps/${AppId}`;
export type RootClusterResourceUri =
  | RootClusterServerUri
  | RootClusterKubeUri
  | RootClusterDatabaseUri
  | RootClusterAppUri;
export type RootClusterOrResourceUri = RootClusterUri | RootClusterResourceUri;
export type LeafClusterUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}`;
export type LeafClusterServerUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/servers/${ServerId}`;
export type LeafClusterKubeUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/kubes/${KubeId}`;
export type LeafClusterDatabaseUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/dbs/${DbId}`;
export type LeafClusterAppUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/apps/${AppId}`;
export type LeafClusterResourceUri =
  | LeafClusterServerUri
  | LeafClusterKubeUri
  | LeafClusterDatabaseUri
  | LeafClusterAppUri;
export type LeafClusterOrResourceUri = LeafClusterUri | LeafClusterResourceUri;

export type ResourceUri = RootClusterResourceUri | LeafClusterResourceUri;
export type ClusterUri = RootClusterUri | LeafClusterUri;
export type ServerUri = RootClusterServerUri | LeafClusterServerUri;
export type KubeUri = RootClusterKubeUri | LeafClusterKubeUri;
export type AppUri = RootClusterAppUri | LeafClusterAppUri;
export type DatabaseUri = RootClusterDatabaseUri | LeafClusterDatabaseUri;
export type ClusterOrResourceUri = ResourceUri | ClusterUri;
export type GatewayTargetUri = DatabaseUri | KubeUri | AppUri;

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

type GatewayId = string;
export type GatewayUri = `/gateways/${GatewayId}`;

export const paths = {
  // Resources.
  rootCluster: '/clusters/:rootClusterId',
  leafCluster: '/clusters/:rootClusterId/leaves/:leafClusterId',
  server:
    '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/servers/:serverId',
  serverLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/servers/:serverId',
  kube: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/kubes/:kubeId',
  db: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/dbs/:dbId',
  app: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/apps/:appId',
  // Documents.
  docHome: '/docs/home',
  doc: '/docs/:docId',
  // Gateways.
  gateway: '/gateways/:gatewayId',
};

export const routing = {
  parseClusterUri(uri: string) {
    const leafMatch = routing.parseUri(uri, paths.leafCluster);
    const rootMatch = routing.parseUri(uri, paths.rootCluster);
    return leafMatch || rootMatch;
  },

  // Pass either a root or a leaf cluster URI to get back a root cluster URI.
  ensureRootClusterUri(uri: ClusterOrResourceUri) {
    const { rootClusterId } = routing.parseClusterUri(uri).params;
    return routing.getClusterUri({ rootClusterId }) as RootClusterUri;
  },

  // Pass any resource URI to get back a cluster URI.
  ensureClusterUri(uri: ClusterOrResourceUri) {
    const params = routing.parseClusterUri(uri).params;
    return routing.getClusterUri(params);
  },

  parseKubeUri(uri: string) {
    return routing.parseUri(uri, paths.kube);
  },

  parseAppUri(uri: string) {
    return routing.parseUri(uri, paths.app);
  },

  parseServerUri(uri: string) {
    return routing.parseUri(uri, paths.server);
  },

  parseDbUri(uri: string) {
    return routing.parseUri(uri, paths.db);
  },

  parseUri(path: string, route: string | RouteProps) {
    return matchPath<Params>(path, route);
  },

  /**
   * parseClusterName should be used only when getting the cluster object from ClustersService is
   * not possible.
   *
   * rootClusterId in the URI is not the name of the cluster but rather just the hostname of the
   * proxy. These two might be different.
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
      // paths.serverLeaf is needed as path-to-regexp used by react-router doesn't support
      // optional groups with params. https://github.com/pillarjs/path-to-regexp/issues/142
      //
      // If we used paths.server instead, then the /leaves/ part of the URI would be missing.
      return generatePath(
        paths.serverLeaf,
        params as any
      ) as LeafClusterServerUri;
    } else {
      return generatePath(paths.server, params as any) as RootClusterServerUri;
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

export type Params = {
  rootClusterId?: string;
  leafClusterId?: string;
  serverId?: string;
  kubeId?: string;
  dbId?: string;
  gatewayId?: string;
  tabId?: string;
  sid?: string;
  docId?: string;
  appId?: string;
};
