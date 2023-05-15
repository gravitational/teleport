/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import { Box } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import { Danger } from 'design/Alert';

import useAttempt from 'shared/hooks/useAttemptNext';

import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  ListAwsRdsDatabaseResponse,
  RdsEngineIdentifier,
  Regions,
  integrationService,
} from 'teleport/services/integrations';
import { DatabaseEngine } from 'teleport/Discover/SelectResource';

import { ActionButtons, Header } from '../../Shared';

import { useCreateDatabase } from '../CreateDatabase/useCreateDatabase';
import { CreateDatabaseDialog } from '../CreateDatabase/CreateDatabaseDialog';

import { AwsRegionSelector } from './AwsRegionSelector';
import { DatabaseList } from './RdsDatabaseList';

type TableData = {
  items: ListAwsRdsDatabaseResponse['databases'];
  fetchStatus: FetchStatus;
  startKey?: string;
  currRegion?: Regions;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  startKey: '',
};

export function EnrollRdsDatabase() {
  const {
    createdDb,
    pollTimeout,
    registerDatabase,
    attempt: registerAttempt,
    clearAttempt: clearRegisterAttempt,
    nextStep,
  } = useCreateDatabase();

  const { agentMeta, resourceSpec, emitErrorEvent } = useDiscover();
  const { attempt: fetchDbAttempt, setAttempt: setFetchDbAttempt } =
    useAttempt('');

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    startKey: '',
    fetchStatus: 'disabled',
  });
  const [selectedDb, setSelectedDb] = useState<AwsRdsDatabase>();

  function fetchDatabasesWithNewRegion(region: Regions) {
    // Clear table when fetching with new region.
    fetchDatabases({ ...emptyTableData, currRegion: region });
  }

  function fetchNextPage() {
    fetchDatabases({ ...tableData });
  }

  function refreshDatabaseList() {
    // When refreshing, start the table back at page 1.
    fetchDatabases({ ...tableData, startKey: '', items: [] });
  }

  function fetchDatabases(data: TableData) {
    const integrationName = (agentMeta as DbMeta).integrationName;

    setTableData({ ...data, fetchStatus: 'loading' });
    setFetchDbAttempt({ status: 'processing' });

    integrationService
      .fetchAwsRdsDatabases(
        integrationName,
        getRdsEngineIdentifier(resourceSpec.dbMeta?.engine),
        {
          region: data.currRegion,
          nextToken: data.startKey,
        }
      )
      .then(resp => {
        setFetchDbAttempt({ status: 'success' });
        setTableData({
          currRegion: data.currRegion,
          startKey: resp.nextToken,
          fetchStatus: resp.nextToken ? '' : 'disabled',
          // concat each page fetch.
          items: [...data.items, ...resp.databases],
        });
      })
      .catch((err: Error) => {
        setFetchDbAttempt({ status: 'failed', statusText: err.message });
        setTableData(data); // fallback to previous data
        emitErrorEvent(`failed to fetch aws rds list: ${err.message}`);
      });
  }

  function clear() {
    clearRegisterAttempt();

    if (fetchDbAttempt.status === 'failed') {
      setFetchDbAttempt({ status: '' });
    }
    if (tableData.items.length > 0) {
      setTableData(emptyTableData);
    }
    if (selectedDb) {
      setSelectedDb(null);
    }
  }

  function handleOnProceed() {
    registerDatabase(
      {
        name: selectedDb.name,
        protocol: selectedDb.engine,
        uri: selectedDb.uri,
        labels: selectedDb.labels,
        awsRds: {
          accountId: selectedDb.accountId,
          resourceId: selectedDb.resourceId,
        },
      },
      // Corner case where if registering db fails a user can:
      //   1) change region, which will list new databases or
      //   2) select a different database before re-trying.
      selectedDb.name !== createdDb?.name
    );
  }

  return (
    <Box maxWidth="800px">
      <Header>Enroll a RDS Database</Header>
      {fetchDbAttempt.status === 'failed' && (
        <Danger mt={3}>{fetchDbAttempt.statusText}</Danger>
      )}
      <AwsRegionSelector
        onFetch={fetchDatabasesWithNewRegion}
        onRefresh={refreshDatabaseList}
        clear={clear}
        disableSelector={fetchDbAttempt.status === 'processing'}
        disableFetch={
          fetchDbAttempt.status === 'processing' || tableData.items.length > 0
        }
      />
      <DatabaseList
        items={tableData.items}
        fetchStatus={tableData.fetchStatus}
        selectedDatabase={selectedDb}
        onSelectDatabase={setSelectedDb}
        fetchNextPage={fetchNextPage}
      />
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={fetchDbAttempt.status === 'processing' || !selectedDb}
      />
      {registerAttempt.status !== '' && (
        <CreateDatabaseDialog
          pollTimeout={pollTimeout}
          attempt={registerAttempt}
          next={nextStep}
          close={clearRegisterAttempt}
          retry={handleOnProceed}
          dbName={selectedDb.name}
        />
      )}
    </Box>
  );
}

function getRdsEngineIdentifier(engine: DatabaseEngine): RdsEngineIdentifier {
  switch (engine) {
    case DatabaseEngine.MySql:
      return 'mysql';
    case DatabaseEngine.Postgres:
      return 'postgres';
    case DatabaseEngine.AuroraMysql:
      return 'aurora-mysql';
    case DatabaseEngine.AuroraPostgres:
      return 'aurora-postgres';
  }
}
