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

import React from 'react';
import { Indicator, Box } from 'design';

import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import ErrorMessage from 'teleport/components/AgentErrorMessage';

import AgentButtonAdd from 'teleport/components/AgentButtonAdd';

import AppList from './AppList';
import { useApps, State } from './useApps';

export default function Container() {
  const ctx = useTeleport();
  const state = useApps(ctx);
  return <Apps {...state} />;
}

export function Apps(props: State) {
  const {
    clusterId,
    isLeafCluster,
    canCreate,
    attempt,
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
    pageIndicators,
    onLabelClick,
  } = props;

  const hasNoApps =
    attempt.status === 'success' &&
    fetchedData.agents.length === 0 &&
    isSearchEmpty;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Applications</FeatureHeaderTitle>
        {attempt.status === 'success' && !hasNoApps && (
          <AgentButtonAdd
            agent="application"
            beginsWithVowel={true}
            isLeafCluster={isLeafCluster}
            canCreate={canCreate}
          />
        )}
      </FeatureHeader>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && (
        <ErrorMessage message={attempt.statusText} />
      )}
      {attempt.status !== 'processing' && !hasNoApps && (
        <AppList
          apps={fetchedData.agents}
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
        />
      )}
      {attempt.status === 'success' && hasNoApps && (
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
  title: 'Add your first application to Teleport',
  byline:
    'Teleport Application Access provides secure access to internal applications.',
  docsURL: 'https://goteleport.com/docs/application-access/getting-started/',
  resourceType: 'application',
  readOnly: {
    title: 'No Applications Found',
    resource: 'applications',
  },
};
