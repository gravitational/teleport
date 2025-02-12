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

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import NodeList from 'teleport/components/NodeList';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import useTeleport from 'teleport/useTeleport';
import AgentButtonAdd from 'teleport/components/AgentButtonAdd';

import { SearchResource } from 'teleport/Discover/SelectResource';

import { State, useNodes } from './useNodes';

export default function Container() {
  const teleCtx = useTeleport();
  const state = useNodes(teleCtx);
  return <Nodes {...state} />;
}

export function Nodes(props: State) {
  const {
    fetchedData,
    getNodeLoginOptions,
    startSshSession,
    attempt,
    canCreate,
    isLeafCluster,
    clusterId,
    fetchNext,
    fetchPrev,
    params,
    pageSize,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    fetchStatus,
    isSearchEmpty,
    pageIndicators,
    onLabelClick,
  } = props;

  function onLoginSelect(e: React.MouseEvent, login: string, serverId: string) {
    e.preventDefault();
    startSshSession(login, serverId);
  }

  const hasNoNodes =
    attempt.status === 'success' &&
    fetchedData.agents.length === 0 &&
    isSearchEmpty;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Servers</FeatureHeaderTitle>
        {attempt.status === 'success' && !hasNoNodes && (
          <AgentButtonAdd
            agent={SearchResource.SERVER}
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
      {attempt.status !== 'processing' && !hasNoNodes && (
        <NodeList
          nodes={fetchedData.agents}
          onLoginMenuOpen={getNodeLoginOptions}
          onLoginSelect={onLoginSelect}
          fetchNext={fetchNext}
          fetchPrev={fetchPrev}
          fetchStatus={fetchStatus}
          pageSize={pageSize}
          pageIndicators={pageIndicators}
          params={params}
          setParams={setParams}
          setSort={setSort}
          pathname={pathname}
          replaceHistory={replaceHistory}
          onLabelClick={onLabelClick}
        />
      )}
      {attempt.status === 'success' && hasNoNodes && (
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
  title: 'Add your first server to Teleport',
  byline:
    'Teleport Server Access consolidates SSH access across all environments.',
  docsURL:
    'https://goteleport.com/docs/enroll-resources/server-access/getting-started/',
  resourceType: SearchResource.SERVER,
  readOnly: {
    title: 'No Servers Found',
    resource: 'servers',
  },
};
