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

import React from 'react';
import { Box, Indicator } from 'design';

import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import cfg from 'teleport/config';
import history from 'teleport/services/history/history';
import { storageService } from 'teleport/services/storageService';

import AgentButtonAdd from 'teleport/components/AgentButtonAdd';

import { SearchResource } from 'teleport/Discover/SelectResource';

import DesktopList from './DesktopList';
import { State, useDesktops } from './useDesktops';

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
    canCreate,
    isLeafCluster,
    getWindowsLoginOptions,
    openRemoteDesktopTab,
    fetchedData,
    fetchNext,
    fetchPrev,
    pageSize,
    params,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    fetchStatus,
    isSearchEmpty,
    onLabelClick,
    pageIndicators,
  } = props;

  const hasNoDesktops =
    attempt.status === 'success' &&
    fetchedData.agents.length === 0 &&
    isSearchEmpty;

  const enabled = storageService.areUnifiedResourcesEnabled();
  if (enabled) {
    history.replace(cfg.getUnifiedResourcesRoute(clusterId));
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Desktops</FeatureHeaderTitle>
        {attempt.status === 'success' && !hasNoDesktops && (
          <AgentButtonAdd
            agent={SearchResource.DESKTOP}
            beginsWithVowel={false}
            isLeafCluster={isLeafCluster}
            canCreate={canCreate}
          />
        )}
      </FeatureHeader>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && (
        <ErrorMessage message={attempt.statusText} />
      )}
      {attempt.status !== 'processing' && !hasNoDesktops && (
        <DesktopList
          desktops={fetchedData.agents}
          username={username}
          clusterId={clusterId}
          onLoginMenuOpen={getWindowsLoginOptions}
          onLoginSelect={openRemoteDesktopTab}
          fetchNext={fetchNext}
          fetchPrev={fetchPrev}
          fetchStatus={fetchStatus}
          pageIndicators={pageIndicators}
          pageSize={pageSize}
          params={params}
          setParams={setParams}
          setSort={setSort}
          pathname={pathname}
          replaceHistory={replaceHistory}
          onLabelClick={onLabelClick}
        />
      )}
      {attempt.status === 'success' && hasNoDesktops && (
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
  resourceType: SearchResource.DESKTOP,
  readOnly: {
    title: 'No Desktops Found',
    resource: 'desktops',
  },
};
