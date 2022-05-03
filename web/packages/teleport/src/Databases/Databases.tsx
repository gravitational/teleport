/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
import DatabaseList from './DatabaseList';
import useDatabases, { State } from './useDatabases';
import ButtonAdd from './ButtonAdd';
import AddDatabase from './AddDatabase';

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
    showAddDialog,
    hideAddDialog,
    isAddDialogVisible,
    isEnterprise,
    username,
    version,
    clusterId,
    authType,
    results,
    fetchNext,
    fetchPrev,
    from,
    to,
    pageSize,
    params,
    setParams,
    startKeys,
    setSort,
    pathname,
    replaceHistory,
    fetchStatus,
    isSearchEmpty,
  } = props;

  const hasNoDatabases =
    attempt.status === 'success' &&
    results.databases.length === 0 &&
    isSearchEmpty;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Databases</FeatureHeaderTitle>
        {!hasNoDatabases && (
          <ButtonAdd
            isLeafCluster={isLeafCluster}
            canCreate={canCreate}
            onClick={showAddDialog}
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
        <>
          <DatabaseList
            databases={results.databases}
            username={username}
            clusterId={clusterId}
            authType={authType}
            fetchNext={fetchNext}
            fetchPrev={fetchPrev}
            fetchStatus={fetchStatus}
            from={from}
            to={to}
            totalCount={results.totalCount}
            pageSize={pageSize}
            params={params}
            setParams={setParams}
            startKeys={startKeys}
            setSort={setSort}
            pathname={pathname}
            replaceHistory={replaceHistory}
          />
        </>
      )}
      {hasNoDatabases && (
        <Empty
          clusterId={clusterId}
          canCreate={canCreate && !isLeafCluster}
          onClick={showAddDialog}
          emptyStateInfo={emptyStateInfo}
        />
      )}
      {isAddDialogVisible && (
        <AddDatabase
          isEnterprise={isEnterprise}
          username={username}
          version={version}
          authType={authType}
          onClose={hideAddDialog}
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
  resourceType: 'database',
  readOnly: {
    title: 'No Databases Found',
    resource: 'databases',
  },
};
