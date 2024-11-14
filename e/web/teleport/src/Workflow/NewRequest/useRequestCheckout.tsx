import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';

import {
  getDryRunMaxDuration,
  PendingListItem,
} from 'shared/components/AccessRequests/NewRequest';
import { useSpecifiableFields } from 'shared/components/AccessRequests/NewRequest/useSpecifiableFields';
import { isKubeClusterWithNamespaces } from 'shared/components/AccessRequests/NewRequest/kube';
import KubeService from 'teleport/services/kube';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';

import Ctx from 'e-teleport/teleportContextE';

import { State as NewRequestState } from './useNewRequest';
import { parseResourceIdUri } from './kube';

import type { AccessRequest, ResourceId } from 'e-teleport/services/workflow';

type LoadingStatus = 'loading' | 'loaded';

export function useRequestCheckout({
  ctx,
  selectedResource,
  addedResources,
  reset: clearAddedResources,
}: Props) {
  const isResourceRequest = selectedResource !== 'role';
  const { clusterId } = useStickyClusterId();
  const createAttempt = useAttempt('');
  const fetchResourceRequestRolesAttempt = useAttempt('');
  const [fetchStatus, setFetchStatus] = useState<LoadingStatus>('loading');

  const {
    selectedReviewers,
    setSelectedReviewers,
    resourceRequestRoles,
    setResourceRequestRoles,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    maxDuration,
    onMaxDurationChange,
    maxDurationOptions,
    pendingRequestTtl,
    setPendingRequestTtl,
    pendingRequestTtlOptions,
    dryRunResponse,
    startTime,
    onStartTimeChange,
    onDryRunChange,
    reset: resetSpecifiableFields,
  } = useSpecifiableFields();

  // Format data suitable for table listing.
  const pendingAccessRequests: PendingListItem[] = [];
  const resourceKeys = Object.keys(addedResources) as RequestableResourceKind[];
  resourceKeys.forEach(kind => {
    Object.keys(addedResources[kind]).forEach(id => {
      let subResourceName = '';
      let resourceId = id;
      let resourceName = addedResources[kind][id];
      if (kind === 'namespace') {
        const {
          subResourceName: namespaceName,
          resourceName: kubeClusterName,
        } = parseResourceIdUri(id).params;
        resourceId = kubeClusterName;
        subResourceName = namespaceName;
        resourceName = namespaceName;
      }
      pendingAccessRequests.push({
        kind: kind,
        name: resourceName,
        id: resourceId,
        subResourceName,
      });
    });
  });

  const pendingAccessRequestsWithoutParentResource =
    pendingAccessRequests.filter(
      item => !isKubeClusterWithNamespaces(item, pendingAccessRequests)
    );

  const [numRequestedResources, setNumRequestedResources] = useState(0);
  const numAddedResources = pendingAccessRequestsWithoutParentResource.length;

  useEffect(() => {
    if (isResourceRequest && numAddedResources > 0) {
      fetchResourceRequestRoles();
      // if we add another resource, clear any successful attempt so we can
      // view the checkout screen again
      if (createAttempt.attempt.status === 'success') {
        clearAttempt();
      }
    }
  }, [addedResources, numAddedResources]);

  // Do a "dry run" of an empty access request to get time
  // options and calculate suggested reviewers.
  // Options and reviewers can change depending on the selected
  // roles or resources.
  useEffect(() => {
    if (createAttempt.attempt.status === 'success') {
      return;
    }
    setFetchStatus('loading');
    clearAttempt();

    createAccessRequest({
      maxDuration: getDryRunMaxDuration(),
      dryRun: true,
    })
      .then((resp: AccessRequest) => {
        onDryRunChange(resp);
        setFetchStatus('loaded');
      })
      .catch(() => {
        // If the fetch failed, we can still render the page, but we won't
        // be able to show the max duration options.
        setFetchStatus('loaded');
      });
  }, [addedResources]);

  async function createAccessRequest(
    req: CreateRequest
  ): Promise<AccessRequest> {
    // field 'roles' is expected as just a list of strings
    // in the back.
    let roles: string[];
    let resourceIds: ResourceId[];
    if (selectedResource == 'role') {
      roles = pendingAccessRequests.map(item => item.name);
    } else {
      resourceIds = getResourceIdsForRequests();
      roles = selectedResourceRequestRoles;
    }

    return ctx.workflowService.createAccessRequest({
      reason: req.reason,
      resourceIds,
      roles,
      suggestedReviewers: req.suggestedReviewers || [],
      maxDuration: req.maxDuration,
      requestTTL: req.requestTTL,
      dryRun: req.dryRun,
      assumeStartTime: req.start,
    });
  }

  function createRequest(req: CreateRequest) {
    createAttempt.setAttempt({ status: 'processing' });
    createAccessRequest(req)
      .then(() => {
        createAttempt.setAttempt({ status: 'success' });
        setNumRequestedResources(numAddedResources);
        clearAddedResources();
        resetSpecifiableFields();
      })
      .catch((err: Error) => {
        createAttempt.setAttempt({
          status: 'failed',
          statusText: err.message,
        });
      });
  }

  // Fetches the necessary roles for a resource request
  function fetchResourceRequestRoles() {
    fetchResourceRequestRolesAttempt.setAttempt({ status: 'processing' });
    const resourceIdRequest: ResourceId[] = getResourceIdsForRequests();

    ctx.workflowService
      .fetchResourceRequestRoles(resourceIdRequest)
      .then(roles => {
        fetchResourceRequestRolesAttempt.setAttempt({ status: 'success' });
        setResourceRequestRoles(roles);
        setSelectedResourceRequestRoles(roles);
      })
      .catch((err: Error) => {
        fetchResourceRequestRolesAttempt.setAttempt({
          status: 'failed',
          statusText: err.message,
        });
      });
  }

  function clearAttempt() {
    createAttempt.setAttempt({ status: '' });
  }

  function getResourceIdsForRequests() {
    return pendingAccessRequestsWithoutParentResource.map(resource => ({
      name: resource.id,
      kind: resource.kind,
      clusterName: clusterId,
      subResourceName: resource.subResourceName,
    }));
  }

  async function fetchKubeNamespaces(
    search: string,
    kubeCluster: PendingListItem
  ): Promise<string[]> {
    const kubeSvc = new KubeService();
    const namespaces = await kubeSvc.fetchKubernetesResources(clusterId, {
      kubeCluster: kubeCluster.id,
      search,
      limit: 50,
      searchAsRoles: 'yes',
      kind: 'namespace',
    });
    return namespaces.items.map(i => i.name);
  }

  function cancelCheckout() {
    clearAddedResources();
    resetSpecifiableFields();
    clearAttempt();
  }

  return {
    createAttempt: createAttempt.attempt,
    fetchResourceRequestRolesAttempt: fetchResourceRequestRolesAttempt.attempt,
    requireReason: ctx.storeUser.getAccessStrategy().type === 'reason',
    selectedReviewers,
    setSelectedReviewers,
    createRequest,
    resourceRequestRoles,
    pendingAccessRequests,
    clearAttempt,
    fetchKubeNamespaces,
    numRequestedResources,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    fetchStatus,
    maxDuration,
    onMaxDurationChange,
    maxDurationOptions,
    pendingRequestTtl,
    setPendingRequestTtl,
    pendingRequestTtlOptions,
    dryRunResponse,
    numAddedResources,
    startTime,
    onStartTimeChange,
    cancelCheckout,
  };
}

type Props = {
  ctx: Ctx;
  selectedResource: NewRequestState['selectedResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
};
