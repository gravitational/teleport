import { useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import Ctx from 'e-teleport/teleportContextE';
import { ResourceId, ResourceIdKind } from 'e-teleport/services/workflow';
import { State as NewRequestState, ResourceKind } from '../useNewRequest';

export function useRequestCheckout({
  ctx,
  selectedResource,
  addedResources,
  reset,
}: Props) {
  const { clusterId } = useStickyClusterId();
  const { attempt, setAttempt } = useAttempt('');

  // Format data suitable for table listing.
  const data: { kind: ResourceKind; name: string; id: string }[] = [];
  const resourceKeys = Object.keys(addedResources) as ResourceKind[];
  resourceKeys.forEach(kind => {
    Object.keys(addedResources[kind]).forEach(id =>
      data.push({ kind, id, name: addedResources[kind][id] })
    );
  });
  const [numRequestedResources, setNumRequestedResources] = useState(0);

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
        kind: item.kind as ResourceIdKind,
        clusterName: clusterId,
      }));
    }

    setAttempt({ status: 'processing' });
    ctx.workflowService
      .createAccessRequest({
        reason,
        resourceIds,
        roles,
        suggestedReviewers,
      })
      .then(() => {
        setAttempt({ status: 'success' });
        setNumRequestedResources(data.length);
        reset();
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  return {
    attempt,
    requireReason: ctx.storeUser.getAccessStrategy().type === 'reason',
    reviewers: ctx.storeUser.getSuggestedReviewers(),
    createRequest,
    data,
    clearAttempt,
    numRequestedResources,
  };
}

export type Props = {
  ctx: Ctx;
  selectedResource: NewRequestState['selectedResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
};

export type State = ReturnType<typeof useRequestCheckout>;
