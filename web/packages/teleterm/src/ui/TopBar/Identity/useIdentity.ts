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

import { useCallback } from 'react';

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { getErrorMessage } from 'shared/utils/error';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { WorkspaceColor } from 'teleterm/ui/services/workspacesService';
import { Workspace } from 'teleterm/ui/services/workspacesService';
import { RootClusterUri } from 'teleterm/ui/uri';

export function useIdentity() {
  const ctx = useAppContext();

  const workspaces = useStoreSelector(
    'workspacesService',
    useCallback(state => state.workspaces, [])
  );
  const clusters = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters, [])
  );
  const activeClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );

  async function changeRootCluster(clusterUri: RootClusterUri): Promise<void> {
    await ctx.workspacesService.setActiveWorkspace(clusterUri);
  }

  const addCluster = useCallback(() => {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }, [ctx.commandLauncher]);

  function refreshCluster(clusterUri: RootClusterUri): void {
    ctx.commandLauncher.executeCommand('cluster-connect', { clusterUri });
  }

  function logout(clusterUri: RootClusterUri): void {
    ctx.commandLauncher.executeCommand('cluster-logout', { clusterUri });
  }

  async function forget(clusterUri: RootClusterUri): Promise<void> {
    try {
      await ctx.mainProcessClient.forgetCluster(clusterUri);
    } catch (err) {
      ctx.notificationsService.notifyError({
        title: 'Failed to forget cluster',
        description: getErrorMessage(err),
      });
    }
  }

  function getActiveRootCluster(): Cluster | undefined {
    return ctx.clustersService.findCluster(activeClusterUri);
  }

  function changeColor(color: WorkspaceColor): undefined {
    const clusterUri = ctx.workspacesService.getRootClusterUri();
    if (!clusterUri) {
      return;
    }
    ctx.workspacesService.changeWorkspaceColor(clusterUri, color);
  }

  const identityItems: IdentityItem[] = Object.entries(workspaces)
    .filter(([uri]) => uri !== activeClusterUri)
    .map(([uri, workspace]) => ({
      uri,
      workspace: workspace,
      cluster: clusters.get(uri),
    }));

  return {
    changeRootCluster,
    addCluster,
    refreshCluster,
    logout,
    forget,
    changeColor,
    activeRootCluster: getActiveRootCluster(),
    identityItems,
  };
}

export interface IdentityItem {
  uri: RootClusterUri;
  workspace: Workspace;
  cluster: Cluster | undefined;
}
