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
import { CircleCheck, CircleCross, CircleDashed, Warning } from 'design/Icon';
import { IconSize } from 'design/Icon/Icon';
import {
  DangerAccessible,
  SecondaryAccessible,
  WarningAccessible,
} from 'design/Label/Label';
import { HoverTooltip } from 'design/Tooltip';

import { IntegrationStatusCode } from 'teleport/services/integrations';

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

const PointerFlex = styled(Flex)`
  cursor: pointer;
`;

const HEALTHY = {
  status: Status.Healthy,
  label: 'Healthy',
  tooltip: 'Integration is connected and working.',
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

const ISSUES = (tooltip: string) => ({
  status: Status.Issues,
  label: 'Issues',
  tooltip,
});

export function getStatus(item: IntegrationLike): {
  status: Status;
  label: string;
  tooltip: string;
} {
  if (item.resourceType === 'integration') {
    return HEALTHY;
  }

  if (item.resourceType === 'external-audit-storage') {
    return item.statusCode === IntegrationStatusCode.Draft ? DRAFT : HEALTHY;
  }

  switch (item.statusCode) {
    case IntegrationStatusCode.Unknown:
      return UNKNOWN(
        'The integration is in an unknown state. If this state persists, try removing and re-connecting the integration.'
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
        'The integration was denied access. This could be a result of revoked authorization on the 3rd party provider. Try removing and re-connecting the integration.'
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
    Label: SecondaryAccessible,
  },
  [Status.Draft]: {
    Icon: CircleDashed,
    Label: SecondaryAccessible,
  },
  [Status.Unknown]: {
    Icon: CircleDashed,
    Label: SecondaryAccessible,
  },
  [Status.Failed]: {
    Icon: CircleCross,
    Label: DangerAccessible,
  },
  [Status.OktaConfigError]: {
    Icon: CircleCross,
    Label: DangerAccessible,
  },
  [Status.Issues]: {
    Icon: Warning,
    Label: WarningAccessible,
  },
};

const statusLabel = (status: Status, label: string) => {
  const { Icon, Label } = StatusUI[status];

  return (
    <Label>
      <Flex alignItems="center" gap={1} p={0.5}>
        <Icon size="small" />
        {label}
      </Flex>
    </Label>
  );
};
