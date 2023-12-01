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

import React from 'react';
import { Box, Indicator } from 'design';

import KubeList from 'teleport/Kubes/KubeList';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import useTeleport from 'teleport/useTeleport';
import cfg from 'teleport/config';
import history from 'teleport/services/history/history';
import { storageService } from 'teleport/services/storageService';

import AgentButtonAdd from 'teleport/components/AgentButtonAdd';

import { SearchResource } from 'teleport/Discover/SelectResource';

import { State, useKubes } from './useKubes';

export default function Container() {
  const ctx = useTeleport();
  const state = useKubes(ctx);
  return <Kubes {...state} />;
}

const DOC_URL = 'https://goteleport.com/docs/kubernetes-access/guides';

export function Kubes(props: State) {
  const {
    attempt,
    username,
    authType,
    isLeafCluster,
    clusterId,
    canCreate,
    fetchedData,
    fetchNext,
    fetchPrev,
    pageSize,
    params,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    fetchStatus,
    isSearchEmpty,
    onLabelClick,
    accessRequestId,
    pageIndicators,
  } = props;

  const hasNoKubes =
    attempt.status === 'success' &&
    fetchedData.agents.length === 0 &&
    isSearchEmpty;

  const enabled = storageService.areUnifiedResourcesEnabled();
  if (enabled) {
    history.replace(cfg.getUnifiedResourcesRoute(clusterId));
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Kubernetes</FeatureHeaderTitle>
        {attempt.status === 'success' && !hasNoKubes && (
          <AgentButtonAdd
            agent={SearchResource.KUBERNETES}
            beginsWithVowel={false}
            isLeafCluster={isLeafCluster}
            canCreate={canCreate}
          />
        )}
      </FeatureHeader>
      {attempt.status === 'failed' && (
        <ErrorMessage message={attempt.statusText} />
      )}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status !== 'processing' && !hasNoKubes && (
        <KubeList
          kubes={fetchedData.agents}
          username={username}
          authType={authType}
          clusterId={clusterId}
          fetchNext={fetchNext}
          fetchPrev={fetchPrev}
          fetchStatus={fetchStatus}
          pageIndicators={pageIndicators}
          pageSize={pageSize}
          params={params}
          setParams={setParams}
          setSort={setSort}
          pathname={pathname}
          replaceHistory={replaceHistory}
          onLabelClick={onLabelClick}
          accessRequestId={accessRequestId}
        />
      )}
      {attempt.status === 'success' && hasNoKubes && (
        <Empty
          clusterId={clusterId}
          canCreate={canCreate && !isLeafCluster}
          emptyStateInfo={emptyStateInfo}
        />
      )}
    </FeatureBox>
  );
}

const emptyStateInfo: EmptyStateInfo = {
  title: 'Add your first Kubernetes cluster to Teleport',
  byline:
    'Teleport Kubernetes Access provides secure access to Kubernetes clusters.',
  docsURL: DOC_URL,
  resourceType: SearchResource.KUBERNETES,
  readOnly: {
    title: 'No Kubernetes Clusters Found',
    resource: 'kubernetes clusters',
  },
};
