/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useCallback, useState } from 'react';

import { Flex } from 'design';

import {
  UnifiedResources as SharedUnifiedResources,
  useUnifiedResourcesFetch,
} from 'shared/components/UnifiedResources';
import { UnifiedResourcesPinning } from 'shared/components/UnifiedResources/types';

import useStickyClusterId from 'teleport/useStickyClusterId';
import localStorage from 'teleport/services/localStorage';
import { useUser } from 'teleport/User/UserContext';
import { useTeleport } from 'teleport';
import { useUrlFiltering } from 'teleport/components/hooks';
import { UnifiedTabPreference } from 'teleport/services/userPreferences/types';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import {
  FeatureHeader,
  FeatureHeaderTitle,
  FeatureBox,
} from 'teleport/components/Layout';
import AgentButtonAdd from 'teleport/components/AgentButtonAdd';
import { SearchResource } from 'teleport/Discover/SelectResource';
import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';

import { ResourceActionButton } from './ResourceActionButton';
import SearchPanel from './SearchPanel';

export function UnifiedResources() {
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const enabled = localStorage.areUnifiedResourcesEnabled();

  if (!enabled) {
    history.replace(cfg.getNodesRoute(clusterId));
  }

  return (
    <ClusterResources
      key={clusterId} // when the current cluster changes, remount the component
      clusterId={clusterId}
      isLeafCluster={isLeafCluster}
    />
  );
}

function ClusterResources({
  clusterId,
  isLeafCluster,
}: {
  clusterId: string;
  isLeafCluster: boolean;
}) {
  const teleCtx = useTeleport();

  const pinningNotSupported = localStorage.arePinnedResourcesDisabled();
  const {
    getClusterPinnedResources,
    preferences,
    updatePreferences,
    updateClusterPinnedResources,
  } = useUser();
  const canCreate = teleCtx.storeUser.getTokenAccess().create;

  const { params, setParams, replaceHistory, pathname, onLabelClick } =
    useUrlFiltering({
      sort: {
        fieldName: 'name',
        dir: 'ASC',
      },
      pinnedOnly:
        preferences.unifiedResourcePreferences.defaultTab ===
        UnifiedTabPreference.Pinned,
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
        try {
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
        } catch (err) {
          if (!localStorage.areUnifiedResourcesEnabled()) {
            history.replace(cfg.getNodesRoute(clusterId));
          } else {
            throw err;
          }
        }
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
    <FeatureBox px={4}>
      <SharedUnifiedResources
        params={params}
        fetchResources={fetch}
        resourcesFetchAttempt={attempt}
        updateUnifiedResourcesPreferences={preferences => {
          updatePreferences({ unifiedResourcePreferences: preferences });
        }}
        availableKinds={[
          'app',
          'db',
          'windows_desktop',
          'kube_cluster',
          'node',
        ]}
        pinning={pinning}
        onLabelClick={onLabelClick}
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
          replaceHistory(
            encodeUrlQueryParams(
              pathname,
              newParams.search,
              newParams.sort,
              newParams.kinds,
              !!newParams.query /* isAdvancedSearch */,
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
    </FeatureBox>
  );
}

const emptyStateInfo: EmptyStateInfo = {
  title: 'Add your first resource to Teleport',
  byline:
    'Connect SSH servers, Kubernetes clusters, Windows Desktops, Databases, Web apps and more from our integrations catalog.',
  readOnly: {
    title: 'No Resources Found',
    resource: 'resources',
  },
  resourceType: 'unified_resource',
};
