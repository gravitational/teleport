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

// eslint-disable-next-line import/named
import { RouteProps, matchPath, generatePath } from 'react-router';

export const paths = {
  rootCluster: '/clusters/:rootClusterId',
  leafCluster: '/clusters/:rootClusterId/leaves/:leafClusterId',
  server:
    '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/servers/:serverId',
  kube: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/kubes/:kubeId',
  db: '/clusters/:rootClusterId/(leaves)?/:leafClusterId?/dbs/:dbId',
  gateway: '/gateways/:gatewayId',
  docHome: '/docs/home',
  doc: '/docs/:docId',
};

export const routing = {
  parseClusterUri(uri: string) {
    const leafMatch = routing.parseUri(uri, paths.leafCluster);
    const rootMatch = routing.parseUri(uri, paths.rootCluster);
    return leafMatch || rootMatch;
  },

  // Pass either a root or a leaf cluster URI to get back a root cluster URI.
  ensureRootClusterUri(uri: string) {
    const { rootClusterId } = routing.parseClusterUri(uri).params;
    return routing.getClusterUri({ rootClusterId });
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
    return generatePath(paths.doc, params as any);
  },

  getClusterUri(params: Params) {
    if (params.leafClusterId) {
      return generatePath(paths.leafCluster, params as any);
    }

    return generatePath(paths.rootCluster, params as any);
  },

  getServerUri(params: Params) {
    return generatePath(paths.server, params as any);
  },

  isClusterServer(clusterUri: string, serverUri: string) {
    return serverUri.startsWith(`${clusterUri}/servers/`);
  },

  isClusterKube(clusterUri: string, kubeUri: string) {
    return kubeUri.startsWith(`${clusterUri}/kubes/`);
  },

  isClusterDb(clusterUri: string, dbUri: string) {
    return dbUri.startsWith(`${clusterUri}/dbs/`);
  },

  isClusterApp(clusterUri: string, appUri: string) {
    return appUri.startsWith(`${clusterUri}/apps/`);
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
