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
import { Indicator, Box } from 'design';
import { Danger } from 'design/Alert';
import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import DesktopList from './DesktopList';
import useDesktops, { State } from './useDesktops';

const DOC_URL = 'https://goteleport.com/docs/desktop-access/getting-started/';

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
    desktops,
    canCreate,
    isLeafCluster,
    getWindowsLoginOptions,
    openRemoteDesktopTab,
  } = props;

  const isEmpty = attempt.status === 'success' && desktops.length === 0;
  const hasDesktops = attempt.status === 'success' && desktops.length > 0;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Desktops</FeatureHeaderTitle>
      </FeatureHeader>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      {hasDesktops && (
        <>
          <DesktopList
            desktops={desktops}
            username={username}
            clusterId={clusterId}
            onLoginMenuOpen={getWindowsLoginOptions}
            onLoginSelect={openRemoteDesktopTab}
          />
        </>
      )}
      {isEmpty && (
        <Empty
          clusterId={clusterId}
          canCreate={canCreate && !isLeafCluster}
          emptyStateInfo={emptyStateInfo}
        />
      )}
    </FeatureBox>
  );
}

const emptyStateInfo: EmptyStateInfo = {
  title: 'Add your first Windows desktop to Teleport',
  byline:
    'Teleport Desktop Access provides graphical desktop access to remote Windows hosts.',
  docsURL: DOC_URL,
  resourceType: 'desktop',
  readOnly: {
    title: 'No Desktops Found',
    resource: 'desktops',
  },
};
