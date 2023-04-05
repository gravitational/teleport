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

type Props<Integrations> = {
  list: Integrations[];
  onDelete(i: Integrations): void;
};

type Integrations = Integration | Plugin;

export function IntegrationList(props: Props<Integrations>) {
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

const StatusCell = ({ item }: { item: Integrations }) => {
  let success;
  let warning;
  let error;

  if (item.resourceType === 'plugin') {
    if (item.statusCode === 'Running') {
      success = true;
    } else if (
      item.statusCode === 'Unauthorized' ||
      item.statusCode === 'Unknown error'
    ) {
      error = true;
    } else if (item.statusCode === 'Bot not invited to channel') {
      warning = true;
    }
  } else {
    // default to integration resource type.
    if (item.statusCode === IntegrationCode.Running) {
      success = true;
    } else if (item.statusCode === IntegrationCode.Paused) {
      warning = true;
    } else if (item.statusCode === IntegrationCode.Error) {
      error = true;
    }
  }

  // If no status code were matched, defaults to unknown.

  return (
    <Cell>
      <Flex alignItems="center">
        <StatusLight success={success} warning={warning} error={error} />
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

const StatusLight = styled(Box)`
  border-radius: 50%;
  margin-right: 4px;
  width: 8px;
  height: 8px;
  background-color: ${({ success, error, warning, theme }) => {
    if (success) {
      return theme.colors.success;
    }
    if (error) {
      return theme.colors.error.light;
    }
    if (warning) {
      return theme.colors.warning;
    }
    return theme.colors.grey[300]; // Unknown
  }};
`;

const IconCell = ({ item }: { item: Integrations }) => {
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
