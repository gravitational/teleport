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

import { Cluster, LoggedInUser } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceServiceState } from 'teleterm/ui/services/workspacesService';
import { RootClusterUri } from 'teleterm/ui/uri';

export function useIdentity() {
  const ctx = useAppContext();

  ctx.clustersService.useState();
  useWorkspaceServiceState();

  async function changeRootCluster(clusterUri: RootClusterUri): Promise<void> {
    await ctx.workspacesService.setActiveWorkspace(clusterUri);
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
      profileStatusError: cluster.profileStatusError,
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
  profileStatusError: string;
}
