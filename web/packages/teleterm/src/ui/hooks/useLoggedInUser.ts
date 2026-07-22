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

import { LoggedInUser } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { useStoreSelector } from './useStoreSelector';

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
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(store => store.rootClusterUri, [])
  );
  const loggedInUser = useStoreSelector(
    'clustersService',
    useCallback(
      state => state.clusters.get(rootClusterUri)?.loggedInUser,
      [rootClusterUri]
    )
  );

  return loggedInUser;
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
  const { rootClusterUri } = useWorkspaceContext();
  const loggedInUser = useStoreSelector(
    'clustersService',
    useCallback(
      state => state.clusters.get(rootClusterUri)?.loggedInUser,
      [rootClusterUri]
    )
  );

  return loggedInUser;
}
