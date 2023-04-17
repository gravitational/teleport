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
import { DatabaseList } from './DatabaseList';

export default {
  title: 'Teleport/Discover/Database/EnrollRds',
};

export const AwsRegionsSelectorDisabled = () => (
  <AwsRegionSelector
    onFetch={() => null}
    disableBtn={true}
    disableSelector={true}
    clear={() => null}
  />
);

export const AwsRegionsSelectorEnabled = () => (
  <AwsRegionSelector
    onFetch={() => null}
    disableBtn={false}
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

const fixtures = [
  { name: 'postgres-name', engine: 'postgres', endpoint: '' },
  { name: 'mysql-name', engine: 'mysql', endpoint: '' },
  { name: 'alpaca', engine: 'postgres', endpoint: '' },
  { name: 'banana', engine: 'postgres', endpoint: '' },
  { name: 'watermelon', engine: 'mysql', endpoint: '' },
  { name: 'llama', engine: 'postgres', endpoint: '' },
];
