/*
Copyright 2019 Gravitational, Inc.

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
import { Label, Flex, Text, ButtonBorder } from 'design';
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
import Table, { Cell } from 'design/DataTable';
import * as types from 'teleterm/ui/services/clusters/types';
import { useApps, State } from './useApps';
import { renderLabelCell } from '../renderLabelCell';

export default function Container() {
  const state = useApps();
  return <AppList {...state} />;
}

export function AppList(props: State) {
  const { apps = [] } = props;

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
          key: 'publicAddr',
          headerText: 'Address',
          render: renderAddress,
          isSortable: true,
        },
        {
          key: 'labelsList',
          headerText: 'Labels',
          render: renderLabelCell,
        },
        {
          altKey: 'launch-btn',
          render: renderConnectButton,
        },
      ]}
      emptyText="No Applications Found"
      pagination={{ pageSize: 100, pagerPosition: 'bottom' }}
    />
  );
}

const renderConnectButton = () => {
  return (
    <Cell align="right">
      <ButtonBorder size="small">LAUNCH</ButtonBorder>
    </Cell>
  );
};

const renderAppIcon = (app: types.Application) => {
  return (
    <Cell style={{ userSelect: 'none' }}>
      <Flex
        height="32px"
        width="32px"
        bg={getIconColor(app.name)}
        borderRadius="100%"
        justifyContent="center"
        alignItems="center"
      >
        <Text fontSize={3} bold caps>
          {app.name[0]}
        </Text>
      </Flex>
    </Cell>
  );
};

const renderAddress = (app: types.Application) => {
  return <Cell>https://{app.publicAddr}</Cell>;
};

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

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: baseline;
  }
`;
