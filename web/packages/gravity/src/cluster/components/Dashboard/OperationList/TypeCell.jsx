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
import Icon, * as Icons from 'design/Icon/Icon';
import { OpTypeEnum } from 'gravity/services/enums';
import { StatusEnum } from 'gravity/services/operations';

const OperationIconMap = {
  [OpTypeEnum.OPERATION_EXPAND]: Icons.SettingsOverscan,
  [OpTypeEnum.OPERATION_INSTALL]: Icons.Unarchive,
  [OpTypeEnum.OPERATION_SHRINK]: Icons.Shrink,
  [OpTypeEnum.OPERATION_UNINSTALL]: Icons.PhonelinkErase,
};

function getColor(opStatus) {
  // first pick the color based on event code
  switch (opStatus) {
    case StatusEnum.COMPLETED:
      return 'success';
    case StatusEnum.FAILED:
      return 'danger';
    case StatusEnum.PROCESSING:
      return 'warning';
  }

  return 'info';
}

function getTypeText(opType) {
  switch (opType) {
    case OpTypeEnum.OPERATION_INSTALL:
      return 'Installing this cluster';
    case OpTypeEnum.OPERATION_SHRINK:
      return 'Removing a server';
    case OpTypeEnum.OPERATION_EXPAND:
      return 'Adding a server';
    case OpTypeEnum.OPERATION_UPDATE:
      return 'Updating this cluster';
    case OpTypeEnum.OPERATION_UNINSTALL:
      return 'Uninstalling this cluster';
  }

  return 'Unknown Operation';
}

export default function TypeCell({ rowIndex, data }) {
  const { isSession, operation } = data[rowIndex];

  let description;
  let bgColor;
  let IconType;

  if (!isSession) {
    description = getTypeText(operation.type);
    IconType = OperationIconMap[operation.type] || Icons.Cog;
    bgColor = getColor(operation.status);
  } else {
    bgColor = 'bgTerminal';
    IconType = Icons.Cli;
    description = 'Session in progress...';
  }

  return (
    <Cell style={{ fontSize: '14px' }}>
      <StyledEventType>
        <StyledIcon p="1" mr="3" bg={bgColor} as={IconType} fontSize="4" />
        {description}
      </StyledEventType>
    </Cell>
  );
}

const StyledIcon = styled(Icon)`
  border-radius: 50%;
`;

const StyledEventType = styled.div`
  display: flex;
  align-items: center;
  min-width: 130px;
  font-size: 12px;
  font-weight: 500;
  line-height: 24px;
  white-space: nowrap;
`;
