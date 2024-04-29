import { useAsync } from 'shared/hooks/useAsync';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

export function useAssumeAccess() {
  const ctx = useAppContext();
  const {
    localClusterUri: clusterUri,
    rootClusterUri,
    documentsService,
  } = useWorkspaceContext();
  const { requestResourcesRefresh } = useResourcesContext();

  const [assumeRoleAttempt, runAssumeRole] = useAsync((requestId: string) =>
    retryWithRelogin(ctx, clusterUri, async () => {
      await ctx.clustersService.assumeRole(rootClusterUri, [requestId], []);
      // refresh the current resource tabs
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

    // refresh the current resource tabs
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
