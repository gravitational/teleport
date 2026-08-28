/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { ReactNode } from 'react';
import styled, { useTheme } from 'styled-components';

import { Alert, Box, Button, Flex, H2, Link, Text } from 'design';
import { ArrowSquareOut } from 'design/Icon';

import cfg from 'teleport/config';
import { GroupState } from 'teleport/services/managedUpdates';

export const DOCS_URL =
  'https://goteleport.com/docs/upgrading/agent-managed-updates/';
export const TOOLS_DOCS_URL =
  'https://goteleport.com/docs/upgrading/client-tools-managed-updates/';
export const SUPPORT_URL = 'https://support.goteleport.com';
export const POLLING_INTERVAL_MS = 60_000; // 1 minute

export const Card = styled(Box)`
  background-color: ${p => p.theme.colors.levels.surface};
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[2]};
`;

export const CardTitle = styled(H2)`
  font-size: 18px;
  font-weight: 400;
  margin-bottom: ${p => p.theme.space[2]}px;
`;

export const TableContainer = styled(Box)`
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${p => p.theme.radii[3]}px;
  overflow-x: auto;

  table {
    border-collapse: collapse;
    width: 100%;

    thead tr {
      background-color: ${p =>
        p.theme.type === 'light'
          ? p.theme.colors.levels.deep
          : p.theme.colors.levels.elevated};
      cursor: default;
      &:hover {
        background-color: ${p =>
          p.theme.type === 'light'
            ? p.theme.colors.levels.deep
            : p.theme.colors.levels.elevated};
      }
    }

    thead > tr > th {
      ${p => p.theme.typography.h3};
      padding-top: ${p => p.theme.space[2]}px;
      padding-bottom: ${p => p.theme.space[2]}px;
      text-align: left;
      color: ${p => p.theme.colors.text.main};
      border-bottom: 1px solid
        ${p => p.theme.colors.interactive.tonal.neutral[2]};
    }

    tbody tr {
      cursor: pointer;
      transition: background-color 0.15s ease;
      height: 68px;
      border-top: none;
      &:hover {
        border-top: none;
        background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
        &:after {
          box-shadow: none;
        }
        + tr {
          border-top: none;
        }
      }
      &:not(:last-child) {
        border-bottom: 1px solid
          ${p => p.theme.colors.interactive.tonal.neutral[0]};
      }
    }

    td {
      padding: ${p => p.theme.space[2]}px;
      vertical-align: middle;
    }
  }
`;

export const VersionTableContainer = styled(TableContainer)`
  table {
    thead > tr > th {
      padding-top: ${p => p.theme.space[1]}px;
      padding-bottom: ${p => p.theme.space[1]}px;
    }
    tbody tr {
      height: auto;
      cursor: unset;
      &:hover {
        background-color: transparent;
        border-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
      }
      &:hover:after {
        box-shadow: none;
      }
      &:hover + tr {
        border-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
      }
    }
  }
`;

const StatusDot = styled(Box)<{ $color: string }>`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: ${p => p.$color};
`;

const ProgressBarContainer = styled(Box)<{ $backgroundColor: string }>`
  width: 100%;
  height: 6px;
  background-color: ${p => p.$backgroundColor};
  border-radius: ${p => p.theme.radii[2]}px;
  overflow: hidden;
`;

const ProgressBarFill = styled(Box)<{ $percent: number; $color: string }>`
  width: ${p => p.$percent}%;
  height: 100%;
  background-color: ${p => p.$color};
  border-radius: ${p => p.theme.radii[2]}px;
  transition: width 0.3s ease;
`;

export function InfoItem({
  label,
  value,
  valueLink,
  labelWidth = 140,
  mb = 1,
}: {
  label: string;
  value: ReactNode;
  valueLink?: string;
  labelWidth?: number;
  mb?: number;
}) {
  return (
    <Flex gap={2} mb={mb}>
      <Text
        color="text.muted"
        bold
        css={`
          min-width: ${labelWidth}px;
        `}
      >
        {label}:
      </Text>
      {valueLink ? (
        <Link
          href={valueLink}
          target="_blank"
          css={`
            display: inline-flex;
            align-items: center;
          `}
        >
          {value} <ArrowSquareOut size="small" ml={1} />
        </Link>
      ) : (
        <Text>{value}</Text>
      )}
    </Flex>
  );
}

