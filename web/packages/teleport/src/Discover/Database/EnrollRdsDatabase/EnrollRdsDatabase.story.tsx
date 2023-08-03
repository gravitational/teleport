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

import React from 'react';

import { AwsRegionSelector } from './AwsRegionSelector';
import { DatabaseList } from './RdsDatabaseList';
import { CheckedAwsRdsDatabase } from './EnrollRdsDatabase';

export default {
  title: 'Teleport/Discover/Database/EnrollRds',
};

export const AwsRegionsSelectorDisabled = () => (
  <AwsRegionSelector
    onFetch={() => null}
    onRefresh={() => null}
    disableSelector={true}
    clear={() => null}
  />
);

export const AwsRegionsSelectorEnabled = () => (
  <AwsRegionSelector
    onFetch={() => null}
    onRefresh={() => null}
    disableSelector={false}
    clear={() => null}
  />
);

export const AwsRegionsSelectorRefreshEnabled = () => (
  <AwsRegionSelector
    onFetch={() => null}
    onRefresh={() => null}
    disableSelector={false}
    clear={() => null}
  />
);

export const RdsDatabaseList = () => (
  <DatabaseList
    items={fixtures}
    fetchNextPage={() => null}
    onSelectDatabase={() => null}
    selectedDatabase={null}
    fetchStatus="disabled"
  />
);

export const RdsDatabaseListWithSelection = () => (
  <DatabaseList
    items={fixtures}
    fetchNextPage={() => null}
    onSelectDatabase={() => null}
    selectedDatabase={fixtures[2]}
    fetchStatus=""
  />
);

export const RdsDatabaseListLoading = () => (
  <DatabaseList
    items={fixtures}
    fetchNextPage={() => null}
    onSelectDatabase={() => null}
    selectedDatabase={fixtures[2]}
    fetchStatus="loading"
  />
);

const fixtures: CheckedAwsRdsDatabase[] = [
  {
    name: 'postgres-name',
    engine: 'postgres',
    uri: '',
    labels: [],
    status: 'available',
    accountId: '',
    resourceId: '',
    region: 'us-west-2',
    subnets: ['subnet1', 'subnet2'],
  },
  {
    name: 'mysql-name',
    engine: 'mysql',
    uri: '',
    labels: [],
    status: 'available',
    accountId: '',
    resourceId: '',
    dbServerExists: true,
    region: 'us-west-2',
    subnets: ['subnet1', 'subnet2'],
  },
  {
    name: 'alpaca',
    engine: 'aurora',
    uri: '',
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'os', value: 'windows' },
    ],
    status: 'deleting',
    accountId: '',
    resourceId: '',
    region: 'us-west-2',
    subnets: ['subnet1', 'subnet2'],
  },
  {
    name: 'banana',
    engine: 'postgres',
    uri: '',
    labels: [],
    status: 'failed',
    accountId: '',
    resourceId: '',
    region: 'us-west-2',
    subnets: ['subnet1', 'subnet2'],
  },
  {
    name: 'watermelon',
    engine: 'mysql',
    uri: '',
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'os', value: 'mac' },
      { name: 'fruit', value: 'watermelon' },
    ],
    status: 'Unknown' as any,
    accountId: '',
    resourceId: '',
    dbServerExists: true,
    region: 'us-west-2',
    subnets: ['subnet1', 'subnet2'],
  },
  {
    name: 'llama',
    engine: 'postgres',
    uri: '',
    labels: [{ name: 'testing-name', value: 'testing-value' }],
    status: 'available',
    accountId: '',
    resourceId: '',
    region: 'us-west-2',
    subnets: ['subnet1', 'subnet2'],
  },
];
