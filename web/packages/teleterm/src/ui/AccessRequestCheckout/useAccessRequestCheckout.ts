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
  ReviewerOption,
  getDryRunMaxDuration,
} from 'shared/components/AccessRequests/NewRequest';

import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';

import { Option } from 'shared/components/Select';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { PendingAccessRequest } from 'teleterm/ui/services/workspacesService';
import { retryWithRelogin } from 'teleterm/ui/utils';
import {
  CreateAccessRequestRequest,
  AccessRequest as TeletermAccessRequest,
} from 'teleterm/services/tshd/types';

import { ResourceKind } from '../DocumentAccessRequests/NewRequest/useNewRequest';

import { makeUiAccessRequest } from '../DocumentAccessRequests/useAccessRequests';

import type { AccessRequest } from 'shared/services/accessRequests';

export default function useAccessRequestCheckout() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();
  const clusterUri =
    ctx.workspacesService?.getActiveWorkspace()?.localClusterUri;
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();

  // Contains max time options (to calculate max duration and requestTTL options)
  // and suggested reviewers that were available both statically (from roles)
  // and dynamically (from access lists).
  const [dryRunResponse, setDryRunResponse] = useState<AccessRequest | null>();
  // The reviewers defined in the users roles (static) and access list owners
  // (dynamic).
  const [suggestedReviewers, setSuggestedReviewers] = useState<string[]>([]);
  // User selected reviewers from suggested reviewers options and/or
  // any other reviewers they manually added.
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerOption[]>(
    []
  );

  // Access request lifetime upon creation.
  // Duration countdown starts from access request creation.
  const [maxDuration, setMaxDuration] = useState<Option<number>>();
  // How long the request can be in a PENDING state before it expires.
  const [requestTTL, setRequestTTL] = useState<Option<number>>();

  const [showCheckout, setShowCheckout] = useState(false);
  const [hasExited, setHasExited] = useState(false);
  const [requestedCount, setRequestedCount] = useState(0);
  const [resourceRequestRoles, setResourceRequestRoles] = useState<string[]>(
    []
  );
  const [selectedResourceRequestRoles, setSelectedResourceRequestRoles] =
    useState<string[]>([]);

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
    // Do a new dry run per checkout to get the latest time options
    // and latest calculated suggested reviewers.
    if (showCheckout) {
      performDryRun();
    }
  }, [showCheckout]);

  useEffect(() => {
    if (!pendingAccessRequest) {
      return;
    }

    const data = getPendingAccessRequestsPerResource(pendingAccessRequest);
    runFetchResourceRoles(() =>
      retryWithRelogin(ctx, clusterUri, () =>
        ctx.clustersService.getRequestableRoles({
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
        })
      ).then(response => {
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
      setDryRunResponse(null);
    }
  }, [showCheckout, hasExited, createRequestAttempt.status]);

  function getPendingAccessRequestsPerResource(
    resourceIds: PendingAccessRequest
  ) {
    const data: {
      kind: ResourceKind;
      clusterName: string;
      /** Identifier of the resource. Should be sent in requests. */
      id: string;
      /** Name of the resource, for presentation purposes only. */
      name: string;
    }[] = [];
    if (!workspaceAccessRequest) {
      return data;
    }
    const clusterName = ctx.clustersService.findCluster(clusterUri)?.name;
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
    setDryRunResponse(accessRequest);

    const reviewers = accessRequest.reviewers.map(r => r.name).sort();
    setSuggestedReviewers(reviewers);
    // Initially select suggested reviewers for the requestor.
    setSelectedReviewers(
      reviewers.map(r => ({
        value: r,
        label: r,
        isSelected: true,
      }))
    );
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

  return {
    showCheckout,
    isCollapsed,
    assumedRequests: getAssumedRequests(),
    toggleResource,
    data: getPendingAccessRequestsPerResource(pendingAccessRequest),
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
    suggestedReviewers,
    selectedReviewers,
    setSelectedReviewers,
    dryRunResponse,
    maxDuration,
    setMaxDuration,
    requestTTL,
    setRequestTTL,
  };
}
