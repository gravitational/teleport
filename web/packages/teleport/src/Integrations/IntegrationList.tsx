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

import { Box, Flex, Text } from 'design';
import Table, { Cell, StyledPanel } from 'design/DataTable';
import InputSearch from 'design/DataTable/InputSearch';
import { CircleCheck, CircleCross, Warning } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';
import {
  applyFilters,
  FilterMap,
  ListFilters,
} from 'shared/components/ListFilters';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { useAsync } from 'shared/hooks/useAsync';
import { saveOnDisk } from 'shared/utils/saveOnDisk';
import { pluralize } from 'shared/utils/text';

import cfg from 'teleport/config';
import {
  filterByIntegrationStatus,
  filterBySearch,
  sortByStatus,
} from 'teleport/Integrations/helpers';
import api from 'teleport/services/api';
import {
  ExternalAuditStorageIntegration,
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
  Plugin,
} from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { ExternalAuditStorageOpType } from './Operations/useIntegrationOperation';
import { StatusLabel } from './shared/StatusLabel';
import { StatusOptions, type Status } from './types';

type Props = {
  list: IntegrationLike[];
  onDeletePlugin?(p: Plugin): void;
  integrationOps?: {
    onDeleteIntegration(i: Integration): void;
    onEditIntegration(i: Integration): void;
  };
  onDeleteExternalAuditStorage?(opType: ExternalAuditStorageOpType): void;
};

export type IntegrationLike =
  | Integration
  | Plugin
  | ExternalAuditStorageIntegration;

// statusKinds are the integration types with status pages; we enable clicking on the row directly to route to the view
const statusKinds = [
  'okta',
  'entra-id',
  IntegrationKind.AwsOidc,
  IntegrationKind.AwsRa,
];

type Filters = {
  Status: Status;
};

export function IntegrationList(props: Props) {
  const history = useHistory();

  function handleRowClick(row: IntegrationLike) {
    if ('isManagedByTerraform' in row && row.isManagedByTerraform) {
      history.push(cfg.getIaCIntegrationRoute(row.kind, row.name));
      return;
    }

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

  const [searchValue, setSearchValue] = useState<string>('');

  const [filters, setFilters] = useState<FilterMap<IntegrationLike, Filters>>({
    Status: {
      options: StatusOptions,
      selected: [],
      apply: filterByIntegrationStatus,
    },
  });

  const filteredList = useMemo(
    () => applyFilters(filterBySearch(props.list, searchValue), filters),
    [props.list, searchValue, filters]
  );

  const { clusterId } = useStickyClusterId();
  return (
    <>
      <Box mb={3}>
        <InputSearch
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
      </Box>
      <StyledPanel>
        <ListFilters filters={filters} onFilterChange={setFilters} />
      </StyledPanel>
      <Table
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
            render: item => <NameCell item={item} />,
          },
          {
            key: 'statusCode',
            headerText: 'Status',
            isSortable: true,
            onSort: sortByStatus,
            render: item => (
              <Cell>
                <StatusLabel integration={item} />
              </Cell>
            ),
          },
          {
            key: 'summary',
            headerText: 'Issues',
            render: item => <IssuesCell integration={item} />,
          },
          {
            key: 'details',
            headerText: 'Details',
            render: item => <DetailsCell integration={item} />,
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
                          to={cfg.getIntegrationStatusRoute(
                            item.kind,
                            item.name
                          )}
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
    </>
  );
}

const NameCell = ({ item }: { item: IntegrationLike }) => {
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

const IssuesCell = ({ integration }: { integration: IntegrationLike }) => {
  const issueCount = integration.summary?.unresolvedUserTasks?.length;
  if (issueCount > 0) {
    // In the list tooltip, we only want to show up to 3 tasks. If there are more,
    // the user can go to the integration dashboard to see all of them.
    const showTopX = 3;
    return (
      <Cell>
        <HoverTooltip
          tipContent={
            <Box>
              <Text fontWeight={600}>
                {issueCount.toLocaleString()} {pluralize(issueCount, 'Issue')}
              </Text>
              <TaskUL>
                {integration.summary.unresolvedUserTasks
                  .slice(0, showTopX)
                  .map(issue => (
                    <TaskLI key={issue.name}>{issue.title}</TaskLI>
                  ))}
              </TaskUL>
              {issueCount > showTopX && (
                <Text>
                  and {(issueCount - showTopX).toLocaleString()} more...
                </Text>
              )}
            </Box>
          }
          css={`
            display: inline-flex;
          `}
        >
          <PointerFlex inline alignItems="center" gap={1}>
            {issueCount.toLocaleString()}
            <Warning size="small" />
          </PointerFlex>
        </HoverTooltip>
      </Cell>
    );
  }
  return <Cell>-</Cell>;
};

const TaskUL = styled.ul`
  margin: 0;
  padding-left: ${p => p.theme.space[2]}px;
`;

const TaskLI = styled.li`
  list-style-position: inside;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
`;

const percentFormatter = new Intl.NumberFormat(undefined, {
  style: 'percent',
  minimumFractionDigits: 0,
  maximumFractionDigits: 1,
  roundingMode: 'trunc',
});

export function percent(n: number, d: number): string {
  if (!Number.isFinite(n) || !Number.isFinite(d) || d <= 0) {
    return '-';
  }

  const ratio = n / d;

  if (isNaN(ratio)) {
    return '-';
  }

  return percentFormatter.format(ratio);
}

const DetailsCell = ({ integration }: { integration: IntegrationLike }) => {
  let content = <Text>{integration.details}</Text>;
  switch (integration.kind) {
    case IntegrationKind.AwsOidc:
    case IntegrationKind.AzureOidc:
      if (integration.summary?.resourcesCount) {
        const rc = integration.summary.resourcesCount;
        const hasFailures = rc.failed > 0;
        const enrolledPct = percent(rc.enrolled, rc.found);
        const failedPct = percent(rc.failed, rc.found);
        content = (
          <HoverTooltip
            tipContent={
              <Box>
                <Text fontWeight={600}>
                  {rc.found.toLocaleString()} Resources Found
                </Text>
                <Flex alignItems="center" mt={1} gap={1}>
                  <CircleCheck size="small" color="success.main" />
                  <Text>
                    {rc.enrolled.toLocaleString()} enrolled ({enrolledPct})
                  </Text>
                </Flex>
                {hasFailures && (
                  <>
                    <Flex alignItems="center" gap={1}>
                      <CircleCross size="small" color="error.main" />
                      <Text>
                        {rc.failed.toLocaleString()} failed ({failedPct})
                      </Text>
                    </Flex>
                    <Text mt={1}>
                      For more information on failures, please check the
                      integration overview.
                    </Text>
                  </>
                )}
              </Box>
            }
            css={`
              display: inline-flex;
            `}
          >
            <PointerFlex inline gap={1}>
              {rc.enrolled.toLocaleString()} Resources enrolled ({enrolledPct})
            </PointerFlex>
          </HoverTooltip>
        );
      }
      break;
  }
  return <Cell>{content}</Cell>;
};

const PointerFlex = styled(Flex)`
  cursor: pointer;
`;
