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
  AwsDatabase,
  ListAwsDatabaseResponse,
  Regions,
  integrationService,
} from 'teleport/services/integrations';

import { ActionButtons, Header } from '../../Shared';

import { AwsRegionSelector } from './AwsRegionSelector';
import { DatabaseList } from './DatabaseList';

type TableData = {
  items: ListAwsDatabaseResponse['databases'];
  fetchStatus: FetchStatus;
  startKey?: string;
  currRegion?: Regions;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  startKey: '',
};

// TODO(lisa): need to add a new event for this, or can
// we re-use "create database event"?
export function EnrollRdsDatabase() {
  const { nextStep, agentMeta } = useDiscover();
  const { attempt, setAttempt } = useAttempt('');

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    startKey: '',
    fetchStatus: 'disabled',
  });
  const [selectedDb, setSelectedDb] = useState<AwsDatabase>();

  function fetchDatabasesWithNewRegion(region: Regions) {
    // Clear table when fetching with new region.
    fetchDatabases({ ...emptyTableData, currRegion: region });
  }

  function fetchNextPage() {
    fetchDatabases({ ...tableData });
  }

  function fetchDatabases(data: TableData) {
    const integrationName = (agentMeta as DbMeta).awsIntegrationName;

    setTableData({ ...data, fetchStatus: 'loading' });
    setAttempt({ status: 'processing' });

    // TODO(lisa): re-visit after backend implementation is final
    integrationService
      .fetchAwsDatabases(integrationName, {
        region: data.currRegion,
        nextToken: data.startKey,
      })
      .then(resp => {
        setAttempt({ status: 'success' });
        setTableData({
          currRegion: data.currRegion,
          startKey: resp.nextToken,
          fetchStatus: resp.nextToken ? '' : 'disabled',
          // concat each page fetch.
          items: [...data.items, ...resp.databases],
        });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        setTableData(data); // fallback to previous data
      });
  }

  function handleOnProceed() {
    // TODO(lisa):
    // Update agent meta with the selected RDS database.
    nextStep();
  }

  function clear() {
    if (attempt.status === 'failed') {
      setAttempt({ status: '' });
    }
    if (tableData.items.length > 0) {
      setTableData(emptyTableData);
    }
    if (selectedDb) {
      setSelectedDb(null);
    }
  }

  return (
    <Box maxWidth="800px">
      <Header>Enroll a RDS Database</Header>
      {attempt.status === 'failed' && (
        <Danger mt={3}>{attempt.statusText}</Danger>
      )}
      <AwsRegionSelector
        onFetch={fetchDatabasesWithNewRegion}
        clear={clear}
        disableSelector={attempt.status === 'processing'}
        disableBtn={
          attempt.status === 'processing' || tableData.items.length > 0
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
        disableProceed={attempt.status === 'processing' || !selectedDb}
      />
    </Box>
  );
}
