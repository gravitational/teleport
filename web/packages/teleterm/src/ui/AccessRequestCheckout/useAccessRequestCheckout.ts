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

import { useState, useEffect } from 'react';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';

import useAttempt from 'shared/hooks/useAttemptNext';

import {
  getDryRunMaxDuration,
  PendingListItem,
} from 'shared/components/AccessRequests/NewRequest';
import { useSpecifiableFields } from 'shared/components/AccessRequests/NewRequest/useSpecifiableFields';

import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  PendingAccessRequest,
  extractResourceRequestProperties,
  ResourceRequest,
} from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { retryWithRelogin } from 'teleterm/ui/utils';
import {
  CreateAccessRequestRequest,
  AccessRequest as TeletermAccessRequest,
} from 'teleterm/services/tshd/types';

import { routing } from 'teleterm/ui/uri';

import { ResourceKind } from '../DocumentAccessRequests/NewRequest/useNewRequest';

import { makeUiAccessRequest } from '../DocumentAccessRequests/useAccessRequests';

export default function useAccessRequestCheckout() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();
  const clusterUri =
    ctx.workspacesService?.getActiveWorkspace()?.localClusterUri;
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();

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
    onDryRunChange,
    startTime,
    onStartTimeChange,
  } = useSpecifiableFields();

  const [showCheckout, setShowCheckout] = useState(false);
  const [hasExited, setHasExited] = useState(false);
  const [requestedCount, setRequestedCount] = useState(0);

  const { attempt: createRequestAttempt, setAttempt: setCreateRequestAttempt } =
    useAttempt('');

  const { attempt: fetchResourceRolesAttempt, run: runFetchResourceRoles } =
    useAttempt('success');

  const workspaceAccessRequest =
    ctx.workspacesService.getActiveWorkspaceAccessRequestsService();
  const docService = ctx.workspacesService.getActiveWorkspaceDocumentService();
  const pendingAccessRequest =
    workspaceAccessRequest?.getPendingAccessRequest();

  useEffect(() => {
    // Do a new dry run per changes to pending data
    // to get the latest time options and latest calculated
    // suggested reviewers.
    // Options and reviewers can change depending on the selected
    // roles or resources.
    if (showCheckout && requestedCount == 0) {
      performDryRun();
    }
  }, [showCheckout, pendingAccessRequest]);

  useEffect(() => {
    if (!pendingAccessRequest || requestedCount > 0) {
      return;
    }

    const data = getPendingAccessRequestsPerResource(pendingAccessRequest);
    runFetchResourceRoles(() =>
      retryWithRelogin(ctx, clusterUri, async () => {
        const { response } = await ctx.tshd.getRequestableRoles({
          clusterUri: rootClusterUri,
          resourceIds: data
            .filter(d => d.kind !== 'role')
            .map(d => ({
              // We have to use id, not name.
              // These fields are the same for all resources except servers,
              // where id is UUID and name is the hostname.
              name: d.id,
              kind: d.kind,
              clusterName: d.clusterName,
              subResourceName: '',
            })),
        });
        setResourceRequestRoles(response.applicableRoles);
        setSelectedResourceRequestRoles(response.applicableRoles);
      })
    );
  }, [pendingAccessRequest]);

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
      onDryRunChange(null /* set dryRunResponse to null */);
    }
  }, [showCheckout, hasExited, createRequestAttempt.status]);

  function getPendingAccessRequestsPerResource(
    pendingRequest: PendingAccessRequest
  ): PendingListItemWithOriginalItem[] {
    const data: PendingListItemWithOriginalItem[] = [];
    if (!workspaceAccessRequest) {
      return data;
    }

    switch (pendingRequest.kind) {
      case 'role': {
        const clusterName =
          ctx.clustersService.findCluster(rootClusterUri)?.name;
        pendingRequest.roles.forEach(role => {
          data.push({
            kind: 'role',
            id: role,
            name: role,
            clusterName,
          });
        });
        break;
      }
      case 'resource': {
        pendingRequest.resources.forEach(resourceRequest => {
          const { kind, id, name } =
            extractResourceRequestProperties(resourceRequest);
          data.push({
            kind,
            id,
            name,
            originalItem: resourceRequest,
            clusterName: ctx.clustersService.findClusterByResource(
              resourceRequest.resource.uri
            )?.name,
          });
        });
      }
    }
    return data;
  }

  function isCollapsed() {
    if (!workspaceAccessRequest) {
      return true;
    }
    return workspaceAccessRequest.getCollapsed();
  }

  async function toggleResource(
    pendingListItem: PendingListItemWithOriginalItem
  ) {
    if (pendingListItem.kind === 'role') {
      await workspaceAccessRequest.addOrRemoveRole(pendingListItem.id);
      return;
    }

    await workspaceAccessRequest.addOrRemoveResource(
      pendingListItem.originalItem
    );
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

  /**
   * Shared logic used both during dry runs and regular access request creation.
   */
  function prepareAndCreateRequest(req: CreateRequest) {
    const data = getPendingAccessRequestsPerResource(pendingAccessRequest);
    const params: CreateAccessRequestRequest = {
      rootClusterUri,
      reason: req.reason,
      suggestedReviewers: req.suggestedReviewers || [],
      dryRun: req.dryRun,
      resourceIds: data
        .filter(d => d.kind !== 'role')
        .map(d => ({
          name: d.id,
          clusterName: d.clusterName,
          kind: d.kind,
          subResourceName: '',
        })),
      roles: data.filter(d => d.kind === 'role').map(d => d.name),
      assumeStartTime: req.start && Timestamp.fromDate(req.start),
      maxDuration: req.maxDuration && Timestamp.fromDate(req.maxDuration),
      requestTtl: req.requestTTL && Timestamp.fromDate(req.requestTTL),
    };

    // if we have a resource access request, we pass along the selected roles from the checkout
    if (params.resourceIds.length > 0) {
      params.roles = selectedResourceRequestRoles;
    }

    setCreateRequestAttempt({ status: 'processing' });

    return retryWithRelogin(ctx, clusterUri, () =>
      ctx.clustersService.createAccessRequest(params).then(({ response }) => {
        return { accessRequest: response.request, requestedCount: data.length };
      })
    ).catch(e => {
      setCreateRequestAttempt({ status: 'failed', statusText: e.message });
      throw e;
    });
  }

  async function performDryRun() {
    let teletermAccessRequest: TeletermAccessRequest;

    try {
      const { accessRequest } = await prepareAndCreateRequest({
        dryRun: true,
        maxDuration: getDryRunMaxDuration(),
      });
      teletermAccessRequest = accessRequest;
    } catch {
      return;
    }

    setCreateRequestAttempt({ status: '' });

    const accessRequest = makeUiAccessRequest(teletermAccessRequest);
    onDryRunChange(accessRequest);
  }

  async function createRequest(req: CreateRequest) {
    let requestedCount: number;
    try {
      const response = await prepareAndCreateRequest(req);
      requestedCount = response.requestedCount;
    } catch {
      return;
    }

    setRequestedCount(requestedCount);
    reset();
    setCreateRequestAttempt({ status: 'success' });
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

  const shouldShowClusterNameColumn =
    pendingAccessRequest?.kind === 'resource' &&
    Array.from(pendingAccessRequest.resources.values()).some(a =>
      routing.isLeafCluster(a.resource.uri)
    );

  return {
    showCheckout,
    isCollapsed,
    assumedRequests: getAssumedRequests(),
    toggleResource,
    data: getPendingAccessRequestsPerResource(pendingAccessRequest),
    shouldShowClusterNameColumn,
    createRequest,
    reset,
    setHasExited,
    goToRequestsList,
    requestedCount,
    clearCreateAttempt,
    clusterUri,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    resourceRequestRoles,
    rootClusterUri,
    fetchResourceRolesAttempt,
    createRequestAttempt,
    collapseBar,
    setShowCheckout,
    selectedReviewers,
    setSelectedReviewers,
    dryRunResponse,
    maxDuration,
    onMaxDurationChange,
    maxDurationOptions,
    pendingRequestTtl,
    setPendingRequestTtl,
    pendingRequestTtlOptions,
    startTime,
    onStartTimeChange,
  };
}

type PendingListItemWithOriginalItem = Omit<PendingListItem, 'kind'> &
  (
    | {
        kind: Exclude<ResourceKind, 'role'>;
        originalItem: ResourceRequest;
      }
    | {
        kind: 'role';
      }
  );