export function DocsLink({ docsUrl }: { docsUrl: string }) {
  const theme = useTheme();
  return (
    <Link
      href={docsUrl}
      target="_blank"
      css={`
        white-space: nowrap;
        display: inline-flex;
        align-items: center;
        color: ${theme.colors.interactive.solid.primary.default};
        text-decoration: none;
        font-weight: 500;
        &:hover {
          color: ${theme.colors.interactive.solid.primary.hover};
        }
      `}
    >
      See guide in Docs <ArrowSquareOut size="small" ml={1} />
    </Link>
  );
}

export function StatusBadge({ state }: { state: GroupState }) {
  const theme = useTheme();
  const config = {
    done: {
      label: 'Done',
      color: theme.colors.interactive.solid.success.default,
    },
    active: {
      label: 'In progress',
      color: theme.colors.interactive.solid.accent.default,
    },
    canary: {
      label: 'In progress',
      color: theme.colors.interactive.solid.accent.default,
    },
    rolledback: {
      label: 'Rolled back',
      color: theme.colors.interactive.solid.alert.default,
    },
    unstarted: {
      label: 'Scheduled',
      color: theme.colors.interactive.solid.primary.default,
    },
  }[state] || {
    label: 'Scheduled',
    color: theme.colors.interactive.solid.primary.default,
  };

  return (
    <Flex alignItems="center" gap={2}>
      <StatusDot $color={config.color} />
      <Text typography="body2">{config.label}</Text>
    </Flex>
  );
}

export function ProgressBar({ percent }: { percent: number }) {
  const theme = useTheme();

  let color: string;
  if (percent >= 100) {
    color =
      theme.type === 'light'
        ? theme.colors.dataVisualisation.secondary.caribbean
        : theme.colors.dataVisualisation.primary.caribbean;
  } else {
    color =
      theme.type === 'light'
        ? theme.colors.dataVisualisation.secondary.picton
        : theme.colors.dataVisualisation.primary.picton;
  }

  const backgroundColor =
    theme.type === 'light'
      ? theme.colors.interactive.tonal.neutral[2]
      : theme.colors.levels.popout;

  return (
    <ProgressBarContainer $backgroundColor={backgroundColor}>
      <ProgressBarFill $percent={Math.min(percent, 100)} $color={color} />
    </ProgressBarContainer>
  );
}

export function NotConfiguredText({ docsUrl }: { docsUrl: string }) {
  if (cfg.isCloud) {
    return (
      <Text
        css={`
          font-style: italic;
        `}
      >
        Could not detect a configuration for this feature.
      </Text>
    );
  }

  return (
    <>
      <Text color="text.slightlyMuted" mb={3}>
        Follow the guide to set this up for your cluster.
      </Text>
      <Button as="a" href={docsUrl} target="_blank" px={3}>
        View configuration guide in Docs
        <ArrowSquareOut size="small" ml={2} />
      </Button>
    </>
  );
}

export function NoPermissionCardContent() {
  return (
    <Text
      color="text.muted"
      css={`
        font-style: italic;
      `}
    >
      Missing required permissions to view.
    </Text>
  );
}

export function MissingPermissionsBanner({
  missingPermissions,
}: {
  missingPermissions: string[];
}) {
  return (
    <Alert kind="info" mb={3}>
      You do not have all the required permissions.
      <Text typography="body2">
        Missing role permissions:{' '}
        {missingPermissions.map((p, i) => (
          <span key={p}>
            <code>{p}</code>
            {i < missingPermissions.length - 1 && ', '}
          </span>
        ))}
      </Text>
    </Alert>
  );
}
