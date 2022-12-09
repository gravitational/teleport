import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import Ctx from 'e-teleport/teleportContextE';

import { State as NewRequestState, ResourceKind } from '../useNewRequest';

import type { AgentIdKind } from 'teleport/services/agents';
import type { ResourceId } from 'e-teleport/services/workflow';

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
  const [resourceRequestRoles, setResourceRequestRoles] = useState<string[]>(
    []
  );
  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState<string[]>([]);

  // Format data suitable for table listing.
  const data: {
    kind: ResourceKind;
    name: string;
    id: string;
  }[] = [];
  const resourceKeys = Object.keys(addedResources) as ResourceKind[];
  resourceKeys.forEach(kind => {
    Object.keys(addedResources[kind]).forEach(id =>
      data.push({ kind, id, name: addedResources[kind][id] })
    );
  });
  const [numRequestedResources, setNumRequestedResources] = useState(0);

  useEffect(() => {
    if (isResourceRequest) fetchResourceRequestRoles();
  }, [addedResources]);

  function createRequest(reason = '', suggestedReviewers?: string[]) {
    // field 'roles' is expected as just a list of strings
    // in the back.
    let roles: string[];
    let resourceIds: ResourceId[];
    if (selectedResource == 'role') {
      roles = data.map(item => item.name);
    } else {
      resourceIds = data.map(item => ({
        name: item.id,
        kind: item.kind as AgentIdKind,
        clusterName: clusterId,
      }));
      roles = selectedResourceRequestRoles;
    }

    createAttempt.setAttempt({ status: 'processing' });
    ctx.workflowService
      .createAccessRequest({
        reason,
        resourceIds,
        roles,
        suggestedReviewers,
      })
      .then(() => {
        createAttempt.setAttempt({ status: 'success' });
        setNumRequestedResources(data.length);
        reset();
      })
      .catch((err: Error) => {
        createAttempt.setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  // Fetches the necessary roles for a resource request
  function fetchResourceRequestRoles() {
    fetchResourceRequestRolesAttempt.setAttempt({ status: 'processing' });
    const resourceIdRequest: {
      kind: AgentIdKind;
      name: string;
      clusterName: string;
    }[] = data.map(resource => ({
      kind: resource.kind as AgentIdKind,
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
    reviewers: ctx.storeUser.getSuggestedReviewers(),
    createRequest,
    resourceRequestRoles,
    data,
    clearAttempt,
    numRequestedResources,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
  };
}

export type Props = {
  ctx: Ctx;
  selectedResource: NewRequestState['selectedResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
};

export type State = ReturnType<typeof useRequestCheckout>;
