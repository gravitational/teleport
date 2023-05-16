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
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { LoggedInUser } from 'teleterm/services/tshd/types';

/**
 * useLoggedInUser returns the user logged into the root cluster of the active workspace. The return
 * value changes depending on the active workspace.
 *
 * It should be used within components that reside outside of WorkspaceContext, typically anything
 * that's outside of Document-type components.
 *
 * It might return undefined if there's no active workspace.
 */
export function useLoggedInUser(): LoggedInUser | undefined {
  const { clustersService, workspacesService } = useAppContext();
  clustersService.useState();
  workspacesService.useState();

  const clusterUri = workspacesService.getRootClusterUri();
  if (!clusterUri) {
    return;
  }

  const cluster = clustersService.findCluster(clusterUri);
  return cluster?.loggedInUser;
}

/**
 * useWorkspaceLoggedInUser returns the user logged into the root cluster of the workspace specified
 * by WorkspaceContext. The returned value won't change when the UI switches between workspaces.
 *
 * It should be used for components which are bound to a particular workspace and which don't change
 * their workspace over their lifecycle; typically those are Document-type components and anything
 * rendered inside of them.
 *
 * In general, the callsite should always assume that this function might return undefined.
 * Each workspace is always rendered, even when the cluster is not connected, with at least the default
 * document. In that scenario, useWorkspaceLoggedInUser could return undefined when used within the
 * default document.
 */
export function useWorkspaceLoggedInUser(): LoggedInUser | undefined {
  const { clustersService } = useAppContext();
  clustersService.useState();
  const { rootClusterUri } = useWorkspaceContext();

  const cluster = clustersService.findCluster(rootClusterUri);
  return cluster?.loggedInUser;
}
