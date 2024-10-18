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
import {
  excludeKubeClusterWithNamespaces,
  KubeNamespaceRequest,
} from 'shared/components/AccessRequests/NewRequest/kube';
import { PendingKubeResourceItem } from 'shared/components/AccessRequests/NewRequest/RequestCheckout/RequestCheckout';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  PendingAccessRequest,
  extractResourceRequestProperties,
  ResourceRequest,
  toResourceRequest,
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

    const data = getPendingAccessRequestsPerResource({
      pendingRequest: pendingAccessRequest,
      excludeSubResourceParentResource: true,
    });
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
              subResourceName: d.subResourceName || '',
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

  /**
   *
   * @param pendingRequest holds a list or map of resources to process
   * @param excludeSubResourceParentResource when true, resources that have
   * subresources, will be excluded from the returned list. eg:
   * if a kube_cluster resource has a list of namespaces (subresources),
   * then if this flag is true, kube_cluster will be excluded from the result.
   */
  function getPendingAccessRequestsPerResource({
    pendingRequest,
    excludeSubResourceParentResource = false,
  }: {
    pendingRequest: PendingAccessRequest;
    excludeSubResourceParentResource?: boolean;
  }): PendingListItemWithOriginalItem[] {
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
          // If this request is a kube cluster and has namespaces
          // extract each as own request.
          if (
            resourceRequest.kind === 'kube' &&
            resourceRequest.resource.namespaces?.size > 0
          ) {
            // Process each namespace.
            resourceRequest.resource.namespaces.forEach(namespaceRequest => {
              const { kind, id, name } =
                extractResourceRequestProperties(namespaceRequest);

              const item = {
                kind,
                id,
                name,
                subResourceName: name,
                originalItem: namespaceRequest,
                clusterName: ctx.clustersService.findClusterByResource(
                  namespaceRequest.resource.uri
                )?.name,
              };
              data.push(item);
            });
            if (excludeSubResourceParentResource) {
              return;
            }
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
          data.push(item);
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

  async function bulkToggleKubeResources(
    items: PendingKubeResourceItem[],
    kubeCluster: PendingListKubeClusterWithOriginalItem
  ) {
    await workspaceAccessRequest.addOrRemoveResources(
      items.map(item => {
        return toResourceRequest({
          kind: item.kind,
          resourceId: item.id,
          resourceName: item.subResourceName,
          clusterUri: kubeCluster.originalItem.resource.uri,
        });
      })
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
    const data = getPendingAccessRequestsPerResource({
      pendingRequest: pendingAccessRequest,
      excludeSubResourceParentResource: true,
    });

    const params: CreateAccessRequestRequest = {
      rootClusterUri,
      reason: req.reason,
      suggestedReviewers: req.suggestedReviewers || [],
      dryRun: req.dryRun,
      resourceIds: data
        .filter(d => d.kind !== 'role')
        .filter(d => excludeKubeClusterWithNamespaces(d, data))
        .map(d => {
          if (d.kind === 'namespace') {
            return {
              name: d.id,
              kind: d.kind,
              clusterName: d.clusterName,
              subResourceName: d.subResourceName,
            };
          }
          return {
            name: d.id,
            clusterName: d.clusterName,
            kind: d.kind,
            subResourceName: '',
          };
        }),
      roles: data.filter(d => d.kind === 'role').map(d => d.name),
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
          requestedCount: data.filter(d =>
            excludeKubeClusterWithNamespaces(d, data)
          ).length,
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
    pendingAccessRequest?.kind === 'resource' &&
    Array.from(pendingAccessRequest.resources.values()).some(a =>
      routing.isLeafCluster(a.resource.uri)
    );

  return {
    showCheckout,
    isCollapsed,
    assumedRequests: getAssumedRequests(),
    toggleResource,
    data: getPendingAccessRequestsPerResource({
      pendingRequest: pendingAccessRequest,
    }),
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
  originalItem: ResourceRequest;
};
