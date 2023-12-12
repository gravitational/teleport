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

import React, { useState } from 'react';
import { Box, Link, Text, Toggle } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import { Danger } from 'design/Alert';

import useAttempt from 'shared/hooks/useAttemptNext';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  RdsEngineIdentifier,
  Regions,
  integrationService,
} from 'teleport/services/integrations';
import { DatabaseEngine } from 'teleport/Discover/SelectResource';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import { Database } from 'teleport/services/databases';
import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';
import { isIamPermError } from 'teleport/Discover/Shared/Aws/error';
import cfg from 'teleport/config';

import { ActionButtons, Header, Mark } from '../../Shared';

import { useCreateDatabase } from '../CreateDatabase/useCreateDatabase';
import { CreateDatabaseDialog } from '../CreateDatabase/CreateDatabaseDialog';

import { DatabaseList } from './RdsDatabaseList';
import { AutoEnrollDialog } from './AutoEnrollDialog';

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
    enableAutoDiscovery,
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
  const [wantAutoDiscover, setWantAutoDiscover] = useState(true);

  function fetchDatabasesWithNewRegion(region: Regions) {
    // Clear table when fetching with new region.
    fetchDatabases({ ...emptyTableData, currRegion: region });
  }

  function fetchNextPage() {
    fetchDatabases({ ...tableData });
  }

  function refreshDatabaseList() {
    setSelectedDb(null);
    // When refreshing, start the table back at page 1.
    fetchDatabases({ ...tableData, startKey: '', items: [] });
  }

  async function fetchDatabases(data: TableData) {
    const integrationName = agentMeta.awsIntegration.name;

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
    if (wantAutoDiscover) {
      enableAutoDiscovery({ selectedRegion: tableData.currRegion });
    } else {
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
  }

  const hasIamPermError = isIamPermError(fetchDbAttempt);
  let DialogComponent;
  if (registerAttempt.status !== '') {
    if (wantAutoDiscover) {
      DialogComponent = (
        <AutoEnrollDialog
          attempt={registerAttempt}
          next={nextStep}
          close={clearRegisterAttempt}
          retry={handleOnProceed}
          region={tableData.currRegion}
        />
      );
    } else {
      DialogComponent = (
        <CreateDatabaseDialog
          pollTimeout={pollTimeout}
          attempt={registerAttempt}
          next={nextStep}
          close={clearRegisterAttempt}
          retry={handleOnProceed}
          dbName={selectedDb.name}
        />
      );
    }
  }

  const showTable = !hasIamPermError && tableData.currRegion;

  return (
    <Box maxWidth="800px">
      <Header>Enroll a RDS Database</Header>
      {fetchDbAttempt.status === 'failed' && !hasIamPermError && (
        <Danger mt={3}>{fetchDbAttempt.statusText}</Danger>
      )}
      <Text mt={4}>
        Select the AWS Region you would like to see databases for:
      </Text>
      <AwsRegionSelector
        onFetch={fetchDatabasesWithNewRegion}
        onRefresh={refreshDatabaseList}
        clear={clear}
        disableSelector={fetchDbAttempt.status === 'processing'}
      />
      {showTable && (
        <>
          <Box mb={2}>
            <Toggle
              isToggled={wantAutoDiscover}
              onToggle={() => setWantAutoDiscover(p => !p)}
              disabled={tableData.items.length === 0}
            >
              <Box ml={2} mr={1}>
                Auto-Enroll all Databases for selected region
              </Box>
              <ToolTipInfo>
                Auto-Enroll will automatically identify all RDS databases from
                the selected region and register them as database resources in
                your infrastructure.
              </ToolTipInfo>
            </Toggle>
            {!cfg.isCloud && wantAutoDiscover && (
              <Box mt={2} mb={3}>
                Auto-enrolling requires you to setup a{' '}
                <Mark>Discovery Service</Mark>. <br /> Follow{' '}
                <Link
                  target="_blank"
                  href="https://goteleport.com/docs/database-access/guides/aws-discovery/"
                >
                  this guide
                </Link>{' '}
                to configure one before going to the next step.
              </Box>
            )}
          </Box>
          <DatabaseList
            wantAutoDiscover={wantAutoDiscover}
            items={tableData.items}
            fetchStatus={tableData.fetchStatus}
            selectedDatabase={selectedDb}
            onSelectDatabase={setSelectedDb}
            fetchNextPage={fetchNextPage}
          />
        </>
      )}
      {hasIamPermError && (
        <Box mb={5}>
          <ConfigureIamPerms
            kind="rds"
            region={tableData.currRegion}
            integrationRoleArn={
              (agentMeta as DbMeta).awsIntegration.spec.roleArn
            }
          />
        </Box>
      )}
      {showTable && wantAutoDiscover && (
        <Text mt={4} mb={-3}>
          <b>Note:</b> Auto-Enroll will enroll <Mark>all</Mark> database engines
          in this region (eg: postgres, mysql, aurora etc.)
        </Text>
      )}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={
          fetchDbAttempt.status === 'processing' ||
          (!wantAutoDiscover && !selectedDb) ||
          hasIamPermError
        }
      />
      {DialogComponent}
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
