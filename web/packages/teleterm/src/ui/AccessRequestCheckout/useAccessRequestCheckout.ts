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

import { useEffect, useState } from 'react';

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import {
  getDryRunMaxDuration,
  isKubeClusterWithNamespaces,
  PendingKubeResourceItem,
  PendingListItem,
  RequestableResourceKind,
} from 'shared/components/AccessRequests/NewRequest';
import { useSpecifiableFields } from 'shared/components/AccessRequests/NewRequest/useSpecifiableFields';
import { CreateRequest } from 'shared/components/AccessRequests/Shared/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  CreateAccessRequestRequest,
  AccessRequest as TeletermAccessRequest,
} from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceServiceState } from 'teleterm/ui/services/workspacesService';
import {
  extractResourceRequestProperties,
  mapKubeNamespaceUriToRequest,
  mapRequestToKubeNamespaceUri,
  PendingAccessRequest,
  ResourceRequest,
} from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { routing } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { makeUiAccessRequest } from '../DocumentAccessRequests/useAccessRequests';

export default function useAccessRequestCheckout() {
  const ctx = useAppContext();
  useWorkspaceServiceState();
  ctx.clustersService.useState();
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
    reasonMode,
    reasonPrompts,
  } = useSpecifiableFields();

  const [showCheckout, setShowCheckout] = useState(false);
  const [hasExited, setHasExited] = useState(false);
  const [requestedCount, setRequestedCount] = useState(0);

  const { attempt: createRequestAttempt, setAttempt: setCreateRequestAttempt } =
    useAttempt('');
  // isCreatingRequest is an auxiliary variable that helps to differentiate between a dry run being
  // performed vs an actual request being created, as both types of requests use the same attempt
  // object (createRequestAttempt).
  // TODO(ravicious): Remove this in React 19 when useSyncExternalStore updates are batched with
  // other updates.
  const [isCreatingRequest, setIsCreatingRequest] = useState(false);

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
    if (showCheckout && requestedCount == 0 && !isCreatingRequest) {
      performDryRun();
    }
  }, [
    showCheckout,
    pendingAccessRequestRequest,
    requestedCount,
    isCreatingRequest,
  ]);

  useEffect(() => {
    if (!pendingAccessRequestRequest || requestedCount > 0) {
      return;
    }

    runFetchResourceRoles(() =>
      retryWithRelogin(ctx, rootClusterUri, async () => {
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
  }, [pendingAccessRequestRequest, requestedCount]);

  useEffect(() => {
    clearCreateAttempt();
  }, [rootClusterUri]);

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

  function updateNamespacesForKubeCluster(
    items: PendingKubeResourceItem[],
    kubeCluster: PendingListKubeClusterWithOriginalItem
  ) {
    workspaceAccessRequest.updateNamespacesForKubeCluster(
      items.map(item =>
        mapRequestToKubeNamespaceUri({
          id: item.id,
          name: item.subResourceName,
          clusterUri: kubeCluster.originalItem.resource.uri,
        })
      ),
      kubeCluster.originalItem.resource.uri
    );
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

    return retryWithRelogin(ctx, rootClusterUri, () =>
      ctx.clustersService.createAccessRequest(params).then(({ response }) => {
        return {
          accessRequest: response.request,
          requestedCount: pendingAccessRequestsWithoutParentResource.length,
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
        reason: 'placeholder-reason',
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
    setIsCreatingRequest(true);
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
    setIsCreatingRequest(false);
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

  async function fetchKubeNamespaces(
    search: string,
    kubeCluster: PendingListKubeClusterWithOriginalItem
  ): Promise<string[]> {
    const { response } = await ctx.tshd.listKubernetesResources({
      searchKeywords: search,
      limit: 50,
      useSearchAsRoles: true,
      nextKey: '',
      resourceType: 'namespace',
      clusterUri: kubeCluster.originalItem.resource.uri,
      predicateExpression: '',
      kubernetesCluster: kubeCluster.id,
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
    toggleResource,
    pendingAccessRequests,
    shouldShowClusterNameColumn,
    createRequest,
    reset,
    setHasExited,
    goToRequestsList,
    requestedCount,
    clearCreateAttempt,
    selectedResourceRequestRoles,
    setSelectedResourceRequestRoles,
    resourceRequestRoles,
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
    updateNamespacesForKubeCluster,
    reasonMode,
    reasonPrompts,
  };
}

type ResourceKind =
  | Extract<
      RequestableResourceKind,
      | 'node'
      | 'app'
      | 'db'
      | 'kube_cluster'
      | 'saml_idp_service_provider'
      | 'namespace'
      | 'aws_ic_account_assignment'
      | 'windows_desktop'
    >
  | 'role';

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

export type PendingListKubeClusterWithOriginalItem = Omit<
  PendingListItem,
  'kind'
> & {
  kind: Extract<ResourceKind, 'kube_cluster'>;
  originalItem: Extract<ResourceRequest, { kind: 'kube' }>;
};
