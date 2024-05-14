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
import { Link as InternalRouteLink } from 'react-router-dom';

import { Box, Flex, Image } from 'design';
import { AWSIcon } from 'design/SVGIcon';
import slackIcon from 'design/assets/images/icons/slack.svg';
import openaiIcon from 'design/assets/images/icons/openai.svg';
import jamfIcon from 'design/assets/images/icons/jamf.svg';
import opsgenieIcon from 'design/assets/images/icons/opsgenie.svg';
import oktaIcon from 'design/assets/images/icons/okta.svg';
import jiraIcon from 'design/assets/images/icons/jira.svg';
import mattermostIcon from 'design/assets/images/icons/mattermost.svg';
import pagerdutyIcon from 'design/assets/images/icons/pagerduty.svg';
import servicenowIcon from 'design/assets/images/icons/servicenow.svg';
import discordIcon from 'design/assets/images/icons/discord.svg';
import emailIcon from 'design/assets/images/icons/email.svg';
import msteamIcon from 'design/assets/images/icons/msteams.svg';
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
  ExternalAuditStorageIntegration,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { ExternalAuditStorageOpType } from './Operations/useIntegrationOperation';
import { UpdateAwsOidcThumbprint } from './UpdateAwsOidcThumbprint';

type Props<IntegrationLike> = {
  list: IntegrationLike[];
  onDeletePlugin?(p: Plugin): void;
  integrationOps?: {
    onDeleteIntegration(i: Integration): void;
    onEditIntegration(i: Integration): void;
  };
  onDeleteExternalAuditStorage?(opType: ExternalAuditStorageOpType): void;
};

type IntegrationLike = Integration | Plugin | ExternalAuditStorageIntegration;

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

            if (
              item.resourceType === 'integration' &&
              // Currently, only AWSOIDC supports editing.
              item.kind === IntegrationKind.AwsOidc
            ) {
              return (
                <Cell align="right">
                  <MenuButton>
                    <MenuItem
                      onClick={() =>
                        props.integrationOps.onEditIntegration(item)
                      }
                    >
                      Edit...
                    </MenuItem>
                    <MenuItem
                      onClick={() =>
                        props.integrationOps.onDeleteIntegration(item)
                      }
                    >
                      Delete...
                    </MenuItem>
                  </MenuButton>
                </Cell>
              );
            }

            // draft external audit storage
            if (item.statusCode === IntegrationStatusCode.Draft) {
              return (
                <Cell align="right">
                  <MenuButton>
                    <MenuItem
                      as={InternalRouteLink}
                      to={{
                        pathname: cfg.getIntegrationEnrollRoute(
                          IntegrationKind.ExternalAuditStorage
                        ),
                        state: { continueDraft: true },
                      }}
                    >
                      Continue Setup...
                    </MenuItem>
                    <MenuItem
                      onClick={() =>
                        props.onDeleteExternalAuditStorage('draft')
                      }
                    >
                      Delete...
                    </MenuItem>
                  </MenuButton>
                </Cell>
              );
            }

            // active external audit storage
            return (
              <Cell align="right">
                <MenuButton>
                  <MenuItem
                    onClick={() =>
                      props.onDeleteExternalAuditStorage('cluster')
                    }
                  >
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

  if (
    item.resourceType === 'integration' &&
    item.kind === IntegrationKind.AwsOidc &&
    (!item.spec.issuerS3Bucket || !item.spec.issuerS3Prefix)
  ) {
    return (
      <Cell>
        <Flex alignItems="center">
          <StatusLight status={status} />
          {getStatusCodeTitle(item.statusCode)}
          <Box mx="1">
            <UpdateAwsOidcThumbprint integration={item} />
          </Box>
        </Flex>
      </Cell>
    );
  }
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
  if (item.resourceType === 'integration') {
    return Status.Success;
  }

  if (item.resourceType === 'external-audit-storage') {
    switch (item.statusCode) {
      case IntegrationStatusCode.Draft:
        return Status.Warning;
      default:
        return Status.Success;
    }
  }

  switch (item.statusCode) {
    case IntegrationStatusCode.Unknown:
      return null;
    case IntegrationStatusCode.Running:
      return Status.Success;
    case IntegrationStatusCode.SlackNotInChannel:
      return Status.Warning;
    case IntegrationStatusCode.Draft:
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
      return theme.colors.success.main;
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
      case 'slack':
        formattedText = 'Slack';
        icon = <IconContainer src={slackIcon} />;
        break;
      case 'openai':
        formattedText = 'OpenAI';
        icon = <IconContainer src={openaiIcon} />;
        break;
      case 'jamf':
        formattedText = 'Jamf';
        icon = <IconContainer src={jamfIcon} />;
        break;
      case 'okta':
        formattedText = 'Okta';
        icon = <IconContainer src={oktaIcon} />;
        break;
      case 'opsgenie':
        formattedText = 'Opsgenie';
        icon = <IconContainer src={opsgenieIcon} />;
        break;
      case 'jira':
        formattedText = 'Jira';
        icon = <IconContainer src={jiraIcon} />;
        break;
      case 'mattermost':
        formattedText = 'Mattermost';
        icon = <IconContainer src={mattermostIcon} />;
        break;
      case 'servicenow':
        formattedText = 'ServiceNow';
        icon = <IconContainer src={servicenowIcon} />;
        break;
      case 'pagerduty':
        formattedText = 'PagerDuty';
        icon = <IconContainer src={pagerdutyIcon} />;
        break;
      case 'discord':
        formattedText = 'Discord';
        icon = <IconContainer src={discordIcon} />;
        break;
      case 'email':
        formattedText = 'Email';
        icon = <IconContainer src={emailIcon} />;
        break;
      case 'msteams':
        formattedText = 'Microsoft Teams';
        icon = <IconContainer src={msteamIcon} />;
        break;
    }
  } else {
    // Default is integration.
    switch (item.kind) {
      case IntegrationKind.AwsOidc:
      case IntegrationKind.ExternalAuditStorage:
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
