/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useCallback, useMemo, useState, type JSX } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { Danger } from 'design/Alert';
import { DefaultTab } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  BulkAction,
  FilterKind,
  IncludedResourceMode,
  ResourceAvailabilityFilter,
  UnifiedResources as SharedUnifiedResources,
  UnifiedResourceDefinition,
  UnifiedResourcesPinning,
  useUnifiedResourcesFetch,
} from 'shared/components/UnifiedResources';
import { buildPredicateExpression } from 'shared/components/UnifiedResources/shared/predicateExpression';
import {
  getResourceId,
  openStatusInfoPanel,
} from 'shared/components/UnifiedResources/shared/StatusInfo';

import { useTeleport } from 'teleport';
import AgentButtonAdd from 'teleport/components/AgentButtonAdd';
import { ClusterDropdown } from 'teleport/components/ClusterDropdown/ClusterDropdown';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import { useUrlFiltering } from 'teleport/components/hooks';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { ServersideSearchPanel } from 'teleport/components/ServersideSearchPanel';
import cfg from 'teleport/config';
import { SearchResource } from 'teleport/Discover/SelectResource';
import { useNoMinWidth } from 'teleport/Main';
import {
  SamlAppActionProvider,
  useSamlAppAction,
} from 'teleport/SamlApplications/useSamlAppActions';
import { UnifiedResource } from 'teleport/services/agents';
import { FeatureFlags } from 'teleport/types';
import { useUser } from 'teleport/User/UserContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { ResourceActionButton } from './ResourceActionButton';
import { StatusInfo } from './StatusInfo';

export function UnifiedResources() {
  const { clusterId, isLeafCluster } = useStickyClusterId();

  return (
    <FeatureBox px={4}>
      <ResizingResourceWrapper>
        <SamlAppActionProvider>
          <ClusterResources
            key={clusterId} // when the current cluster changes, remount the component
            clusterId={clusterId}
            isLeafCluster={isLeafCluster}
          />
        </SamlAppActionProvider>
      </ResizingResourceWrapper>
    </FeatureBox>
  );
}

const ResizingResourceWrapper = styled(Box)`
  width: 100%;
  padding-right: ${props => props.theme.space[3]}px;
`;

const getAvailableKindsWithAccess = (flags: FeatureFlags): FilterKind[] => {
  return [
    {
      kind: 'node',
      disabled: !flags.nodes,
    },
    {
      kind: 'app',
      disabled: !flags.applications,
    },
    {
      kind: 'db',
      disabled: !flags.databases,
    },
    {
      kind: 'kube_cluster',
      disabled: !flags.kubernetes,
    },
    {
      kind: 'windows_desktop',
      disabled: !flags.desktops,
    },
    {
      kind: 'git_server',
      disabled: !flags.gitServers,
    },
  ];
};

