/*
Copyright 2021-2022 Gravitational, Inc.

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
import { AgentLabel, AgentFilter } from 'teleport/services/agents';
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
          <AmazonAws fontSize={6} />
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
  params: AgentFilter;
  setParams: (params: AgentFilter) => void;
  setSort: (sort: SortType) => void;
  pathname: string;
  replaceHistory: (path: string) => void;
  onLabelClick: (label: AgentLabel) => void;
  pageIndicators: PageIndicators;
};

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;
