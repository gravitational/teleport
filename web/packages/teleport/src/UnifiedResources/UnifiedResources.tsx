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

import React, { useCallback } from 'react';

import { Flex } from 'design';

import {
  UnifiedResources as SharedUnifiedResources,
  UnifiedResourcesPinning,
  useUnifiedResourcesFetch,
} from 'shared/components/UnifiedResources';

import useStickyClusterId from 'teleport/useStickyClusterId';
import localStorage from 'teleport/services/localStorage';
import { useUser } from 'teleport/User/UserContext';
import { useTeleport } from 'teleport';
import { useUrlFiltering } from 'teleport/components/hooks';
import { UnifiedTabPreference } from 'teleport/services/userPreferences/types';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
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
    <Wrapper
      key={clusterId} // when the current cluster changes, remount the component
      clusterId={clusterId}
      isLeafCluster={isLeafCluster}
    />
  );
}

function Wrapper({
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

  const { fetch, resources, attempt } = useUnifiedResourcesFetch({
    params: params,
    fetchFunc: useCallback(
      async (params, signal) => {
        const response = await teleCtx.resourceService.fetchUnifiedResources(
          clusterId,
          params,
          signal
        );

        return {
          startKey: response.startKey,
          agents: response.agents,
          totalCount: response.agents.length,
        };
      },
      [clusterId, teleCtx.resourceService]
    ),
  });

  return (
    <SharedUnifiedResources
      params={params}
      fetchResources={fetch}
      resourcesFetchAttempt={attempt}
      updateUnifiedResourcesPreferences={preferences => {
        updatePreferences({ unifiedResourcePreferences: preferences });
      }}
      availableKinds={['app', 'db', 'windows_desktop', 'kube_cluster', 'node']}
      Header={pinAllButton => (
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
            {pinAllButton}
          </Flex>
        </>
      )}
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
      pinning={pinning}
      onLabelClick={onLabelClick}
      EmptySearchResults={
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
    />
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
