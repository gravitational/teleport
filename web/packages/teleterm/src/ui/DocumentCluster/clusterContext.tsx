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

import React from 'react';
import { matchPath } from 'react-router';
import { useStore, Store } from 'shared/libs/stores';

import { IAppContext } from 'teleterm/ui/types';
import { ClusterUri, DocumentUri, KubeUri, routing } from 'teleterm/ui/uri';
import { connectToKube } from 'teleterm/ui/services/workspacesService';

import type * as tsh from 'teleterm/services/tshd/types';

type State = {
  navLocation: NavLocation;
  clusterName: string;
  leaf: boolean;
  leafConnected: boolean;
  status: 'requires_login' | 'not_found' | '';
  statusText: string;
};

class ClusterContext extends Store<State> {
  private _cluster: tsh.Cluster;

  readonly appCtx: IAppContext;
  readonly clusterUri: ClusterUri;
  readonly documentUri: DocumentUri;
  readonly state: State = {
    navLocation: '/resources/servers',
    clusterName: '',
    leaf: false,
    leafConnected: false,
    status: '',
    statusText: '',
  };

  constructor(
    appCtx: IAppContext,
    clusterUri: ClusterUri,
    documentUri: DocumentUri
  ) {
    super();
    this.clusterUri = clusterUri;
    this.documentUri = documentUri;
    this.appCtx = appCtx;
    this.appCtx.clustersService.subscribe(this.refresh);
    this.state.clusterName = routing.parseClusterName(clusterUri);
    this.refresh();
  }

  login = () => {
    const rootCluster = this.appCtx.clustersService.findRootClusterByResource(
      this.clusterUri
    );

    this.appCtx.commandLauncher.executeCommand('cluster-connect', {
      clusterUri: rootCluster?.uri,
    });
  };

  connectKube = (kubeUri: KubeUri) => {
    connectToKube(this.appCtx, { uri: kubeUri });
  };

  refresh = () => {
    const rootCluster = this.appCtx.clustersService.findRootClusterByResource(
      this.clusterUri
    );
    const cluster = this.appCtx.clustersService.findCluster(this.clusterUri);

    if (!rootCluster) {
      this.state.status = 'not_found';
      this.state.statusText = `cluster ${this.clusterUri} is not found'`;
      this.setState(this.state);
      return;
    }

    if (!rootCluster.connected) {
      this.state.status = 'requires_login';
      this.setState(this.state);
      return;
    }

    if (!cluster) {
      this.state.status = 'not_found';
      this.state.statusText = `cluster ${this.clusterUri} is not found'`;
      this.setState(this.state);
      return;
    }

    if (cluster === this._cluster) {
      return;
    }

    this._cluster = cluster;
    this.state.status = '';
    this.state.clusterName = cluster.name;
    this.state.leaf = cluster.leaf;
    this.state.leafConnected = cluster.leaf && cluster.connected;
    this.setState(this.state);
  };

  dispose() {
    this.appCtx.clustersService.unsubscribe(this.refresh);
  }

  isLocationActive(location: NavLocation, exact = false) {
    return Boolean(
      matchPath(this.state.navLocation, {
        path: location,
        exact,
      })
    );
  }

  changeLocation(navLocation: NavLocation) {
    this.setState({
      navLocation,
    });
  }

  useState(): Readonly<State> {
    return useStore(this).state;
  }
}

const ClusterReactContext = React.createContext<ClusterContext>(null);

const ClusterContextProvider: React.FC<{ value: ClusterContext }> = props => {
  return <ClusterReactContext.Provider {...props} />;
};

const useClusterContext = () => {
  return React.useContext(ClusterReactContext);
};

export type NavLocation =
  | '/resources/'
  | '/resources/databases'
  | '/resources/servers'
  | '/resources/apps'
  | '/resources/kubes';

export default ClusterContext;

export { ClusterContextProvider, ClusterReactContext, useClusterContext };
