import { useEffect, useRef, useState } from 'react';

import {
  getDryRunMaxDuration,
  PendingListItem,
} from 'shared/components/AccessRequests/NewRequest';
import { isKubeClusterWithNamespaces } from 'shared/components/AccessRequests/NewRequest/kube';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';
import { useSpecifiableFields } from 'shared/components/AccessRequests/NewRequest/useSpecifiableFields';
import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import {
  getResourceIDString,
  RequestKind,
  ResourceAccessId,
  ResourceId,
} from 'shared/services/accessRequests';

import {
  AccessRequest,
  CreateAccessRequest,
} from 'e-teleport/services/workflow';
import Ctx from 'e-teleport/teleportContextE';
import KubeService from 'teleport/services/kube';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { parseResourceIdUri } from './kube';
import { State as NewRequestState } from './useNewRequest';

type LoadingStatus = 'loading' | 'loaded';

export function useRequestCheckout({
  ctx,
  isResourceRequest,
  addedResources,
  addedResourceConstraints,
  setResourceConstraints,
  reset: clearAddedResources,
}: Props) {
  const { clusterId } = useStickyClusterId();
  const createAttempt = useAttempt('');
  const fetchResourceRequestRolesAttempt = useAttempt('');
  const [fetchStatus, setFetchStatus] = useState<LoadingStatus>('loading');
  const dryRunAbortRef = useRef<AbortController | null>(null);

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
    reasonMode,
    reasonPrompts,
    requestKind,
    setRequestKind,
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
        clusterName: clusterId,
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
    const abortController = new AbortController();
    if (isResourceRequest && numAddedResources > 0) {
      fetchResourceRequestRoles(abortController.signal);
      // if we add another resource, clear any successful attempt so we can
      // view the checkout screen again
      if (createAttempt.attempt.status === 'success') {
        clearAttempt();
      }
    }
    return () => abortController.abort();
  }, [addedResources, numAddedResources]);

  // If adding constraints, switch to a short-term request.
  useEffect(() => {
    if (
      requestKind === RequestKind.LongTerm &&
      Object.keys(addedResourceConstraints)?.length
    ) {
      setRequestKind(RequestKind.ShortTerm);
    }
  }, [addedResourceConstraints]);

  // Do a "dry run" of an empty access request to get time
  // options and calculate suggested reviewers.
  // Options and reviewers can change depending on the selected
  // roles or resources.
  // Rerun the dry run when the user adds or removes resources,
  // or 'requestKind' changes.
  useEffect(() => {
    if (createAttempt.attempt.status === 'success') {
      return;
    }

    // abort any in-flight dry run and use a fresh controller
    const controller = new AbortController();
    dryRunAbortRef.current?.abort();
    dryRunAbortRef.current = controller;

    // use timer so we don't show loading state for short requests
    const timer = setTimeout(() => {
      if (fetchStatus !== 'loading') {
        setFetchStatus('loading');
      }
    }, 400);

    clearAttempt();

    createAccessRequest(
      {
        maxDuration: getDryRunMaxDuration(),
        reason: 'placeholder-reason',
        dryRun: true,
        requestKind,
      },
      controller.signal
    )
      .then((resp: AccessRequest) => {
        onDryRunChange(resp);
        clearTimeout(timer);
        setFetchStatus('loaded');
      })
      .catch(e => {
        if (e?.name === 'AbortError') {
          return;
        }
        // If the fetch failed, we can still render the page, but we won't
        // be able to show the max duration or long-term options.
        clearTimeout(timer);
        setFetchStatus('loaded');
      });

    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [addedResources, addedResourceConstraints, requestKind]);

  async function createAccessRequest(
    req: CreateRequest,
    signal?: AbortSignal
  ): Promise<AccessRequest> {
    // field 'roles' is expected as just a list of strings
    // in the back.
    let roles: string[];
    let resourceAccessIds: ResourceAccessId[];

    if (!isResourceRequest) {
      roles = pendingAccessRequests.map(item => item.name);
    } else {
      resourceAccessIds = getResourceIDsForRequest({
        resources: pendingAccessRequestsWithoutParentResource,
        resourceConstraints: addedResourceConstraints,
        cluster: clusterId,
      });
      roles = selectedResourceRequestRoles;
    }

    const params: CreateAccessRequest = {
      reason: req.reason,
      resourceAccessIds,
      suggestedReviewers: req.suggestedReviewers || [],
      dryRun: req.dryRun,
      requestKind: req.requestKind,
      roles,
      maxDuration: req.maxDuration,
      requestTTL: req.requestTTL,
      assumeStartTime: req.start,
    };

    return ctx.workflowService.createAccessRequest(params, signal);
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
  // TODO(kiosion): update to handle ResourceAccessId.
  function fetchResourceRequestRoles(signal: AbortSignal) {
    fetchResourceRequestRolesAttempt.setAttempt({ status: 'processing' });
    const resourceIdRequest: ResourceId[] = getResourceIDsForRequest({
      resources: pendingAccessRequestsWithoutParentResource,
      resourceConstraints: addedResourceConstraints,
      cluster: clusterId,
    }).map(r => r.id);

    ctx.workflowService
      .fetchResourceRequestRoles(resourceIdRequest, signal)
      .then(roles => {
        fetchResourceRequestRolesAttempt.setAttempt({ status: 'success' });
        setResourceRequestRoles(roles);
        setSelectedResourceRequestRoles(roles);
      })
      .catch((err: Error) => {
        if (!signal.aborted) {
          fetchResourceRequestRolesAttempt.setAttempt({
            status: 'failed',
            statusText: err.message,
          });
        }
      });
  }

  function clearAttempt() {
    createAttempt.setAttempt({ status: '' });
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

  // If setting request kind to longTerm, and Constraints are present,
  // they should be cleared.
  const setRequestKindWrapper = (newKind: RequestKind) => {
    const addedConstraintsKeys = Object.keys(
      addedResourceConstraints
    ) as (keyof typeof addedResourceConstraints)[];
    if (newKind === RequestKind.LongTerm && addedConstraintsKeys.length) {
      for (const k of addedConstraintsKeys) {
        setResourceConstraints(k, undefined);
      }
    }
    setRequestKind(newKind);
  };

  return {
    createAttempt: createAttempt.attempt,
    fetchResourceRequestRolesAttempt: fetchResourceRequestRolesAttempt.attempt,
    requireReason:
      ctx.storeUser.getAccessStrategy().type === 'reason' ||
      reasonMode === 'required',
    reasonPrompts,
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
    requestKind,
    setRequestKind: setRequestKindWrapper,
  };
}

const getResourceIDsForRequest = ({
  resources,
  resourceConstraints,
  cluster,
}: {
  resources: PendingListItem[];
  resourceConstraints: NewRequestState['addedResourceConstraints'];
  cluster: string;
}): ResourceAccessId[] =>
  resources.map(r => ({
    id: {
      name: r.id,
      kind: r.kind,
      clusterName: cluster,
      subResourceName: r.subResourceName,
    },
    constraints: getConstraintsForResource({
      resource: r,
      resourceConstraints,
      cluster,
    }),
  }));

const getConstraintsForResource = ({
  resource,
  resourceConstraints,
  cluster,
}: {
  resource: PendingListItem;
  resourceConstraints: NewRequestState['addedResourceConstraints'];
  cluster: string;
}) =>
  // Only AWS Console apps support constraints for now.
  resource.kind === 'app'
    ? resourceConstraints[
        getResourceIDString({ cluster, kind: resource.kind, name: resource.id })
      ]
    : undefined;

type Props = {
  ctx: Ctx;
  isResourceRequest: boolean;
  addedResources: NewRequestState['addedResources'];
  addedResourceConstraints: NewRequestState['addedResourceConstraints'];
  setResourceConstraints: NewRequestState['setResourceConstraints'];
  reset: NewRequestState['clearAddedResources'];
};
