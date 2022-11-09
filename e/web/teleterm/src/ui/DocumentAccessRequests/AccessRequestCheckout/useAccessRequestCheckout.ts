import { useState, useEffect } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { ResourceKind } from '../NewRequest/useNewRequest';

export default function useAccessRequestCheckout() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();
  const clusterUri =
    ctx.workspacesService?.getActiveWorkspace()?.localClusterUri;
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();

  const [showCheckout, setShowCheckout] = useState(false);
  const [hasExited, setHasExited] = useState(false);
  const [requestedCount, setRequestedCount] = useState(0);

  const {
    attempt: createRequestAttempt,
    setAttempt: setCreateRequestAttempt,
    run: runCreateRequest,
  } = useAttempt('');

  const workspaceAccessRequest =
    ctx.workspacesService.getActiveWorkspaceAccessRequestsService();
  const docService = ctx.workspacesService.getActiveWorkspaceDocumentService();

  useEffect(() => {
    clearCreateAttempt();
  }, [clusterUri]);

  useEffect(() => {
    if (
      !showCheckout &&
      hasExited &&
      createRequestAttempt.status === 'success'
    ) {
      clearCreateAttempt();
      setRequestedCount(0);
    }
  }, [showCheckout, hasExited, createRequestAttempt.status]);

  function getPendingAccessRequestsPerResource() {
    const data: {
      kind: ResourceKind;
      clusterName: string;
      id: string;
      name: string;
    }[] = [];
    if (!workspaceAccessRequest) {
      return data;
    }
    const clusterName = ctx.clustersService.findCluster(clusterUri)?.name;
    const resourceIds = workspaceAccessRequest.getPendingAccessRequest();
    const resourceKeys = Object.keys(resourceIds) as ResourceKind[];
    resourceKeys.forEach(kind => {
      Object.keys(resourceIds[kind]).forEach(id => {
        data.push({ kind, id, name: resourceIds[kind][id], clusterName });
      });
    });
    return data;
  }

  function isCollapsed() {
    if (!workspaceAccessRequest) {
      return true;
    }
    return workspaceAccessRequest.getCollapsed();
  }

  function toggleResource(
    kind: ResourceKind,
    resourceId: string,
    resourceName: string
  ) {
    workspaceAccessRequest.addOrRemoveResource(kind, resourceId, resourceName);
  }

  function getAssumedRequests() {
    if (!clusterUri) {
      return [];
    }
    const assumed = ctx.clustersService.getAssumedRequests(rootClusterUri);
    if (!assumed) {
      return [];
    }
    return Object.values(assumed);
  }

  function createRequest(reason: string, suggestedReviewers: string[]) {
    const data = getPendingAccessRequestsPerResource();
    const req = {
      rootClusterUri,
      reason,
      suggestedReviewers,
      resourceIds: data.filter(d => d.kind !== 'role'),
      roles: data.filter(d => d.kind === 'role').map(d => d.name),
    };
    runCreateRequest(() =>
      retryWithRelogin(ctx, clusterUri, () =>
        ctx.clustersService.createAccessRequest(req).then(() => {
          setRequestedCount(data.length);
          reset();
        })
      )
    );
  }

  function clearCreateAttempt() {
    setCreateRequestAttempt({ status: '', statusText: '' });
  }

  function collapseBar() {
    if (workspaceAccessRequest) {
      return workspaceAccessRequest.toggleBar();
    }
  }

  function reset() {
    if (workspaceAccessRequest) {
      return workspaceAccessRequest.clearPendingAccessRequest();
    }
    clearCreateAttempt();
  }

  function goToRequestsList() {
    const activeDoc = docService.getActive();
    if (activeDoc && activeDoc.kind === 'doc.access_requests') {
      docService.update(activeDoc.uri, {
        state: 'browsing',
        title: 'Access Requests',
      });
    } else {
      const listDoc = docService.createAccessRequestDocument({
        clusterUri: rootClusterUri,
        state: 'browsing',
      });

      docService.add(listDoc);
      docService.open(listDoc.uri);
    }
  }

  return {
    showCheckout,
    isCollapsed,
    assumedRequests: getAssumedRequests(),
    toggleResource,
    data: getPendingAccessRequestsPerResource(),
    createRequest,
    reset,
    setHasExited,
    goToRequestsList,
    requestedCount,
    clearCreateAttempt,
    clusterUri,
    rootClusterUri,
    attempt: createRequestAttempt,
    collapseBar,
    setShowCheckout,
  };
}
