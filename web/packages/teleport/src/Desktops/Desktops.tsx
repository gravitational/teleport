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
import { Indicator, Box, Flex, Text, Link } from 'design';
import { Danger } from 'design/Alert';
import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import InputSearch from 'teleport/components/InputSearch';
import DesktopList from './DesktopList';
import useDesktops, { State } from './useDesktops';
import ButtonAdd from './ButtonAdd';
import AddDialog from './AddDatabase';

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktops(ctx);
  return <Desktops {...state} />;
}

export function Desktops(props: State) {
  const {
    databases,
    attempt,
    isLeafCluster,
    canCreate,
    showAddDialog,
    hideAddDialog,
    isAddDialogVisible,
    username,
    version,
    clusterId,
    authType,
    searchValue,
    setSearchValue,
  } = props;

  const isEmpty = attempt.status === 'success' && databases.length === 0;
  const hasDatabases = attempt.status === 'success' && databases.length > 0;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Desktops</FeatureHeaderTitle>
        <ButtonAdd
          isLeafCluster={isLeafCluster}
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
      {hasDatabases && (
        <>
          <Flex
            mb={4}
            alignItems="center"
            flex="0 0 auto"
            justifyContent="space-between"
          >
            <InputSearch mr={3} value={searchValue} onChange={setSearchValue} />
          </Flex>
          <DesktopList
            databases={databases}
            username={username}
            clusterId={clusterId}
            authType={authType}
            searchValue={searchValue}
          />
        </>
      )}
      {isAddDialogVisible && (
        <AddDialog
          username={username}
          version={version}
          authType={authType}
          onClose={hideAddDialog}
        />
      )}
    </FeatureBox>
  );
}

