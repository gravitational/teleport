/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function useAssumeAccess() {
  const ctx = useAppContext();
  const {
    localClusterUri: clusterUri,
    rootClusterUri,
    documentsService,
  } = useWorkspaceContext();
  const { requestResourcesRefresh } = useResourcesContext(rootClusterUri);

  const [assumeRoleAttempt, runAssumeRole] = useAsync((requestId: string) =>
    retryWithRelogin(ctx, clusterUri, async () => {
      await ctx.clustersService.assumeRoles(rootClusterUri, [requestId]);
      // Refresh the current resource tabs in the workspace.
      requestResourcesRefresh();
    })
  );

  async function assumeAccessList(): Promise<void> {
    const { hasLoggedIn } = await new Promise<{
      hasLoggedIn: boolean;
    }>(resolve => {
      ctx.modalsService.openRegularDialog({
        kind: 'cluster-connect',
        clusterUri: rootClusterUri,
        onCancel: () => resolve({ hasLoggedIn: false }),
        onSuccess: () => resolve({ hasLoggedIn: true }),
        prefill: undefined,
        reason: undefined,
      });
    });

    if (!hasLoggedIn) {
      return;
    }

    // Refresh the current resource tabs in the workspace.
    requestResourcesRefresh();

    // open new cluster tab
    const clusterDocument = documentsService.createClusterDocument({
      clusterUri,
      queryParams: undefined,
    });
    documentsService.add(clusterDocument);
    documentsService.open(clusterDocument.uri);
  }

  return {
    assumeAccessList,
    assumeRole: runAssumeRole,
    assumeRoleAttempt,
  };
}
