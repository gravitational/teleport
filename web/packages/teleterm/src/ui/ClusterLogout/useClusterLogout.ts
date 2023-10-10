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
