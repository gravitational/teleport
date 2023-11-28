import React, { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { differenceInHours, formatDuration } from 'date-fns';

import { Option } from 'shared/components/Select';

import {
  middleValues,
  requestTtlMiddleValues,
} from 'teleport/AccessRequests/utils';

import Ctx from 'e-teleport/teleportContextE';

import { State as NewRequestState, ResourceKind } from '../useNewRequest';

import type { ResourceIdKind } from 'teleport/services/agents';
import type { AccessRequest, ResourceId } from 'e-teleport/services/workflow';

const SEVEN_DAYS_IN_MS = 1000 * 60 * 60 * 24 * 7;

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

  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  const [requestTTL, setRequestTTL] = useState<Option<number>>();

  const [durationOptions, setDurationOptions] = useState<Option<number>[]>([]);
  const [requestTTLDurationOptions, setRequestTTLDurationOptions] = useState<
    Option<number>[]
  >([]);
  const [suggestedReviewers, setSuggestedReviewers] = useState<string[]>([]);

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

  React.useEffect(() => {
    // duration is set to the max - 7 days
    const maxAccessDuration = new Date(Date.now() + SEVEN_DAYS_IN_MS);

    createAccessRequest('', [], maxAccessDuration, null, true)
      .then((resp: AccessRequest) => {
        // sessionTTL and maxDuration were introduced in v13.3.0.
        // Older backends will not return these values.
        if (!resp.sessionTTL || !resp.maxDuration) {
          setFetchStatus('loaded');
          return;
        }
        const values = middleValues(
          new Date(resp.created),
          new Date(resp.sessionTTL),
          new Date(resp.maxDuration)
        ).map(e => ({
          value: e.timestamp,
          label: formatDuration(e.duration),
        }));

        setDurationOptions(values);
        if (values.length >= 1) {
          setMaxDuration(values[0]);
        }
        const created = new Date(resp.created);
        const requestTTLValues = requestTtlMiddleValues(
          created,
          new Date(resp.sessionTTL)
        ).map(e => ({
          value: e.timestamp,
          label: formatDuration(e.duration),
        }));

        setRequestTTLDurationOptions(requestTTLValues);
        if (requestTTLValues.length >= 1) {
          // Get the largest value closest to 24 hours.
          const index = Math.max(
            0,
            requestTTLValues.findLastIndex(
              value => differenceInHours(created, value.value) <= 24
            )
          );
          setRequestTTL(requestTTLValues[index]);
        }

        setSuggestedReviewers(resp.reviewers.map(r => r.name));

        setFetchStatus('loaded');
        // setAttemptStatus();
      })
      .catch(() => {
        // If the fetch failed, we can still render the page, but we won't
        // be able to show the max duration options.
        setFetchStatus('loaded');
      });
  }, []);

  async function createAccessRequest(
    reason = '',
    suggestedReviewers?: string[],
    maxDuration?: Date,
    requestTTL?: Date,
    dryRun?: boolean
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
      reason,
      resourceIds,
      roles,
      suggestedReviewers,
      maxDuration,
      requestTTL,
      dryRun,
    });
  }

  function createRequest(
    reason = '',
    suggestedReviewers?: string[],
    maxDuration?: Date,
    requestTTL?: Date,
    dryRun?: boolean
  ) {
    createAttempt.setAttempt({ status: 'processing' });
    createAccessRequest(
      reason,
      suggestedReviewers,
      maxDuration,
      requestTTL,
      dryRun
    )
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
    reviewers: [
      ...new Set(
        ctx.storeUser.getSuggestedReviewers().concat(suggestedReviewers)
      ),
    ].sort(),
    createRequest,
    resourceRequestRoles,
    data,
    clearAttempt,
    numRequestedResources,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    fetchStatus,
    durationOptions,
    maxDuration,
    setMaxDuration,
    requestTTLDurationOptions,
    requestTTL,
    setRequestTTL,
  };
}

export type Props = {
  ctx: Ctx;
  selectedResource: NewRequestState['selectedResource'];
  addedResources: NewRequestState['addedResources'];
  reset: NewRequestState['clearAddedResources'];
};

export type State = ReturnType<typeof useRequestCheckout>;
