/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { ComponentType } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import {
  CircleCheck,
  CircleCross,
  Info,
  Question,
  Warning,
  Wrench,
} from 'design/Icon';
import { IconSize } from 'design/Icon/Icon';
import Label, {
  DangerOutlined,
  SecondaryOutlined,
  WarningOutlined,
} from 'design/Label/Label';
import { HoverTooltip } from 'design/Tooltip';
import { pluralize } from 'shared/utils/text';

import {
  IntegrationStatusCode,
  IntegrationWithSummary,
} from 'teleport/services/integrations';

import { IntegrationLike } from '../IntegrationList';
import { Status } from '../types';

export const StatusLabel = ({
  integration,
}: {
  integration: IntegrationLike;
}) => {
  const { status, label, tooltip } = getStatus(integration);

  return (
    <HoverTooltip
      tipContent={
        <Box>
          <Text fontWeight={600}>Status</Text>
          <Text>{tooltip}</Text>
        </Box>
      }
    >
      <PointerFlex inline alignItems="center">
        {statusLabel(status, label)}
      </PointerFlex>
    </HoverTooltip>
  );
};

export const SummaryStatusLabel = ({
  summary,
}: {
  summary: IntegrationWithSummary;
}) => {
  const hasIssues = summary.unresolvedUserTasks > 0;
  const isSyncing = isSummarySyncing(summary);
  const { status, label } = isSyncing
    ? SCANNING()
    : hasIssues
      ? ISSUES()
      : HEALTHY;
  return (
    <DefaultFlex alignItems="center" gap={2} flexWrap="wrap">
      {statusLabel(status, label)}
      {isSyncing && (
        <StatusNote typography="body3">(scanning in progress)</StatusNote>
      )}
    </DefaultFlex>
  );
};

const PointerFlex = styled(Flex)`
  cursor: pointer;
`;
const DefaultFlex = styled(Flex)`
  cursor: default;
`;

const StatusNote = styled(Text)`
  display: flex;
  align-items: center;
  line-height: 1;
`;

const HEALTHY = {
  status: Status.Healthy,
  label: 'Healthy',
  tooltip: 'Integration is connected and active.',
};

const DRAFT = {
  status: Status.Draft,
  label: 'Draft',
  tooltip: 'Integration setup has not been completed.',
};

const FAILED = (tooltip: string) => ({
  status: Status.Failed,
  label: 'Failed',
  tooltip,
});

const UNKNOWN = (tooltip: string) => ({
  status: Status.Unknown,
  label: 'Unknown',
  tooltip,
});

const ISSUES = (tooltip?: string) => ({
  status: Status.Issues,
  label: 'Issues',
  tooltip,
});

const SCANNING = (tooltip?: string) => ({
  status: Status.Scanning,
  label: 'Scanning',
  tooltip,
});

const PrimaryOutlined = (props: { children: React.ReactNode }) => (
  <Label kind="outline-primary" {...props} />
);

export function getStatus(item: IntegrationLike): {
  status: Status;
  label: string;
  tooltip: string;
} {
  if (item.resourceType === 'integration') {
    const issueCount = item.summary?.unresolvedUserTasks.length ?? 0;
    const hasIssues = issueCount > 0;
    return hasIssues
      ? ISSUES(
          `Integration is active but has ${issueCount} ${pluralize(issueCount, 'issue')} to address. Check the integration overview for more details.`
        )
      : HEALTHY;
  }

  if (item.resourceType === 'external-audit-storage') {
    return item.statusCode === IntegrationStatusCode.Draft ? DRAFT : HEALTHY;
  }

  switch (item.statusCode) {
    case IntegrationStatusCode.Unknown:
      return UNKNOWN(
        'Integration is in an unknown state. If this state persists, try removing and re-connecting the integration.'
      );
    case IntegrationStatusCode.Running:
      return HEALTHY;
    case IntegrationStatusCode.Draft:
      return DRAFT;
    case IntegrationStatusCode.SlackNotInChannel:
      return ISSUES(
        'The Slack integration must be invited to the default channel in order to receive access request notifications.'
      );
    case IntegrationStatusCode.Unauthorized:
      return FAILED(
        'Integration was denied access. This could be a result of revoked authorization on the 3rd party provider. Try removing and re-connecting the integration.'
      );
    case IntegrationStatusCode.OktaConfigError:
      return FAILED(
        `There was an error with the integration's configuration.${item.status?.errorMessage ? ` ${item.status.errorMessage}` : ''}`
      );
    default:
      return FAILED(
        'Integration failed due to an unknown error. Try removing and re-connecting the integration.'
      );
  }
}

const StatusUI: Record<
  Status,
  {
    Icon: ComponentType<{ size?: IconSize }>;
    Label: ComponentType<{ children: React.ReactNode }>;
  }
> = {
  [Status.Healthy]: {
    Icon: CircleCheck,
    Label: SecondaryOutlined,
  },
  [Status.Draft]: {
    Icon: Wrench,
    Label: SecondaryOutlined,
  },
  [Status.Unknown]: {
    Icon: Question,
    Label: SecondaryOutlined,
  },
  [Status.Failed]: {
    Icon: CircleCross,
    Label: DangerOutlined,
  },
  [Status.Issues]: {
    Icon: Warning,
    Label: WarningOutlined,
  },
  [Status.Scanning]: {
    Icon: Info,
    Label: PrimaryOutlined,
  },
};

const statusLabel = (status: Status, label: string) => {
  const { Icon, Label } = StatusUI[status];

  return (
    <Label aria-label="status">
      <Flex alignItems="center" gap={1}>
        <Icon size="small" />
        {label}
      </Flex>
    </Label>
  );
};

function isSummarySyncing(summary: IntegrationWithSummary): boolean {
  const lastSync = Math.max(
    getTimestamp(summary.awsec2?.discoverLastSync),
    getTimestamp(summary.awsrds?.discoverLastSync),
    getTimestamp(summary.awseks?.discoverLastSync),
    getTimestamp(summary.rolesAnywhereProfileSync?.syncEndTime)
  );

  return lastSync === 0;
}

function getTimestamp(value: unknown): number {
  if (!value) {
    return 0;
  }
  if (value instanceof Date) {
    return value.getTime();
  }
  if (typeof value === 'number') {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Date.parse(value);
    return Number.isNaN(parsed) ? 0 : parsed;
  }
  return 0;
}
