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

import React, { useMemo, useState } from 'react';
import { useHistory } from 'react-router';
import { Link as InternalRouteLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import Table, { Cell } from 'design/DataTable';
import { SecondaryOutlined } from 'design/Label';
import { ResourceIcon } from 'design/ResourceIcon';
import { IconTooltip } from 'design/Tooltip';
import {
  applyFilters,
  FilterMap,
  ListFilters,
} from 'shared/components/ListFilters';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { useAsync } from 'shared/hooks/useAsync';
import { saveOnDisk } from 'shared/utils/saveOnDisk';

import cfg from 'teleport/config';
import {
  filterByIntegrationTags,
  getStatus,
  integrationLikeToIntegrationTags,
} from 'teleport/Integrations/helpers';
import api from 'teleport/services/api';
import {
  getStatusCodeDescription,
  getStatusCodeTitle,
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
  Plugin,
} from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { ExternalAuditStorageOpType } from './Operations/useIntegrationOperation';
import {
  compareByTags,
  getIntegrationTagLabel,
  IntegrationLike,
  IntegrationTag,
  integrationTagOptions,
  Status,
} from './types';

type Filters = {
  Type: IntegrationTag;
};

type Props = {
  list: IntegrationLike[];
  onDeletePlugin?(p: Plugin): void;
  integrationOps?: {
    onDeleteIntegration(i: Integration): void;
    onEditIntegration(i: Integration): void;
  };
  onDeleteExternalAuditStorage?(opType: ExternalAuditStorageOpType): void;
};

// statusKinds are the integration types with status pages; we enable clicking on the row directly to route to the view
const statusKinds = [
  'okta',
  'entra-id',
  IntegrationKind.AwsOidc,
  IntegrationKind.AwsRa,
];

export function IntegrationList(props: Props) {
  const history = useHistory();

  function handleRowClick(row: IntegrationLike) {
    // TODO (avatus) enable this feature by checking isManagedByTerraform.
    // Leaving commented until IaC form and settings page are implemented

    // if ('isManagedByTerraform' in row && row.isManagedByTerraform) {
    //   history.push(cfg.getIaCIntegrationRoute(row.kind, row.name));
    //   return;
    // }

    if (!statusKinds.includes(row.kind)) return;
    history.push(cfg.getIntegrationStatusRoute(row.kind, row.name));
  }

  function getRowStyle(row: IntegrationLike): React.CSSProperties {
    if (
      ('isManagedByTerraform' in row && row.isManagedByTerraform) ||
      statusKinds.includes(row.kind)
    ) {
      return { cursor: 'pointer' };
    }
  }

  const [downloadAttempt, download] = useAsync(
    async (clusterId: string, itemName: string) => {
      return api
        .fetch(cfg.getMsTeamsAppZipRoute(clusterId, itemName))
        .then(response => response.blob())
        .then(blob => {
          saveOnDisk(blob, 'app.zip', 'application/zip');
        });
    }
  );

  const { clusterId } = useStickyClusterId();

  const [filters, setFilters] = useState<FilterMap<IntegrationLike, Filters>>({
    Type: {
      options: integrationTagOptions,
      selected: [],
      apply: filterByIntegrationTags,
    },
  });

  const filteredList = useMemo(
    () => applyFilters(props.list, filters),
    [props.list, filters]
  );

  return (
    <Table
      pagination={{ pageSize: 20 }}
      isSearchable
      filters={<ListFilters filters={filters} onFilterChange={setFilters} />}
      data={filteredList}
      row={{
        onClick: handleRowClick,
        getStyle: getRowStyle,
      }}
      columns={[
        {
          key: 'resourceType',
          isNonRender: true,
        },
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
          render: item => <IconCell item={item} />,
        },
        {
          key: 'tags',
          headerText: 'Type',
          isSortable: true,
          onSort: compareByTags,
          render: item => (
            <Cell>
              <Flex gap={1}>
                {(item.tags ?? integrationLikeToIntegrationTags(item)).map(
                  tag => (
                    <SecondaryOutlined key={tag}>
                      {getIntegrationTagLabel(tag)}
                    </SecondaryOutlined>
                  )
                )}
              </Flex>
            </Cell>
          ),
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
            if (
              item.kind === IntegrationKind.AwsOidc ||
              item.kind === IntegrationKind.AwsRa ||
              item.kind === 'entra-id'
            ) {
              // action menu for these integrations are available on the status page dashboard.
              return;
            }

            if (item.resourceType === 'plugin') {
              return (
                <Cell align="right">
                  <MenuButton>
                    {/* Currently, only okta supports status pages */}
                    {item.kind === 'okta' && (
                      <MenuItem
                        as={InternalRouteLink}
                        to={cfg.getIntegrationStatusRoute(item.kind, item.name)}
                      >
                        View Status
                      </MenuItem>
                    )}
                    {item.kind === 'msteams' && (
                      <MenuItem
                        disabled={downloadAttempt.status === 'processing'}
                        onClick={() => download(clusterId, item.name)}
                      >
                        Download app.zip
                      </MenuItem>
                    )}
                    <MenuItem onClick={() => props.onDeletePlugin(item)}>
                      Delete...
                    </MenuItem>
                  </MenuButton>
                </Cell>
              );
            }

            // Normal 'integration' type.
            if (item.resourceType === 'integration') {
              return (
                <Cell align="right">
                  <MenuButton>
                    {item.kind === IntegrationKind.GitHub && (
                      <MenuItem
                        onClick={() =>
                          props.integrationOps.onEditIntegration(item)
                        }
                      >
                        Edit...
                      </MenuItem>
                    )}
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
        </Flex>
      </Cell>
    );
  }
  const statusDescription = getStatusCodeDescription(
    item.statusCode,
    item.status?.errorMessage
  );
  return (
    <Cell>
      <Flex alignItems="center">
        <StatusLight status={status} />
        {getStatusCodeTitle(item.statusCode)}
        {statusDescription && (
          <Box mx="1">
            <IconTooltip>{statusDescription}</IconTooltip>
          </Box>
        )}
      </Flex>
    </Cell>
  );
};

const StatusLight = styled(Box)<{ status: Status }>`
  border-radius: 50%;
  margin-right: 4px;
  width: 8px;
  height: 8px;
  background-color: ${({ status, theme }) => {
    if (status === Status.Success) {
      return theme.colors.success.main;
    }
    if ([Status.Error, Status.OktaConfigError].includes(status)) {
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
        icon = <IconContainer name="slack" />;
        break;
      case 'jamf':
        formattedText = 'Jamf';
        icon = <IconContainer name="jamf" />;
        break;
      case 'okta':
        formattedText = 'Okta';
        icon = <IconContainer name="okta" />;
        break;
      case 'opsgenie':
        formattedText = 'Opsgenie';
        icon = <IconContainer name="opsgenie" />;
        break;
      case 'jira':
        formattedText = 'Jira';
        icon = <IconContainer name="jira" />;
        break;
      case 'mattermost':
        formattedText = 'Mattermost';
        icon = <IconContainer name="mattermost" />;
        break;
      case 'servicenow':
        formattedText = 'ServiceNow';
        icon = <IconContainer name="servicenow" />;
        break;
      case 'pagerduty':
        formattedText = 'PagerDuty';
        icon = <IconContainer name="pagerduty" />;
        break;
      case 'discord':
        formattedText = 'Discord';
        icon = <IconContainer name="discord" />;
        break;
      case 'email':
        formattedText = 'Email';
        icon = <IconContainer name="email" />;
        break;
      case 'msteams':
        formattedText = 'Microsoft Teams';
        icon = <IconContainer name="microsoftteams" />;
        break;
      case 'entra-id':
        formattedText = 'Microsoft Entra ID';
        icon = <IconContainer name="entraid" />;
        break;
      case 'datadog':
        formattedText = 'Datadog Incident Management';
        icon = <IconContainer name="datadog" />;
        break;
      case 'aws-identity-center':
        formattedText = 'AWS IAM Identity Center';
        icon = <IconContainer name="aws" />;
        break;
      case 'scim':
        formattedText = 'SCIM';
        icon = <IconContainer name="scim" />;
        break;
      case 'intune':
        formattedText = 'Microsoft Intune';
        icon = <IconContainer name="intune" />;
        break;
      default:
        // TODO(ravicious): Remove openai exemption from here once a branch is added for it.
        item.kind satisfies never | 'openai';
    }
  } else {
    // Default is integration.
    switch (item.kind) {
      case IntegrationKind.AwsOidc:
      case IntegrationKind.ExternalAuditStorage:
        formattedText = item.name;
        icon = <IconContainer name="aws" />;
        break;
      case IntegrationKind.AwsRa:
        formattedText = item.name;
        icon = <IconContainer name="awsidentityandaccessmanagementiam" />;
        break;
      case IntegrationKind.AzureOidc:
        formattedText = 'Azure OIDC';
        icon = <IconContainer name="azure" />;
        break;
      case IntegrationKind.GitHub:
        formattedText = item.name;
        icon = <IconContainer name="github" />;
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

const IconContainer = styled(ResourceIcon)`
  width: 22px;
  margin-right: 10px;
`;
