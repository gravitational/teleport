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
import { getErrMessage } from 'shared/utils/errorType';

import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  RdsEngineIdentifier,
  Regions,
  integrationService,
} from 'teleport/services/integrations';
import { DatabaseEngine } from 'teleport/Discover/SelectResource';
import { Database } from 'teleport/services/databases';

import { ActionButtons, Header } from '../../Shared';

import { useCreateDatabase } from '../CreateDatabase/useCreateDatabase';
import { CreateDatabaseDialog } from '../CreateDatabase/CreateDatabaseDialog';

import { AwsRegionSelector } from './AwsRegionSelector';
import { DatabaseList } from './RdsDatabaseList';

type TableData = {
  items: CheckedAwsRdsDatabase[];
  fetchStatus: FetchStatus;
  startKey?: string;
  currRegion?: Regions;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  startKey: '',
};

// CheckedAwsRdsDatabase is a type to describe that a
// AwsRdsDatabase has been checked (by its resource id)
// with the backend whether or not a database server already
// exists for it.
export type CheckedAwsRdsDatabase = AwsRdsDatabase & {
  dbServerExists?: boolean;
};

export function EnrollRdsDatabase() {
  const {
    createdDb,
    pollTimeout,
    registerDatabase,
    attempt: registerAttempt,
    clearAttempt: clearRegisterAttempt,
    nextStep,
    fetchDatabaseServers,
  } = useCreateDatabase();

  const { agentMeta, resourceSpec, emitErrorEvent } = useDiscover();
  const { attempt: fetchDbAttempt, setAttempt: setFetchDbAttempt } =
    useAttempt('');

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    startKey: '',
    fetchStatus: 'disabled',
  });
  const [selectedDb, setSelectedDb] = useState<CheckedAwsRdsDatabase>();

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

  async function fetchDatabases(data: TableData) {
    const integrationName = (agentMeta as DbMeta).integration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    setFetchDbAttempt({ status: 'processing' });

    try {
      const { databases: fetchedRdsDbs, nextToken } =
        await integrationService.fetchAwsRdsDatabases(
          integrationName,
          getRdsEngineIdentifier(resourceSpec.dbMeta?.engine),
          {
            region: data.currRegion,
            nextToken: data.startKey,
          }
        );

      // Abort if there were no rds dbs for the selected region.
      if (fetchedRdsDbs.length <= 0) {
        setFetchDbAttempt({ status: 'success' });
        setTableData({ ...data, fetchStatus: 'disabled' });
        return;
      }

      // Check if fetched rds databases have a database
      // server for it, to prevent user from enrolling
      // the same db and getting an error from it.

      // Build the predicate string that will query for
      // all the fetched rds dbs by its resource ids.
      const resourceIds: string[] = fetchedRdsDbs.map(
        d => `resource.spec.aws.rds.resource_id == "${d.resourceId}"`
      );
      const query = resourceIds.join(' || ');
      const { agents: fetchedDbServers } = await fetchDatabaseServers(
        query,
        fetchedRdsDbs.length // limit
      );

      const dbServerLookupByResourceId: Record<string, Database> = {};
      fetchedDbServers.forEach(
        d => (dbServerLookupByResourceId[d.aws.rds.resourceId] = d)
      );

      // Check for db server matches.
      const checkedRdsDbs: CheckedAwsRdsDatabase[] = fetchedRdsDbs.map(rds => {
        const dbServer = dbServerLookupByResourceId[rds.resourceId];
        if (dbServer) {
          return {
            ...rds,
            dbServerExists: true,
          };
        }
        return rds;
      });

      setFetchDbAttempt({ status: 'success' });
      setTableData({
        currRegion: data.currRegion,
        startKey: nextToken,
        fetchStatus: nextToken ? '' : 'disabled',
        // concat each page fetch.
        items: [...data.items, ...checkedRdsDbs],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchDbAttempt({ status: 'failed', statusText: errMsg });
      setTableData(data); // fallback to previous data
      emitErrorEvent(`database fetch error: ${errMsg}`);
    }
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
        awsRds: selectedDb,
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
