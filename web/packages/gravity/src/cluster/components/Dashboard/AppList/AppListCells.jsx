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
import { Cell } from 'design/DataTable';
import { Text, Flex, Box } from 'design';
import * as States from 'design/LabelState';
import defaultLogo from 'design/assets/images/app-logo.svg';
import { StatusEnum } from 'gravity/services/releases';

export function AppNameCell({ rowIndex, data }) {
  const { icon, chartName } = data[rowIndex];
  const logoSrc = icon || defaultLogo;
  return (
    <Cell style={cellStyle}>
      <Flex>
        <Box as="img" src={logoSrc} width="25px" height="25px" mr="2" />
        {chartName}
      </Flex>
    </Cell>
  );
}

function StateDisabled(props) {
  return <States.StateSuccess bg="grey.500" {...props} />;
}

function getStatusComponent(status) {
  switch (status) {
    case StatusEnum.DEPLOYED:
      return States.StateSuccess;
    case StatusEnum.DELETED:
      return StateDisabled;
    case StatusEnum.SUPERSEDED:
      return States.StateSuccess;
    case StatusEnum.FAILED:
      return States.StateDanger;
    case StatusEnum.DELETING:
    case StatusEnum.PENDING_INSTALL:
    case StatusEnum.PENDING_UPGRADE:
    case StatusEnum.PENDING_ROLLBACK:
      return States.StateWarning;
  }

  return States.StateWarning;
}

export function StatusCell({ rowIndex, data }) {
  const { status } = data[rowIndex];
  const Component = getStatusComponent(status);
  const statusText = status.replace('_', ' ');
  return (
    <Cell style={cellStyle}>
      <Component>{statusText}</Component>
    </Cell>
  );
}

export function EndpointCell({ rowIndex, data }) {
  const { endpoints } = data[rowIndex];
  const $endpoints = endpoints.map(renderEndpoint);
  return <Cell style={{ width: '100%' }}>{$endpoints}</Cell>;
}

function renderEndpoint({ addresses = [] }) {
  const $addresses = addresses.map(addr => (
    <StyledLink
      as="a"
      color="text.primary"
      href={addr}
      target="_blank"
      key={addr}
    >
      {addr}
    </StyledLink>
  ));

  return $addresses;
}

const cellStyle = {
  fontSize: '16px',
};

const StyledLink = styled(Text)`
  display: block;
  font-weight: normal;
  background: none;
  text-decoration: none;
  text-transform: none;
  line-height: 16px;
  font-size: 10px;
  word-break: break-all;
`;
