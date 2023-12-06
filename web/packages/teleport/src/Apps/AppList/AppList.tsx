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
import styled from 'styled-components';
import { Flex, Text, ButtonBorder } from 'design';
import Table, { Cell, ClickableLabelCell } from 'design/DataTable';
import { FetchStatus, SortType } from 'design/DataTable/types';
import {
  pink,
  teal,
  cyan,
  blue,
  green,
  orange,
  brown,
  red,
  deepOrange,
  blueGrey,
} from 'design/theme/palette';
import { AmazonAws } from 'design/Icon';

import { App } from 'teleport/services/apps';
import { ResourceLabel, ResourceFilter } from 'teleport/services/agents';
import ServersideSearchPanel from 'teleport/components/ServersideSearchPanel';

import AwsLaunchButton from './AwsLaunchButton';

import type { PageIndicators } from 'teleport/components/hooks/useServersidePagination';

export default function AppList(props: Props) {
  const {
    apps = [],
    pageSize,
    fetchNext,
    fetchPrev,
    fetchStatus,
    params,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    onLabelClick,
    pageIndicators,
  } = props;

  return (
    <StyledTable
      data={apps}
      columns={[
        {
          altKey: 'app-icon',
          render: renderAppIcon,
        },
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'description',
          headerText: 'Description',
          isSortable: true,
        },
        {
          key: 'addrWithProtocol',
          headerText: 'Address',
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => (
            <ClickableLabelCell labels={labels} onClick={onLabelClick} />
          ),
        },
        {
          altKey: 'launch-btn',
          render: renderLaunchButtonCell,
        },
      ]}
      emptyText="No Applications Found"
      pagination={{
        pageSize,
      }}
      fetching={{
        onFetchNext: fetchNext,
        onFetchPrev: fetchPrev,
        fetchStatus,
      }}
      serversideProps={{
        sort: params.sort,
        setSort,
        serversideSearchPanel: (
          <ServersideSearchPanel
            pageIndicators={pageIndicators}
            params={params}
            setParams={setParams}
            pathname={pathname}
            replaceHistory={replaceHistory}
            disabled={fetchStatus === 'loading'}
          />
        ),
      }}
      isSearchable
    />
  );
}

function renderAppIcon({ name, awsConsole }: App) {
  return (
    <Cell style={{ userSelect: 'none' }}>
      <Flex
        height="32px"
        width="32px"
        bg={awsConsole ? orange[700] : getIconColor(name)}
        borderRadius="100%"
        justifyContent="center"
        alignItems="center"
      >
        {awsConsole ? (
          <AmazonAws size="large" />
        ) : (
          <Text fontSize={3} color="light" bold caps>
            {name[0]}
          </Text>
        )}
      </Flex>
    </Cell>
  );
}

function renderLaunchButtonCell({
  launchUrl,
  awsConsole,
  awsRoles,
  fqdn,
  clusterId,
  publicAddr,
  isCloudOrTcpEndpoint,
  samlApp,
  samlAppSsoUrl,
}: App) {
  let $btn;
  if (awsConsole) {
    $btn = (
      <AwsLaunchButton
        awsRoles={awsRoles}
        fqdn={fqdn}
        clusterId={clusterId}
        publicAddr={publicAddr}
      />
    );
  } else if (isCloudOrTcpEndpoint) {
    $btn = (
      <ButtonBorder
        disabled
        width="88px"
        size="small"
        title="Cloud or TCP applications cannot be launched by the browser"
      >
        LAUNCH
      </ButtonBorder>
    );
  } else if (samlApp) {
    $btn = (
      <ButtonBorder
        as="a"
        width="88px"
        size="small"
        target="_blank"
        href={samlAppSsoUrl}
        rel="noreferrer"
      >
        LOGIN
      </ButtonBorder>
    );
  } else {
    $btn = (
      <ButtonBorder
        as="a"
        width="88px"
        size="small"
        target="_blank"
        href={launchUrl}
        rel="noreferrer"
      >
        LAUNCH
      </ButtonBorder>
    );
  }

  return <Cell align="right">{$btn}</Cell>;
}

function getIconColor(appName: string) {
  let stringValue = 0;
  for (let i = 0; i < appName.length; i++) {
    stringValue += appName.charCodeAt(i);
  }

  const colors = [
    pink[700],
    teal[700],
    cyan[700],
    blue[700],
    green[700],
    orange[700],
    brown[700],
    red[700],
    deepOrange[700],
    blueGrey[700],
  ];

  return colors[stringValue % 10];
}

type Props = {
  apps: App[];
  pageSize: number;
  fetchNext: () => void;
  fetchPrev: () => void;
  fetchStatus: FetchStatus;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  setSort: (sort: SortType) => void;
  pathname: string;
  replaceHistory: (path: string) => void;
  onLabelClick: (label: ResourceLabel) => void;
  pageIndicators: PageIndicators;
};

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;