export function ClusterResources({
  clusterId,
  isLeafCluster,
  getActionButton,
  showCheckout = false,
  availabilityFilter,
  bulkActions = [],
}: {
  clusterId: string;
  isLeafCluster: boolean;
  getActionButton?: (
    resource: UnifiedResource,
    includedResourceMode: IncludedResourceMode
  ) => JSX.Element;
  showCheckout?: boolean;
  /** A list of actions that can be performed on the selected items. */
  bulkActions?: BulkAction[];
  availabilityFilter?: ResourceAvailabilityFilter;
}) {
  const teleCtx = useTeleport();
  const flags = teleCtx.getFeatureFlags();

  useNoMinWidth();

  const {
    getClusterPinnedResources,
    preferences,
    updatePreferences,
    updateClusterPinnedResources,
  } = useUser();
  const canCreate = teleCtx.storeUser.getTokenAccess().create;
  const [loadClusterError, setLoadClusterError] = useState('');

  const { params, setParams } = useUrlFiltering(
    {
      sort: {
        fieldName: 'name',
        dir: 'ASC',
      },
      pinnedOnly:
        preferences?.unifiedResourcePreferences?.defaultTab ===
        DefaultTab.PINNED,
    },
    availabilityFilter?.mode
  );

  const getCurrentClusterPinnedResources = useCallback(
    () => getClusterPinnedResources(clusterId),
    [clusterId, getClusterPinnedResources]
  );
  const updateCurrentClusterPinnedResources = (pinnedResources: string[]) =>
    updateClusterPinnedResources(clusterId, pinnedResources);

  const pinning: UnifiedResourcesPinning = {
    kind: 'supported',
    updateClusterPinnedResources: updateCurrentClusterPinnedResources,
    getClusterPinnedResources: getCurrentClusterPinnedResources,
  };

  const {
    fetch,
    resources: unfilteredResources,
    attempt,
    clear,
  } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        const response = await teleCtx.resourceService.fetchUnifiedResources(
          clusterId,
          {
            search: params.search,
            query: buildPredicateExpression(params.statuses, params.query),
            pinnedOnly: params.pinnedOnly,
            sort: params.sort,
            kinds: params.kinds,
            searchAsRoles: '',
            limit: paginationParams.limit,
            startKey: paginationParams.startKey,
            includedResourceMode: params.includedResourceMode,
          },
          signal
        );

        return {
          startKey: response.startKey,
          agents: response.agents,
          totalCount: response.agents.length,
        };
      },
      [
        clusterId,
        params.kinds,
        params.pinnedOnly,
        params.query,
        params.search,
        params.sort,
        params.includedResourceMode,
        params.statuses,
        teleCtx.resourceService,
      ]
    ),
  });
  const { samlAppToDelete } = useSamlAppAction();
  const resources = useMemo(
    () =>
      samlAppToDelete?.backendDeleted
        ? unfilteredResources.filter(
            res =>
              !(
                res.kind === 'app' &&
                res.samlApp &&
                res.name === samlAppToDelete.name
              )
          )
        : unfilteredResources,
    [samlAppToDelete, unfilteredResources]
  );

  // This state is used to recognize when the `params` value has changed,
  // and reset the overall state of `useUnifiedResourcesFetch` hook. It's tempting to use a
  // `useEffect` here, but doing so can cause unwanted behavior where the previous,
  // now stale `fetch` is executed once more before the new one (with the new
  // `filter`) is executed. This is because the `useEffect` is
  // executed after the render, and `fetch` is called by an IntersectionObserver
  // in `useInfiniteScroll`. If the render includes `useInfiniteScroll`'s `trigger`
  // element, the old, stale `fetch` will be called before `useEffect` has a chance
  // to run and update the state, and thereby the `fetch` function.
  //
  // By using the pattern described in this article:
  // https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes,
  // we can ensure that the state is reset before anything renders, and thereby
  // ensure that the new `fetch` function is used.
  const [prevParams, setPrevParams] = useState(params);
  if (prevParams !== params) {
    setPrevParams(params);
    clear();
  }

  const { setInfoGuideConfig } = useInfoGuide();
  function onShowStatusInfo(resource: UnifiedResourceDefinition) {
    openStatusInfoPanel({
      isEnterprise: cfg.edition === 'ent',
      resource,
      setInfoGuideConfig,
      guide: (
        <StatusInfo
          resource={resource}
          clusterId={clusterId}
          key={getResourceId(resource)}
        />
      ),
    });
  }

  return (
    <>
      {loadClusterError && <Danger>{loadClusterError}</Danger>}
      <SharedUnifiedResources
        onShowStatusInfo={onShowStatusInfo}
        bulkActions={bulkActions}
        params={params}
        fetchResources={fetch}
        resourcesFetchAttempt={attempt}
        availabilityFilter={availabilityFilter}
        unifiedResourcePreferences={preferences.unifiedResourcePreferences}
        updateUnifiedResourcesPreferences={preferences => {
          updatePreferences({ unifiedResourcePreferences: preferences });
        }}
        availableKinds={getAvailableKindsWithAccess(flags)}
        pinning={pinning}
        ClusterDropdown={
          <ClusterDropdown
            clusterLoader={teleCtx.clusterService}
            clusterId={clusterId}
            onError={setLoadClusterError}
          />
        }
        NoResources={
          <Empty
            clusterId={clusterId}
            canCreate={canCreate && !isLeafCluster}
            emptyStateInfo={emptyStateInfo}
          />
        }
        resources={resources.map(resource => ({
          resource,
          ui: {
            ActionButton: getActionButton?.(
              resource,
              params.includedResourceMode
            ) || <ResourceActionButton resource={resource} />,
          },
        }))}
        setParams={setParams}
        Header={
          <>
            <FeatureHeader
              mb={1}
              alignItems="center"
              justifyContent="space-between"
            >
              <FeatureHeaderTitle>Resources</FeatureHeaderTitle>
              <Flex alignItems="center">
                {!showCheckout && (
                  <AgentButtonAdd
                    agent={SearchResource.UNIFIED_RESOURCE}
                    beginsWithVowel={false}
                    isLeafCluster={isLeafCluster}
                    canCreate={canCreate}
                  />
                )}
              </Flex>
            </FeatureHeader>
            <Flex alignItems="center" justifyContent="space-between" mb={3}>
              <ServersideSearchPanel params={params} setParams={setParams} />
            </Flex>
          </>
        }
      />
    </>
  );
}

export const emptyStateInfo: EmptyStateInfo = {
  title: 'Add your first resource to Teleport',
  byline:
    'Connect SSH servers, Kubernetes clusters, Windows Desktops, Databases, Web apps and more from our integrations catalog.',
  readOnly: {
    title: 'No Resources Found',
    resource: 'resources',
  },
};
