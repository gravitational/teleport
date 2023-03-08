/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { Cluster, LoggedInUser } from 'teleterm/services/tshd/types';
import { RootClusterUri } from 'teleterm/ui/uri';

export function useIdentity() {
  const ctx = useAppContext();

  ctx.clustersService.useState();
  ctx.workspacesService.useState();

  function changeRootCluster(clusterUri: RootClusterUri): Promise<void> {
    return ctx.workspacesService.setActiveWorkspace(clusterUri);
  }

  function addCluster(): void {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

  function logout(clusterUri: RootClusterUri): void {
    ctx.commandLauncher.executeCommand('cluster-logout', { clusterUri });
  }

  function getActiveRootCluster(): Cluster | undefined {
    const clusterUri = ctx.workspacesService.getRootClusterUri();
    if (!clusterUri) {
      return;
    }
    return ctx.clustersService.findCluster(clusterUri);
  }

  function getLoggedInUser(): LoggedInUser | undefined {
    const clusterUri = ctx.workspacesService.getRootClusterUri();
    if (!clusterUri) {
      return;
    }
    const cluster = ctx.clustersService.findCluster(clusterUri);
    if (!cluster) {
      return;
    }
    return cluster.loggedInUser;
  }

  const rootClusters: IdentityRootCluster[] = ctx.clustersService
    .getClusters()
    .filter(c => !c.leaf)
    .map(cluster => ({
      active: cluster.uri === ctx.workspacesService.getRootClusterUri(),
      clusterName: cluster.name,
      userName: cluster.loggedInUser?.name,
      uri: cluster.uri,
      connected: cluster.connected,
    }));

  return {
    changeRootCluster,
    addCluster,
    logout,
    loggedInUser: getLoggedInUser(),
    activeRootCluster: getActiveRootCluster(),
    rootClusters,
  };
}

export interface IdentityRootCluster {
  active: boolean;
  clusterName: string;
  userName: string;
  uri: RootClusterUri;
  connected: boolean;
}
