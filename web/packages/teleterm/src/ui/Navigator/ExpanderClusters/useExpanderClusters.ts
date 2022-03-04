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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import AppContext from 'teleterm/ui/appContext';
import { ExpanderClusterState, ClusterNavItem } from './types';

export function useExpanderClusters(): ExpanderClusterState {
  const ctx = useAppContext();
  const items = initItems(ctx);

  // subscribe
  ctx.clustersService.useState();

  function onAddCluster() {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

  function onSyncClusters() {
    ctx.clustersService.syncRootClusters();
  }

  function onLogin(clusterUri: string) {
    ctx.commandLauncher.executeCommand('cluster-connect', { clusterUri });
  }

  function onLogout(clusterUri: string) {
    ctx.clustersService.logout(clusterUri);
  }

  function onOpen(clusterUri: string) {
    ctx.commandLauncher.executeCommand('cluster-open', { clusterUri });
  }

  function onRemove(clusterUri: string) {
    ctx.commandLauncher.executeCommand('cluster-remove', { clusterUri });
  }

  function onOpenContextMenu(navItem: ClusterNavItem) {
    ctx.mainProcessClient.openClusterContextMenu({
      isClusterConnected: navItem.connected,
      onLogin() {
        onLogin(navItem.clusterUri);
      },
      onLogout() {
        onLogout(navItem.clusterUri);
      },
      onRemove() {
        onRemove(navItem.clusterUri);
      },
      onRefresh() {
        ctx.clustersService.syncRootCluster(navItem.clusterUri);
      },
    });
  }

  return {
    items,
    onAddCluster,
    onOpenContextMenu,
    onSyncClusters,
    onOpen,
  };
}

function initItems(ctx: AppContext): ClusterNavItem[] {
  function findLeaves(clusterUri: string) {
    return ctx.clustersService
      .getClusters()
      .filter(c => c.leaf && c.uri.startsWith(clusterUri))
      .map<ClusterNavItem>(cluster => {
        return {
          active: ctx.workspacesService
            .getActiveWorkspaceDocumentService()
            .isClusterDocumentActive(cluster.uri),
          clusterUri: cluster.uri,
          title: cluster.name,
          connected: true,
          syncing: false,
        };
      });
  }

  return ctx.clustersService
    .getClusters()
    .filter(c => !c.leaf)
    .map<ClusterNavItem>(cluster => {
      const { syncing } = ctx.clustersService.getClusterSyncStatus(cluster.uri);
      return {
        active: ctx.workspacesService
          .getActiveWorkspaceDocumentService()
          .isClusterDocumentActive(cluster.uri),
        title: cluster.name,
        clusterUri: cluster.uri,
        connected: cluster.connected,
        syncing: syncing,
        leaves: cluster.connected ? findLeaves(cluster.uri) : [],
      };
    });
}
