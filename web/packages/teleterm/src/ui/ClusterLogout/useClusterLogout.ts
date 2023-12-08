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

import { useAsync } from 'shared/hooks/useAsync';

import { RootClusterUri } from 'teleterm/ui/uri';

import { useAppContext } from '../appContextProvider';

export function useClusterLogout({
  clusterUri,
}: {
  clusterUri: RootClusterUri;
}) {
  const ctx = useAppContext();
  const [{ status, statusText }, removeCluster] = useAsync(async () => {
    await ctx.clustersService.logout(clusterUri);

    if (ctx.workspacesService.getRootClusterUri() === clusterUri) {
      const [firstConnectedWorkspace] =
        ctx.workspacesService.getConnectedWorkspacesClustersUri();
      if (firstConnectedWorkspace) {
        await ctx.workspacesService.setActiveWorkspace(firstConnectedWorkspace);
      } else {
        await ctx.workspacesService.setActiveWorkspace(null);
      }
    }

    // Remove connections first, they depend both on the cluster and the workspace.
    ctx.connectionTracker.removeItemsBelongingToRootCluster(clusterUri);

    // Remove the workspace next, because it depends on the cluster.
    ctx.workspacesService.removeWorkspace(clusterUri);

    // If there are active ssh connections to the agent, killing it will take a few seconds. To work
    // around this, kill the agent only after removing the workspace. Removing the workspace closes
    // ssh tabs, so it should terminate connections to the cluster from the app.
    //
    // If ClustersService.logout above fails, the user should still be able to manage the agent.
    await ctx.connectMyComputerService.killAgentAndRemoveData(clusterUri);

    // Remove the cluster, it does not depend on anything.
    await ctx.clustersService.removeClusterAndResources(clusterUri);
  });

  return {
    status,
    statusText,
    removeCluster,
  };
}
