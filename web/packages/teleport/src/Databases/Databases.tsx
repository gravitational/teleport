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

import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import cfg from 'teleport/config';
import history from 'teleport/services/history/history';
import { storageService } from 'teleport/services/storageService';

import AgentButtonAdd from 'teleport/components/AgentButtonAdd';

import { SearchResource } from 'teleport/Discover/SelectResource';

import DatabaseList from './DatabaseList';
import { State, useDatabases } from './useDatabases';

export default function Container() {
  const ctx = useTeleport();
  const state = useDatabases(ctx);
  return <Databases {...state} />;
}

export function Databases(props: State) {
  const {
    attempt,
    isLeafCluster,
    canCreate,
    username,
    clusterId,
    authType,
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

  const hasNoDatabases =
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
        <FeatureHeaderTitle>Databases</FeatureHeaderTitle>
        {attempt.status === 'success' && !hasNoDatabases && (
          <AgentButtonAdd
            agent={SearchResource.DATABASE}
            beginsWithVowel={false}
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
      {attempt.status !== 'processing' && !hasNoDatabases && (
        <DatabaseList
          databases={fetchedData.agents}
          username={username}
          clusterId={clusterId}
          authType={authType}
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
      {attempt.status === 'success' && hasNoDatabases && (
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
  title: 'Add your first database to Teleport',
  byline:
    'Teleport Database Access provides secure access to PostgreSQL, MySQL, MariaDB, MongoDB, Redis, and Microsoft SQL Server.',
  docsURL: 'https://goteleport.com/docs/database-access/guides/',
  resourceType: SearchResource.DATABASE,
  readOnly: {
    title: 'No Databases Found',
    resource: 'databases',
  },
};
