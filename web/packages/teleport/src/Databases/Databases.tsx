/*
Copyright 2021 Gravitational, Inc.

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
import { Danger } from 'design/Alert';
import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
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
    databases,
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
  } = props;

  const isEmpty =
    (attempt.status === 'success' || attempt.status === 'processing') &&
    databases.length === 0;
  const hasDatabases = attempt.status === 'success' && databases.length > 0;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Databases</FeatureHeaderTitle>
        {!isEmpty && attempt.status !== 'processing' && (
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
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      {hasDatabases && (
        <>
          <DatabaseList
            databases={databases}
            username={username}
            clusterId={clusterId}
            authType={authType}
          />
        </>
      )}
      {isEmpty && (
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
  entityType: 'database',
  readOnly: {
    title: 'No Databases Found',
    resource: 'databases',
  },
};
