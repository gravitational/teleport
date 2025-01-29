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

import { Cluster } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  ProfileColor,
  useWorkspaceServiceState,
} from 'teleterm/ui/services/workspacesService';
import { RootClusterUri } from 'teleterm/ui/uri';

export function useIdentity() {
  const ctx = useAppContext();

  ctx.clustersService.useState();
  useWorkspaceServiceState();

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

  const activeClusterUri = ctx.workspacesService.getRootClusterUri();
  function getActiveRootCluster(): Cluster | undefined {
    return ctx.clustersService.findCluster(activeClusterUri);
  }

  function changeColor(color: ProfileColor): undefined {
    const clusterUri = ctx.workspacesService.getRootClusterUri();
    if (!clusterUri) {
      return;
    }
    ctx.workspacesService.changeProfileColor(clusterUri, color);
  }

  const rootClusters = ctx.clustersService
    .getClusters()
    .filter(c => !c.leaf)
    .filter(c => c.uri !== activeClusterUri);

  return {
    changeRootCluster,
    addCluster,
    refreshCluster,
    logout,
    changeColor,
    activeRootCluster: getActiveRootCluster(),
    rootClusters,
  };
}
