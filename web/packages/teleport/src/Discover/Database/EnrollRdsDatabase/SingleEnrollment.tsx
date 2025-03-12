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

import { useEffect, useState } from 'react';

import { Flex, Subtitle1, Text } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import Validation, { Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import { getRdsEngineIdentifier } from 'teleport/Discover/SelectResource/types';
import { ResourceLabelTooltip } from 'teleport/Discover/Shared/ResourceLabelTooltip';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { ResourceLabel } from 'teleport/services/agents';
import { Database } from 'teleport/services/databases';
import {
  AwsRdsDatabase,
  integrationService,
  Regions,
  Vpc,
} from 'teleport/services/integrations';

import { ActionButtons, LabelsCreater } from '../../Shared';
import { CreateDatabaseDialog } from '../CreateDatabase/CreateDatabaseDialog';
import { useCreateDatabase } from '../CreateDatabase/useCreateDatabase';
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
    clearAttempt,
    nextStep,
    fetchDatabaseServers,
    handleOnTimeout,
  } = useCreateDatabase();

  const { agentMeta, resourceSpec, emitErrorEvent } = useDiscover();

  const [tableData, setTableData] = useState<TableData>();
  const [selectedDb, setSelectedDb] = useState<CheckedAwsRdsDatabase>();
  const [customLabels, setCustomLabels] = useState<ResourceLabel[]>([]);

  useEffect(() => {
    if (vpc) {
      // Start with empty table data for new vpc's.
      fetchRdsDatabases(emptyTableData(), vpc);
    }
  }, [vpc]);

  function onSelectRds(rds: CheckedAwsRdsDatabase) {
    // when changing selected db, clear defined labels
    setCustomLabels([]);
    setSelectedDb(rds);
  }

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

      // Abort early if there were no rds dbs for the selected region/vpc.
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

  function handleOnProceedWithValidation(
    validator: Validator,
    { overwriteDb = false } = {}
  ) {
    if (!validator.validate()) {
      return;
    }

    handleOnProceed({ overwriteDb });
  }

  function handleOnProceed({ overwriteDb = false } = {}) {
    // Corner case where if registering db fails a user can:
    //   1) change region, which will list new databases or
    //   2) select a different database before re-trying.
    const isNewDb = selectedDb.name !== createdDb?.name;
    registerDatabase(
      {
        name: selectedDb.name,
        protocol: selectedDb.engine,
        uri: selectedDb.uri,
        // The labels from the `selectedDb` are AWS tags which
        // will be imported as is.
        labels: [...selectedDb.labels, ...customLabels],
        awsRds: selectedDb,
        awsRegion: region,
        awsVpcId: vpc.id,
      },
      { newDb: isNewDb, overwriteDb }
    );
  }

  const showTable = !!vpc && fetchAttempt.status !== 'failed';

  return (
    <>
      <Validation>
        {({ validator }) => (
          <>
            {showTable && (
              <>
                <Text mt={3}>Select an RDS database to enroll:</Text>
                <DatabaseList
                  wantAutoDiscover={false}
                  items={tableData?.items || []}
                  fetchStatus={tableData?.fetchStatus || 'loading'}
                  selectedDatabase={selectedDb}
                  onSelectDatabase={onSelectRds}
                  fetchNextPage={fetchNextPage}
                />
                {selectedDb && (
                  <>
                    <Flex alignItems="center" gap={1} mb={2} mt={4}>
                      <Subtitle1>Optionally Add More Labels</Subtitle1>
                      <ResourceLabelTooltip
                        toolTipPosition="top"
                        resourceKind="rds"
                      />
                    </Flex>
                    <LabelsCreater
                      labels={customLabels}
                      setLabels={setCustomLabels}
                      isLabelOptional={true}
                      disableBtns={disableBtns}
                      noDuplicateKey={true}
                    />
                  </>
                )}
              </>
            )}
            <ActionButtons
              onProceed={() => handleOnProceedWithValidation(validator)}
              disableProceed={disableBtns || !showTable || !selectedDb}
            />
          </>
        )}
      </Validation>
      {attempt.status !== '' && (
        <CreateDatabaseDialog
          pollTimeout={pollTimeout}
          attempt={attempt}
          next={nextStep}
          close={clearAttempt}
          retry={handleOnProceed}
          onTimeout={handleOnTimeout}
          onOverwrite={() => handleOnProceed({ overwriteDb: true })}
          dbName={selectedDb.name}
        />
      )}
    </>
  );
}
