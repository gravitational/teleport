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

import React, { useCallback, useState } from 'react';

import { Flex } from 'design';
import { Danger } from 'design/Alert';

import {
  FilterKind,
  UnifiedResources as SharedUnifiedResources,
  useUnifiedResourcesFetch,
  UnifiedResourcesPinning,
} from 'shared/components/UnifiedResources';
import { ClusterDropdown } from 'shared/components/ClusterDropdown/ClusterDropdown';

import { DefaultTab } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import useStickyClusterId from 'teleport/useStickyClusterId';
import { storageService } from 'teleport/services/storageService';
import { useUser } from 'teleport/User/UserContext';
import { useTeleport } from 'teleport';
import { useUrlFiltering } from 'teleport/components/hooks';
import {
  FeatureHeader,
  FeatureHeaderTitle,
  FeatureBox,
} from 'teleport/components/Layout';
import { useNoMinWidth } from 'teleport/Main';
import AgentButtonAdd from 'teleport/components/AgentButtonAdd';
import { SearchResource } from 'teleport/Discover/SelectResource';
import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import { FeatureFlags } from 'teleport/types';

import { ResourceActionButton } from './ResourceActionButton';
import SearchPanel from './SearchPanel';

export function UnifiedResources() {
  const { clusterId, isLeafCluster } = useStickyClusterId();

  return (
    <FeatureBox px={4}>
      <ClusterResources
        key={clusterId} // when the current cluster changes, remount the component
        clusterId={clusterId}
        isLeafCluster={isLeafCluster}
      />
    </FeatureBox>
  );
}

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
  ];
};

export function ClusterResources({
  clusterId,
  isLeafCluster,
}: {
  clusterId: string;
  isLeafCluster: boolean;
}) {
  const teleCtx = useTeleport();
  const flags = teleCtx.getFeatureFlags();

  useNoMinWidth();

  const pinningNotSupported = storageService.arePinnedResourcesDisabled();
  const {
    getClusterPinnedResources,
    preferences,
    updatePreferences,
    updateClusterPinnedResources,
  } = useUser();
  const canCreate = teleCtx.storeUser.getTokenAccess().create;
  const [loadClusterError, setLoadClusterError] = useState('');

  const { params, setParams, replaceHistory, pathname } = useUrlFiltering({
    sort: {
      fieldName: 'name',
      dir: 'ASC',
    },
    pinnedOnly:
      preferences?.unifiedResourcePreferences?.defaultTab === DefaultTab.PINNED,
  });

  const getCurrentClusterPinnedResources = useCallback(
    () => getClusterPinnedResources(clusterId),
    [clusterId, getClusterPinnedResources]
  );
  const updateCurrentClusterPinnedResources = (pinnedResources: string[]) =>
    updateClusterPinnedResources(clusterId, pinnedResources);

  const pinning: UnifiedResourcesPinning = pinningNotSupported
    ? { kind: 'not-supported' }
    : {
        kind: 'supported',
        updateClusterPinnedResources: updateCurrentClusterPinnedResources,
        getClusterPinnedResources: getCurrentClusterPinnedResources,
      };

  const { fetch, resources, attempt, clear } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        const response = await teleCtx.resourceService.fetchUnifiedResources(
          clusterId,
          {
            search: params.search,
            query: params.query,
            pinnedOnly: params.pinnedOnly,
            sort: params.sort,
            kinds: params.kinds,
            searchAsRoles: '',
            limit: paginationParams.limit,
            startKey: paginationParams.startKey,
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
        teleCtx.resourceService,
      ]
    ),
  });

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

  return (
    <>
      {loadClusterError && <Danger>{loadClusterError}</Danger>}
      <SharedUnifiedResources
        params={params}
        fetchResources={fetch}
        resourcesFetchAttempt={attempt}
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
            ActionButton: <ResourceActionButton resource={resource} />,
          },
        }))}
        setParams={newParams => {
          setParams(newParams);
          const isAdvancedSearch = !!newParams.query;
          replaceHistory(
            encodeUrlQueryParams(
              pathname,
              isAdvancedSearch ? newParams.query : newParams.search,
              newParams.sort,
              newParams.kinds,
              isAdvancedSearch,
              newParams.pinnedOnly
            )
          );
        }}
        Header={
          <>
            <FeatureHeader
              css={`
                border-bottom: none;
              `}
              mb={1}
              alignItems="center"
              justifyContent="space-between"
            >
              <FeatureHeaderTitle>Resources</FeatureHeaderTitle>
              <Flex alignItems="center">
                <AgentButtonAdd
                  agent={SearchResource.UNIFIED_RESOURCE}
                  beginsWithVowel={false}
                  isLeafCluster={isLeafCluster}
                  canCreate={canCreate}
                />
              </Flex>
            </FeatureHeader>
            <Flex alignItems="center" justifyContent="space-between">
              <SearchPanel
                params={params}
                pathname={pathname}
                replaceHistory={replaceHistory}
                setParams={setParams}
              />
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
  resourceType: 'unified_resource',
};
