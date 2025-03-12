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

import { generatePath, matchPath, type RouteProps } from 'react-router';

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
export type RootClusterResourceUri =
  | RootClusterServerUri
  | RootClusterKubeUri
  | RootClusterDatabaseUri
  | RootClusterAppUri;
export type RootClusterOrResourceUri = RootClusterUri | RootClusterResourceUri;
export type LeafClusterUri = string;
export type LeafClusterServerUri = string;
export type LeafClusterKubeUri = string;
export type LeafClusterKubeResourceNamespaceUri = string;
export type LeafClusterDatabaseUri = string;
export type LeafClusterAppUri = string;
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
export type KubeResourceNamespaceUri =
  | RootClusterKubeResourceNamespaceUri
  | LeafClusterKubeResourceNamespaceUri;
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

export type GatewayUri = string;

export const paths = {
  // Resources.
  rootCluster: '/clusters/:rootClusterId',
  leafCluster: '/clusters/:rootClusterId/leaves/:leafClusterId',
  server:
    '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/servers/:serverId',
  serverLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/servers/:serverId',
  kube: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/kubes/:kubeId',
  kubeLeaf: '/clusters/:rootClusterId/leaves/:leafClusterId/kubes/:kubeId',
  kubeResourceNamespace:
    '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/kubes/:kubeId/namespaces/:kubeNamespaceId',
  kubeResourceNamespaceLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/kubes/:kubeId/namespaces/:kubeNamespaceId',
  db: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/dbs/:dbId',
  dbLeaf: '/clusters/:rootClusterId/leaves/:leafClusterId/dbs/:dbId',
  app: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/apps/:appId',
  appLeaf: '/clusters/:rootClusterId/leaves/:leafClusterId?/apps/:appId',
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

  ensureKubeUri(uri: KubeResourceNamespaceUri) {
    const { kubeId, rootClusterId, leafClusterId } =
      routing.parseKubeResourceNamespaceUri(uri).params;
    return routing.getKubeUri({ kubeId, rootClusterId, leafClusterId });
  },

  parseKubeUri(uri: string) {
    return routing.parseUri(uri, paths.kube);
  },

  parseKubeResourceNamespaceUri(uri: string) {
    return routing.parseUri(uri, paths.kubeResourceNamespace);
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

  getAppUri(params: Params) {
    if (params.leafClusterId) {
      // paths.appLeaf is needed as path-to-regexp used by react-router doesn't support
      // optional groups with params. https://github.com/pillarjs/path-to-regexp/issues/142
      //
      // If we used paths.server instead, then the /leaves/ part of the URI would be missing.
      return generatePath(paths.appLeaf, params as any) as LeafClusterAppUri;
    } else {
      return generatePath(paths.app, params as any) as RootClusterAppUri;
    }
  },

  getDbUri(params: Params) {
    if (params.leafClusterId) {
      // paths.dbLeaf is needed as path-to-regexp used by react-router doesn't support
      // optional groups with params. https://github.com/pillarjs/path-to-regexp/issues/142
      //
      // If we used paths.server instead, then the /leaves/ part of the URI would be missing.
      return generatePath(
        paths.dbLeaf,
        params as any
      ) as LeafClusterDatabaseUri;
    } else {
      return generatePath(paths.db, params as any) as RootClusterDatabaseUri;
    }
  },

  getKubeUri(params: Params) {
    if (params.leafClusterId) {
      // paths.kubeLeaf is needed as path-to-regexp used by react-router doesn't support
      // optional groups with params. https://github.com/pillarjs/path-to-regexp/issues/142
      //
      // If we used paths.server instead, then the /leaves/ part of the URI would be missing.
      return generatePath(paths.kubeLeaf, params as any) as LeafClusterKubeUri;
    } else {
      return generatePath(paths.kube, params as any) as RootClusterKubeUri;
    }
  },

  getKubeResourceNamespaceUri(params: Params) {
    if (params.leafClusterId) {
      // paths.kubeResourceLeaf is needed as path-to-regexp used by react-router doesn't support
      // optional groups with params. https://github.com/pillarjs/path-to-regexp/issues/142
      //
      // If we used paths.kubeResource instead, then the /leaves/ part of the URI would be missing.
      return generatePath(
        paths.kubeResourceNamespaceLeaf,
        params as any
      ) as LeafClusterKubeResourceNamespaceUri;
    } else {
      return generatePath(
        paths.kubeResourceNamespace,
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
};
