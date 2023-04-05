/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { Box, Flex, Image } from 'design';
import awsIcon from 'design/assets/images/icons/aws.svg';
import slackIcon from 'design/assets/images/icons/slack.svg';
import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { IntegrationCode } from 'teleport/services/integrations';

import type { Integration, Plugin } from 'teleport/services/integrations';

type Props<IntegrationLike> = {
  list: IntegrationLike[];
  onDelete(i: IntegrationLike): void;
};

type IntegrationLike = Integration | Plugin;

export function IntegrationList(props: Props<IntegrationLike>) {
  return (
    <Table
      pagination={{ pageSize: 20 }}
      isSearchable
      data={props.list}
      columns={[
        {
          key: 'kind',
          headerText: 'Integration',
          isSortable: true,
          render: item => <IconCell item={item} />,
        },
        {
          key: 'details',
          headerText: 'Details',
        },
        {
          key: 'statusCodeText',
          headerText: 'Status',
          isSortable: true,
          render: item => <StatusCell item={item} />,
        },
        {
          altKey: 'options-btn',
          render: item => (
            <ActionCell
              onDelete={props.onDelete ? () => props.onDelete(item) : null}
            />
          ),
        },
      ]}
      emptyText="No Results Found"
    />
  );
}

const StatusCell = ({ item }: { item: IntegrationLike }) => {
  const status = getStatus(item);

  return (
    <Cell>
      <Flex alignItems="center">
        <StatusLight status={status} />
        {item.statusCodeText}
      </Flex>
    </Cell>
  );
};

const ActionCell = ({ onDelete }: { onDelete: () => void }) => {
  if (!onDelete) {
    return null;
  }

  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={onDelete}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};

enum Status {
  Success,
  Warning,
  Error,
}

function getStatus(item: IntegrationLike) {
  if (item.resourceType === 'plugin') {
    switch (item.statusCode) {
      case 'Running':
        return Status.Success;

      case 'Unauthorized':
      case 'Unknown error':
        return Status.Error;

      case 'Bot not invited to channel':
        return Status.Warning;
    }
    return;
  }

  switch (item.statusCode) {
    case IntegrationCode.Running:
      return Status.Success;

    case IntegrationCode.Paused:
      return Status.Warning;

    case IntegrationCode.Error:
      return Status.Error;
  }
}

const StatusLight = styled(Box)`
  border-radius: 50%;
  margin-right: 4px;
  width: 8px;
  height: 8px;
  background-color: ${({ status, theme }) => {
    if (status === Status.Success) {
      return theme.colors.success;
    }
    if (status === Status.Error) {
      return theme.colors.error.light;
    }
    if (status === Status.Warning) {
      return theme.colors.warning;
    }
    return theme.colors.grey[300]; // Unknown
  }};
`;

const IconCell = ({ item }: { item: IntegrationLike }) => {
  let formattedText;
  let icon;
  if (item.resourceType === 'plugin') {
    switch (item.kind) {
      case 'slack':
        formattedText = 'Slack';
        icon = <IconContainer src={slackIcon} width="18px" />;
        break;
    }
  } else {
    // Default is integration.
    switch (item.kind) {
      case 'aws':
        // The aws icon already has the word "aws" on it,
        // so we set the text to empty.
        formattedText = '';
        icon = <IconContainer src={awsIcon} width="24px" />;
        break;
    }
  }

  return (
    <Cell>
      <Flex alignItems="center">
        {icon}
        {icon && formattedText}
      </Flex>
    </Cell>
  );
};

const IconContainer = styled(Image)`
  padding-right: 8px;
`;
