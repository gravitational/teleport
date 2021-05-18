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

import React, { useState } from 'react';
import { Indicator, Box } from 'design';
import { Danger } from 'design/Alert';
import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import DatabaseList from './DatabaseList';
import useDatabases, { State } from './useDatabases';
import ButtonAdd from './ButtonAdd';
import AddDialog from './AddDatabase';

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
    isEnterprise,
    canCreate,
    showAddDialog,
    hideAddDialog,
    isAddDialogVisible,
    user,
    version,
    clusterId,
    authType,
  } = props;

  const [searchValue, setSearchValue] = useState<string>('');

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Databases</FeatureHeaderTitle>
        <ButtonAdd
          isLeafCluster={isLeafCluster}
          isEnterprise={isEnterprise}
          canCreate={canCreate}
          onClick={showAddDialog}
        />
      </FeatureHeader>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      {attempt.status === 'success' && (
        <DatabaseList
          databases={databases}
          user={user}
          clusterId={clusterId}
          authType={authType}
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
      )}
      {isAddDialogVisible && (
        <AddDialog
          user={user}
          version={version}
          authType={authType}
          onClose={hideAddDialog}
        />
      )}
    </FeatureBox>
  );
}
