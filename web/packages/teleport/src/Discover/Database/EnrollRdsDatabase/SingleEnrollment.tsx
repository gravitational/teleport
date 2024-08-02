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

import React, { useState, useEffect } from 'react';
import { Text } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  Regions,
  Vpc,
  integrationService,
} from 'teleport/services/integrations';
import { Database } from 'teleport/services/databases';
import { getRdsEngineIdentifier } from 'teleport/Discover/SelectResource/types';

import { ActionButtons } from '../../Shared';

import { useCreateDatabase } from '../CreateDatabase/useCreateDatabase';
import { CreateDatabaseDialog } from '../CreateDatabase/CreateDatabaseDialog';

import { DatabaseList } from './RdsDatabaseList';

type TableData = {
  items: CheckedAwsRdsDatabase[];
  fetchStatus: FetchStatus;
  startKey?: string;
};

const emptyTableData = (): TableData => ({
  items: [],
  fetchStatus: 'disabled',
  startKey: '',
});

// CheckedAwsRdsDatabase is a type to describe that a
// AwsRdsDatabase has been checked (by its resource id)
// with the backend whether or not a database server already
// exists for it.
export type CheckedAwsRdsDatabase = AwsRdsDatabase & {
  dbServerExists?: boolean;
};

export function SingleEnrollment({
  region,
  vpc,
  disableBtns,
  onFetchAttempt,
  fetchAttempt,
}: {
  region: Regions;
  vpc?: Vpc;
  disableBtns: boolean;
  fetchAttempt: Attempt;
  onFetchAttempt(a: Attempt): void;
  /**
   * key is expected to be set to the ID of the VPC.
   */
  key: string;
}) {
  const {
    createdDb,
    pollTimeout,
    registerDatabase,
    attempt,
    clearAttempt, // TODO
    nextStep,
    fetchDatabaseServers,
  } = useCreateDatabase();

  const { agentMeta, resourceSpec, emitErrorEvent } = useDiscover();

  const [tableData, setTableData] = useState<TableData>();
  const [selectedDb, setSelectedDb] = useState<CheckedAwsRdsDatabase>();

  useEffect(() => {
    if (vpc) {
      // Start with empty table data for new vpc's.
      fetchRdsDatabases(emptyTableData(), vpc);
    }
  }, [vpc]);

  function fetchNextPage() {
    fetchRdsDatabases({ ...tableData }, vpc);
  }

  async function fetchRdsDatabases(data: TableData, vpc: Vpc) {
    const integrationName = agentMeta.awsIntegration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    onFetchAttempt({ status: 'processing' });

    try {
      const { databases: fetchedDbs, nextToken } =
        await integrationService.fetchAwsRdsDatabases(
          integrationName,
          getRdsEngineIdentifier(resourceSpec.dbMeta?.engine),
          {
            region: region,
            nextToken: data.startKey,
            vpcId: vpc.id,
          }
        );

      // Abort early if there were no rds dbs for the selected region.
      if (fetchedDbs.length <= 0) {
        onFetchAttempt({ status: 'success' });
        setTableData({ ...data, fetchStatus: 'disabled' });
        return;
      }

      // Check if fetched rds databases have a database
      // server for it, to prevent user from enrolling
      // the same db and getting an error from it.

      // Build the predicate string that will query for
      // all the fetched rds dbs by its resource ids.
      const resourceIds: string[] = fetchedDbs.map(
        d => `resource.spec.aws.rds.resource_id == "${d.resourceId}"`
      );
      const query = resourceIds.join(' || ');

      const { agents: fetchedDbServers } = await fetchDatabaseServers(query);

      const dbServerLookupByResourceId: Record<string, Database> = {};
      fetchedDbServers.forEach(
        d => (dbServerLookupByResourceId[d.aws.rds.resourceId] = d)
      );

      // Check for db server matches.
      const checkedRdsDbs: CheckedAwsRdsDatabase[] = fetchedDbs.map(rds => {
        const dbServer = dbServerLookupByResourceId[rds.resourceId];
        if (dbServer) {
          return {
            ...rds,
            dbServerExists: true,
          };
        }
        return rds;
      });

      onFetchAttempt({ status: 'success' });
      setTableData({
        startKey: nextToken,
        fetchStatus: nextToken ? '' : 'disabled',
        // concat each page fetch.
        items: [
          ...data.items,
          ...checkedRdsDbs.sort((a, b) => a.name.localeCompare(b.name)),
        ],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      onFetchAttempt({ status: 'failed', statusText: errMsg });
      setTableData(data); // fallback to previous data
      emitErrorEvent(`database fetch error: ${errMsg}`);
    }
  }

  function handleOnProceed() {
    const isNewDb = selectedDb.name !== createdDb?.name;
    registerDatabase(
      {
        name: selectedDb.name,
        protocol: selectedDb.engine,
        uri: selectedDb.uri,
        labels: selectedDb.labels,
        awsRds: selectedDb,
        awsRegion: region,
        awsVpcId: vpc.id,
      },
      // Corner case where if registering db fails a user can:
      //   1) change region, which will list new databases or
      //   2) select a different database before re-trying.
      isNewDb
    );
  }

  const showTable = !!vpc && fetchAttempt.status !== 'failed';

  return (
    <>
      {showTable && (
        <>
          <Text mt={3}>Select an RDS to enroll:</Text>
          <DatabaseList
            wantAutoDiscover={false}
            items={tableData?.items || []}
            fetchStatus={tableData?.fetchStatus || 'loading'}
            selectedDatabase={selectedDb}
            onSelectDatabase={setSelectedDb}
            fetchNextPage={fetchNextPage}
          />
        </>
      )}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={disableBtns || !showTable || !selectedDb}
      />
      {attempt.status !== '' && (
        <CreateDatabaseDialog
          pollTimeout={pollTimeout}
          attempt={attempt}
          next={nextStep}
          close={clearAttempt}
          retry={handleOnProceed}
          dbName={selectedDb.name}
        />
      )}
    </>
  );
}
