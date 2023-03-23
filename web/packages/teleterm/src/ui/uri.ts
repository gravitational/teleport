/*
Copyright 2019 Gravitational, Inc.

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

import { matchPath, generatePath } from 'react-router';

import type { RouteProps } from 'react-router';

export const paths = {
  rootCluster: '/clusters/:rootClusterId',
  leafCluster: '/clusters/:rootClusterId/leaves/:leafClusterId',
  server:
    '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/servers/:serverId',
  serverLeaf:
    '/clusters/:rootClusterId/leaves/:leafClusterId/servers/:serverId',
  kube: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/kubes/:kubeId',
  db: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/dbs/:dbId',
  gateway: '/gateways/:gatewayId',
  docHome: '/docs/home',
  doc: '/docs/:docId',
};

type RootClusterId = string;
type LeafClusterId = string;
type ServerId = string;
type KubeId = string;
type DbId = string;
export type RootClusterUri = `/clusters/${RootClusterId}`;
export type RootClusterServerUri =
  `/clusters/${RootClusterId}/servers/${ServerId}`;
export type RootClusterKubeUri = `/clusters/${RootClusterId}/kubes/${KubeId}`;
export type RootClusterDatabaseUri = `/clusters/${RootClusterId}/dbs/${DbId}`;
export type RootClusterResourceUri =
  | RootClusterServerUri
  | RootClusterKubeUri
  | RootClusterDatabaseUri;
export type RootClusterOrResourceUri = RootClusterUri | RootClusterResourceUri;
export type LeafClusterUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}`;
export type LeafClusterServerUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/servers/${ServerId}`;
export type LeafClusterKubeUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/kubes/${KubeId}`;
export type LeafClusterDatabaseUri =
  `/clusters/${RootClusterId}/leaves/${LeafClusterId}/dbs/${DbId}`;
export type LeafClusterResourceUri =
  | LeafClusterServerUri
  | LeafClusterKubeUri
  | LeafClusterDatabaseUri;
export type LeafClusterOrResourceUri = LeafClusterUri | LeafClusterResourceUri;

export type ResourceUri = RootClusterResourceUri | LeafClusterResourceUri;
export type ClusterUri = RootClusterUri | LeafClusterUri;
export type ServerUri = RootClusterServerUri | LeafClusterServerUri;
export type KubeUri = RootClusterKubeUri | LeafClusterKubeUri;
export type DatabaseUri = RootClusterDatabaseUri | LeafClusterDatabaseUri;
export type ClusterOrResourceUri = ResourceUri | ClusterUri;

type DocumentId = string;
export type DocumentUri = `/docs/${DocumentId}`;

type GatewayId = string;
export type GatewayUri = `/gateways/${GatewayId}`;

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

  parseServerUri(uri: string) {
    return routing.parseUri(uri, paths.server);
  },

  parseDbUri(uri: string) {
    return routing.parseUri(uri, paths.db);
  },

  parseUri(path: string, route: string | RouteProps) {
    return matchPath<Params>(path, route);
  },

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

  belongsToProfile(
    clusterUri: ClusterOrResourceUri,
    resourceUri: ClusterOrResourceUri
  ) {
    const rootClusterUri = this.ensureRootClusterUri(clusterUri);
    const resourceRootClusterUri = this.ensureRootClusterUri(resourceUri);

    return resourceRootClusterUri === rootClusterUri;
  },
};

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
};
