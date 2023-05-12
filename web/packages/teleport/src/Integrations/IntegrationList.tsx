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
import { AWSIcon } from 'design/SVGIcon';
import slackIcon from 'design/assets/images/icons/slack.svg';
import discordIcon from 'design/assets/images/icons/discord.svg';
import mattermostIcon from 'design/assets/images/icons/mattermost.svg';
import openaiIcon from 'design/assets/images/icons/openai.svg';
import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { ToolTipInfo } from 'shared/components/ToolTip';

import {
  getStatusCodeDescription,
  getStatusCodeTitle,
  Integration,
  IntegrationStatusCode,
  IntegrationKind,
  Plugin,
} from 'teleport/services/integrations';

type Props<IntegrationLike> = {
  list: IntegrationLike[];
  onDeletePlugin?(p: Plugin): void;
  onDeleteIntegration?(i: Integration): void;
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
          key: 'resourceType',
          isNonRender: true,
        },
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
          key: 'statusCode',
          headerText: 'Status',
          isSortable: true,
          render: item => <StatusCell item={item} />,
        },
        {
          altKey: 'options-btn',
          render: item => {
            if (item.resourceType === 'plugin') {
              return (
                <Cell align="right">
                  <MenuButton>
                    <MenuItem onClick={() => props.onDeletePlugin(item)}>
                      Delete...
                    </MenuItem>
                  </MenuButton>
                </Cell>
              );
            }

            return (
              <Cell align="right">
                <MenuButton>
                  <MenuItem onClick={() => props.onDeleteIntegration(item)}>
                    Delete...
                  </MenuItem>
                </MenuButton>
              </Cell>
            );
          },
        },
      ]}
      emptyText="No Results Found"
    />
  );
}

const StatusCell = ({ item }: { item: IntegrationLike }) => {
  const status = getStatus(item);
  const statusDescription = getStatusCodeDescription(item.statusCode);

  return (
    <Cell>
      <Flex alignItems="center">
        <StatusLight status={status} />
        {getStatusCodeTitle(item.statusCode)}
        {statusDescription && (
          <Box mx="1">
            <ToolTipInfo>{statusDescription}</ToolTipInfo>
          </Box>
        )}
      </Flex>
    </Cell>
  );
};

enum Status {
  Success,
  Warning,
  Error,
}

function getStatus(item: IntegrationLike): Status | null {
  if (item.resourceType !== 'plugin') {
    return Status.Success;
  }

  switch (item.statusCode) {
    case IntegrationStatusCode.Unknown:
      return null;
    case IntegrationStatusCode.Running:
      return Status.Success;
    case IntegrationStatusCode.SlackNotInChannel:
      return Status.Warning;
    default:
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
      return theme.colors.error.main;
    }
    if (status === Status.Warning) {
      return theme.colors.warning.main;
    }
    return theme.colors.grey[300]; // Unknown
  }};
`;

const IconCell = ({ item }: { item: IntegrationLike }) => {
  let formattedText;
  let icon;
  if (item.resourceType === 'plugin') {
    switch (item.kind) {
      case 'discord':
        formattedText = 'Discord';
        icon = <IconContainer src={discordIcon} />;
        break;
      case 'mattermost':
        formattedText = 'Mattermost';
        icon = <IconContainer src={mattermostIcon} />;
        break;
      case 'slack':
        formattedText = 'Slack';
        icon = <IconContainer src={slackIcon} />;
        break;
      case 'openai':
        formattedText = 'OpenAI';
        icon = <IconContainer src={openaiIcon} />;
        break;
    }
  } else {
    // Default is integration.
    switch (item.kind) {
      case IntegrationKind.AwsOidc:
        formattedText = item.name;
        icon = (
          <SvgIconContainer>
            <AWSIcon />
          </SvgIconContainer>
        );
        break;
    }
  }

  if (!formattedText) {
    formattedText = item.name;
  }

  return (
    <Cell>
      <Flex alignItems="center">
        {icon}
        {formattedText}
      </Flex>
    </Cell>
  );
};

const IconContainer = styled(Image)`
  width: 22px;
  margin-right: 10px;
`;

const SvgIconContainer = styled(Flex)`
  margin-right: 10px;
`;
