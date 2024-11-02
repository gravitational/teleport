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
  PendingKubeResourceItem,
  isKubeClusterWithNamespaces,
  KubeNamespaceRequest,
} from 'shared/components/AccessRequests/NewRequest';
import { useSpecifiableFields } from 'shared/components/AccessRequests/NewRequest/useSpecifiableFields';

import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  PendingAccessRequest,
  extractResourceRequestProperties,
  ResourceRequest,
  mapRequestToKubeNamespaceUri,
  mapKubeNamespaceUriToRequest,
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
    reset: resetSpecifiableFields,
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
  const pendingAccessRequestRequest =
    workspaceAccessRequest?.getPendingAccessRequest();

  const pendingAccessRequests = getPendingAccessRequestsPerResource(
    pendingAccessRequestRequest
  );

  const pendingAccessRequestsWithoutParentResource =
    pendingAccessRequests.filter(
      p => !isKubeClusterWithNamespaces(p, pendingAccessRequests)
    );

  useEffect(() => {
    // Do a new dry run per changes to pending access requests
    // to get the latest time options and latest calculated
    // suggested reviewers.
    // Options and reviewers can change depending on the selected
    // roles or resources.
    if (showCheckout && requestedCount == 0) {
      performDryRun();
    }
  }, [showCheckout, pendingAccessRequestRequest]);

  useEffect(() => {
    if (!pendingAccessRequestRequest || requestedCount > 0) {
      return;
    }

    runFetchResourceRoles(() =>
      retryWithRelogin(ctx, clusterUri, async () => {
        const { response } = await ctx.tshd.getRequestableRoles({
          clusterUri: rootClusterUri,
          resourceIds: pendingAccessRequestsWithoutParentResource
            .filter(d => d.kind !== 'role')
            .map(d => ({
              // We have to use id, not name.
              // These fields are the same for all resources except servers,
              // where id is UUID and name is the hostname.
              name: d.id,
              kind: d.kind,
              clusterName: d.clusterName,
              subResourceName: d.subResourceName || '',
            })),
        });
        setResourceRequestRoles(response.applicableRoles);
        setSelectedResourceRequestRoles(response.applicableRoles);
      })
    );
  }, [pendingAccessRequestRequest]);

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

  /**
   * @param pendingRequest holds a list or map of resources to process
   */
  function getPendingAccessRequestsPerResource(
    pendingRequest: PendingAccessRequest
  ): PendingListItemWithOriginalItem[] {
    const pendingAccessRequests: PendingListItemWithOriginalItem[] = [];
    if (!workspaceAccessRequest) {
      return pendingAccessRequests;
    }

    switch (pendingRequest.kind) {
      case 'role': {
        const clusterName =
          ctx.clustersService.findCluster(rootClusterUri)?.name;
        pendingRequest.roles.forEach(role => {
          pendingAccessRequests.push({
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
          // If this request is a kube cluster and has namespaces
          // extract each as own request.
          if (
            resourceRequest.kind === 'kube' &&
            resourceRequest.resource.namespaces?.size > 0
          ) {
            // Process each namespace.
            resourceRequest.resource.namespaces.forEach(namespaceRequestUri => {
              const { kind, id, name } =
                mapKubeNamespaceUriToRequest(namespaceRequestUri);

              const item = {
                kind,
                id,
                name,
                subResourceName: name,
                originalItem: resourceRequest,
                clusterName:
                  ctx.clustersService.findClusterByResource(namespaceRequestUri)
                    ?.name,
              };
              pendingAccessRequests.push(item);
            });
          }

          const { kind, id, name } =
            extractResourceRequestProperties(resourceRequest);
          const item: PendingListItemWithOriginalItem = {
            kind,
            id,
            name,
            originalItem: resourceRequest,
            clusterName: ctx.clustersService.findClusterByResource(
              resourceRequest.resource.uri
            )?.name,
          };
          pendingAccessRequests.push(item);
        });
      }
    }
    return pendingAccessRequests;
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

  async function bulkToggleKubeResources(
    items: PendingKubeResourceItem[],
    kubeCluster: PendingListKubeClusterWithOriginalItem
  ) {
    await workspaceAccessRequest.addOrRemoveKubeNamespaces(
      items.map(item =>
        mapRequestToKubeNamespaceUri({
          id: item.id,
          name: item.subResourceName,
          clusterUri: kubeCluster.originalItem.resource.uri,
        })
      )
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
    const params: CreateAccessRequestRequest = {
      rootClusterUri,
      reason: req.reason,
      suggestedReviewers: req.suggestedReviewers || [],
      dryRun: req.dryRun,
      resourceIds: pendingAccessRequestsWithoutParentResource
        .filter(d => d.kind !== 'role')
        .map(d => {
          return {
            name: d.id,
            clusterName: d.clusterName,
            kind: d.kind,
            subResourceName: d.subResourceName || '',
          };
        }),
      roles: pendingAccessRequestsWithoutParentResource
        .filter(d => d.kind === 'role')
        .map(d => d.name),
      assumeStartTime: req.start && Timestamp.fromDate(req.start),
      maxDuration: req.maxDuration && Timestamp.fromDate(req.maxDuration),
      requestTtl: req.requestTTL && Timestamp.fromDate(req.requestTTL),
    };

    // Don't attempt creating anything if there are no resources selected.
    if (!params.resourceIds.length && !params.roles.length) {
      return;
    }

    // if we have a resource access request, we pass along the selected roles from the checkout
    if (params.resourceIds.length > 0) {
      params.roles = selectedResourceRequestRoles;
    }

    setCreateRequestAttempt({ status: 'processing' });

    return retryWithRelogin(ctx, clusterUri, () =>
      ctx.clustersService.createAccessRequest(params).then(({ response }) => {
        return {
          accessRequest: response.request,
          requestedCount:
            pendingAccessRequestsWithoutParentResource.filter.length,
        };
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
      setCreateRequestAttempt({ status: '' });
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
    resetSpecifiableFields();
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

  async function fetchKubeNamespaces({
    kubeCluster,
    search,
  }: KubeNamespaceRequest): Promise<string[]> {
    const { response } = await ctx.tshd.listKubernetesResources({
      searchKeywords: search,
      limit: 50,
      useSearchAsRoles: true,
      nextKey: '',
      resourceType: 'namespace',
      clusterUri,
      predicateExpression: '',
      kubernetesCluster: kubeCluster,
      kubernetesNamespace: '',
    });
    return response.resources.map(i => i.name);
  }

  const shouldShowClusterNameColumn =
    pendingAccessRequestRequest?.kind === 'resource' &&
    Array.from(pendingAccessRequestRequest.resources.values()).some(a =>
      routing.isLeafCluster(a.resource.uri)
    );

  return {
    showCheckout,
    isCollapsed,
    assumedRequests: getAssumedRequests(),
    toggleResource,
    pendingAccessRequests,
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
    fetchKubeNamespaces,
    bulkToggleKubeResources,
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

type PendingListKubeClusterWithOriginalItem = Omit<PendingListItem, 'kind'> & {
  kind: Extract<ResourceKind, 'kube_cluster'>;
  originalItem: Extract<ResourceRequest, { kind: 'kube' }>;
};
