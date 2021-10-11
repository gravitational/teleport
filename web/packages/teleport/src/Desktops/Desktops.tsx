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
import { Indicator, Box, Flex } from 'design';
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

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktops(ctx);
  return <Desktops {...state} />;
}

export function Desktops(props: State) {
  const {
    attempt,
    username,
    clusterId,
    searchValue,
    setSearchValue,
    desktops,
    openRemoteDesktopWindow,
  } = props;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Desktops (preview)</FeatureHeaderTitle>
      </FeatureHeader>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      {attempt.status === 'success' && (
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
            desktops={desktops}
            username={username}
            clusterId={clusterId}
            searchValue={searchValue}
            openRemoteDesktopWindow={openRemoteDesktopWindow}
          />
        </>
      )}
    </FeatureBox>
  );
}
