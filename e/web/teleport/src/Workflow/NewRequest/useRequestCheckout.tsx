import React, { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { Option } from 'shared/components/Select';

import Ctx from 'e-teleport/teleportContextE';
import { CreateRequest } from 'e-teleport/AccessRequests/Shared/types';

import {
  ReviewerOption,
  getDryRunMaxDuration,
  ResourceKind,
} from 'e-teleport/AccessRequests/NewRequest';

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
  const [resourceRequestRoles, setResourceRequestRoles] = useState<string[]>(
    []
  );
  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState<string[]>([]);

  const [fetchStatus, setFetchStatus] = useState<LoadingStatus>('loading');

  // Access request lifetime upon creation.
  // Duration countdown starts from access request creation.
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  // How long the request can be in a PENDING state before it expires.
  const [requestTTL, setRequestTTL] = useState<Option<number>>();

  // The reviewers defined in the users roles (static) and access list owners
  // (dynamic).
  const [suggestedReviewers, setSuggestedReviewers] = useState<string[]>([]);
  // DELETE IN 15.0.0: delete only the comment and remove the null type.
  // A null value after fetchStatus === 'loaded' means max duration and session
  // TTL is not supported (introduced v13.3.0).
  const [dryRunResponse, setDryRunResponse] = useState<AccessRequest | null>();
  // User selected reviewers from suggested reviewers options and/or
  // any other reviewers they manually added.
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerOption[]>(
    []
  );

  // Format data suitable for table listing.
  const data: {
    kind: ResourceKind;
    name: string;
    id: string;
  }[] = [];
  const resourceKeys = Object.keys(addedResources) as ResourceKind[];
  resourceKeys.forEach(kind => {
    Object.keys(addedResources[kind]).forEach(id =>
      data.push({
        kind: kind,
        name: addedResources[kind][id],
        id: id,
      })
    );
  });
  const [numRequestedResources, setNumRequestedResources] = useState(0);

  useEffect(() => {
    if (isResourceRequest) fetchResourceRequestRoles();
  }, [addedResources]);

  // Does an initial "dry run" of an empty access request to get all time
  // options and calculate suggested reviewers.
  React.useEffect(() => {
    createAccessRequest({
      maxDuration: getDryRunMaxDuration(),
      dryRun: true,
    })
      .then((resp: AccessRequest) => {
        // sessionTTL and maxDuration were introduced in v13.3.0.
        // Older backends will not return these values.
        if (!resp.sessionTTL || !resp.maxDuration) {
          setFetchStatus('loaded');
          return;
        }
        setDryRunResponse(resp);

        const reviewers = resp.reviewers.map(r => r.name).sort();
        setSuggestedReviewers(reviewers);
        // Initially select suggested reviewers for the requestor.
        setSelectedReviewers(
          reviewers.map(r => ({
            value: r,
            label: r,
            isSelected: true,
          }))
        );

        setFetchStatus('loaded');
      })
      .catch(() => {
        // If the fetch failed, we can still render the page, but we won't
        // be able to show the max duration options.
        setFetchStatus('loaded');
      });
  }, []);

  async function createAccessRequest(
    req: CreateRequest
  ): Promise<AccessRequest> {
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
      roles = selectedResourceRequestRoles;
    }

    return ctx.workflowService.createAccessRequest({
      reason: req.reason,
      resourceIds,
      roles,
      suggestedReviewers,
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
        setNumRequestedResources(data.length);
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
    }[] = data.map(resource => ({
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
    reviewers: suggestedReviewers,
    selectedReviewers,
    setSelectedReviewers,
    createRequest,
    resourceRequestRoles,
    data,
    clearAttempt,
    numRequestedResources,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    fetchStatus,
    maxDuration,
    setMaxDuration,
    requestTTL,
    setRequestTTL,
    dryRunResponse,
  };
}

type Props = {
  ctx: Ctx;
  selectedResource: NewRequestState['selectedResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
};
