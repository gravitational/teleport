import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { getNumAddedResources } from 'shared/components/AccessRequests/Shared/utils';

import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';

import {
  getDryRunMaxDuration,
  ResourceKind,
} from 'shared/components/AccessRequests/NewRequest';
import { useSpecifiableFields } from 'shared/components/AccessRequests/NewRequest/useSpecifiableFields';

import Ctx from 'e-teleport/teleportContextE';

import { State as NewRequestState } from './useNewRequest';

import type { ResourceIdKind } from 'teleport/services/agents';
import type { AccessRequest, ResourceId } from 'e-teleport/services/workflow';

type LoadingStatus = 'loading' | 'loaded';

export function useRequestCheckout({
  ctx,
  selectedResource,
  addedResources,
  reset,
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
  } = useSpecifiableFields();

  // Format data suitable for table listing.
  const pendingAccessRequests: {
    kind: ResourceKind;
    name: string;
    id: string;
  }[] = [];
  const resourceKeys = Object.keys(addedResources) as ResourceKind[];
  resourceKeys.forEach(kind => {
    Object.keys(addedResources[kind]).forEach(id =>
      pendingAccessRequests.push({
        kind: kind,
        name: addedResources[kind][id],
        id: id,
      })
    );
  });
  const [numRequestedResources, setNumRequestedResources] = useState(0);

  const numAddedResources = getNumAddedResources(addedResources);

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
    setFetchStatus('loading');

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
      resourceIds = pendingAccessRequests.map(item => ({
        name: item.id,
        kind: item.kind as ResourceIdKind,
        clusterName: clusterId,
      }));
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
        setNumRequestedResources(pendingAccessRequests.length);
        reset();
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
    const resourceIdRequest: {
      kind: ResourceIdKind;
      name: string;
      clusterName: string;
    }[] = pendingAccessRequests.map(resource => ({
      kind: resource.kind as ResourceIdKind,
      name: resource.id,
      clusterName: clusterId,
    }));

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
  };
}

type Props = {
  ctx: Ctx;
  selectedResource: NewRequestState['selectedResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
};
