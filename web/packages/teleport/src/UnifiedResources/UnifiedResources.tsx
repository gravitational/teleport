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

import { UnifiedResources as SharedUnifiedResources } from 'shared/components/UnifiedResources';

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
  const teleCtx = useTeleport();
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const enabled = localStorage.areUnifiedResourcesEnabled();
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

  if (!enabled) {
    history.replace(cfg.getNodesRoute(clusterId));
  }

  const getCurrentClusterPinnedResources = useCallback(
    () => getClusterPinnedResources(clusterId),
    [clusterId, getClusterPinnedResources]
  );
  const updateCurrentClusterPinnedResources = useCallback(
    (pinnedResources: string[]) =>
      updateClusterPinnedResources(clusterId, pinnedResources),
    [clusterId, updateClusterPinnedResources]
  );

  return (
    <SharedUnifiedResources
      params={params}
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
      key={clusterId} // when the current cluster changes, remount the component
      getClusterPinnedResources={getCurrentClusterPinnedResources}
      updateClusterPinnedResources={updateCurrentClusterPinnedResources}
      pinningNotSupported={pinningNotSupported}
      onLabelClick={onLabelClick}
      EmptySearchResults={
        <Empty
          clusterId={clusterId}
          canCreate={canCreate && !isLeafCluster}
          emptyStateInfo={emptyStateInfo}
        />
      }
      fetchFunc={useCallback(
        async (params, signal) => {
          const resp = await teleCtx.resourceService.fetchUnifiedResources(
            clusterId,
            params,
            signal
          );

          return {
            startKey: resp.startKey,
            agents: resp.agents,
            totalCount: resp.agents.length,
          };
        },
        [clusterId, teleCtx.resourceService]
      )}
      mapToResource={resource => ({
        resource,
        ui: {
          ActionButton: <ResourceActionButton resource={resource} />,
        },
      })}
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
