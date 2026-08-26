/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useQuery } from '@tanstack/react-query';
import { formatDistanceStrict, formatDistanceToNowStrict } from 'date-fns';
import { useEffect, useState } from 'react';
import { useParams } from 'react-router';
import { Link as RouterLink } from 'react-router-dom';
import styled, { keyframes } from 'styled-components';

import { Card, Flex, H2, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import { ButtonPrimary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import { displayDateTime } from 'design/datetime';
import { ArrowLeft, CircleCheck, Clock, SyncAlt } from 'design/Icon';
import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import { Status as StatusBadge } from 'design/Status';
import { HoverTooltip } from 'design/Tooltip';
import { pluralize } from 'shared/utils/text';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';
import {
  IntegrationKind,
  integrationService,
  IntegrationWithSummary,
  ResourceTypeSummary,
} from 'teleport/services/integrations';

import { ActivityTab } from './ActivityTab';

export function formatRelativeDate(value?: string | Date): string {
  if (!value) {
    return '-';
  }

  const date = value instanceof Date ? value : new Date(Date.parse(value));
  if (Number.isNaN(date.getTime())) {
    return typeof value === 'string' ? value : '-';
  }

  try {
    return `${formatDistanceToNowStrict(date)} ago`;
  } catch {
    return displayDateTime(date);
  }
}
const OVERDUE_REFETCH_INTERVAL_MS = 10 * 1000;

export function IaCIntegrationOverview() {
  const { name, type } = useParams<{ name: string; type: string }>();

  const {
    data: stats,
    error,
    isLoading,
    isError,
  } = useQuery<IntegrationWithSummary>({
    queryKey: ['integrationStats', name],
    queryFn: () => integrationService.fetchIntegrationStats(name),
    enabled: !!name,
    refetchInterval: query => getStatsRefetchIntervalMs(query.state.data),
  });

  useNoMinWidth();

  if (isLoading) {
    return (
      <FeatureBox pt={3}>
        <Flex justifyContent="center" mt={6}>
          <Indicator delay="long" />
        </Flex>
      </FeatureBox>
    );
  }

  if (isError) {
    return (
      <FeatureBox maxWidth="1400px" pt={3}>
        <Danger>{error?.message || 'Failed to load integration stats'}</Danger>
      </FeatureBox>
    );
  }

  const settingsPath = cfg.getIaCIntegrationSettingsRoute(
    type as IntegrationKind,
    name
  );

  const roleArn = stats.awsoidc?.roleArn;

  return (
    <FeatureBox maxWidth="1400px" pt={3}>
      <Flex alignItems="center" justifyContent="space-between" mb={4}>
        <Flex alignItems="center">
          <HoverTooltip placement="bottom" tipContent="Back to Integrations">
            <ButtonIcon as={RouterLink} to={cfg.routes.integrations} mr={2}>
              <ArrowLeft size="medium" />
            </ButtonIcon>
          </HoverTooltip>
          <Flex flexDirection="column">
            <Text bold fontSize={6}>
              {stats.name}
            </Text>
            {roleArn && (
              <Text typography="body3" color="text.slightlyMuted">
                {roleArn}
              </Text>
            )}
          </Flex>
        </Flex>
        <ButtonPrimary as={RouterLink} to={settingsPath} size="small">
          Edit configuration
        </ButtonPrimary>
      </Flex>

      <OverviewContent stats={stats} />
    </FeatureBox>
  );
}

function OverviewContent({ stats }: { stats: IntegrationWithSummary }) {
  const [nowMs, setNowMs] = useState(Date.now);

  useEffect(() => {
    const interval = window.setInterval(() => {
      setNowMs(Date.now());
    }, 30_000);

    return () => {
      window.clearInterval(interval);
    };
  }, []);

  const hasIssues = stats.unresolvedUserTasks > 0;
  const isAzure = stats.subKind === IntegrationKind.AzureOidc;

  return (
    <Flex flexDirection="column" gap={4}>
      <CardsContainer>
        {isAzure ? (
          <ResourceTypeCard
            resourceType="azurevm"
            summary={stats.azurevm}
            nowMs={nowMs}
          />
        ) : (
          <>
            <ResourceTypeCard
              resourceType="ec2"
              summary={stats.awsec2}
              nowMs={nowMs}
            />
            <ResourceTypeCard
              resourceType="rds"
              summary={stats.awsrds}
              nowMs={nowMs}
            />
            <ResourceTypeCard
              resourceType="eks"
              summary={stats.awseks}
              nowMs={nowMs}
            />
          </>
        )}
      </CardsContainer>
      <IntegrationHealthCard stats={stats} hasIssues={hasIssues} />
    </Flex>
  );
}

function IntegrationHealthCard({
  stats,
  hasIssues,
}: {
  stats: IntegrationWithSummary;
  hasIssues: boolean;
}) {
  return (
    <Card p={4}>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <H2>Integration Health</H2>
      </Flex>

      <Flex alignItems="center" gap={2} mb={hasIssues ? 4 : 0}>
        <Text typography="body2">Status:</Text>
        {hasIssues ? (
          <Flex alignItems="center" gap={2}>
            <StatusBadge kind="warning" variant="filled-tonal">
              Needs review
            </StatusBadge>
            <StatusBadge kind="danger" variant="filled-tonal">
              {stats.unresolvedUserTasks}{' '}
              {pluralize(stats.unresolvedUserTasks, 'issue')} detected
            </StatusBadge>
          </Flex>
        ) : (
          <Flex alignItems="center" gap={2}>
            <StatusBadge kind="success" variant="filled-tonal">
              Healthy
            </StatusBadge>
            <Text typography="body3" color="text.slightlyMuted">
              No issues detected
            </Text>
          </Flex>
        )}
      </Flex>

      {hasIssues && <ActivityTab stats={stats} />}
    </Card>
  );
}

type ResourceType = 'ec2' | 'rds' | 'eks' | 'azurevm';

const RESOURCE_TYPE_CONFIG: Record<
  ResourceType,
  { icon: ResourceIconName; label: string; instanceLabel: string }
> = {
  ec2: {
    icon: 'awsec2',
    label: 'EC2 Discovered Instances',
    instanceLabel: 'instances',
  },
  rds: {
    icon: 'awsrds',
    label: 'RDS Discovered Instances',
    instanceLabel: 'instances',
  },
  azurevm: {
    icon: 'azure',
    label: 'Azure Discovered VMs',
    instanceLabel: 'VMs',
  },
  eks: {
    icon: 'eks',
    label: 'EKS Discovered Clusters',
    instanceLabel: 'clusters',
  },
};

function getResourceScanState(
  summary: ResourceTypeSummary | undefined
): ScanState {
  if (!summary) {
    return { state: 'waiting' };
  }

  // Check for valid dates - protobuf zero timestamps convert to Unix epoch (1970)
  const parsedStart = summary.syncStart ? new Date(summary.syncStart) : null;
  const parsedEnd = summary.syncEnd ? new Date(summary.syncEnd) : null;
  const syncStart = parsedStart?.getTime() > 0 ? parsedStart : null;
  const syncEnd = parsedEnd?.getTime() > 0 ? parsedEnd : null;

  if (!syncStart) {
    return { state: 'waiting' };
  }

  if (!syncEnd) {
    return { state: 'scanning', startedAt: syncStart };
  }

  const pollIntervalMs = (summary.pollIntervalSeconds || 0) * 1000;
  const nextScanAt = new Date(syncEnd.getTime() + pollIntervalMs);
  return { state: 'completed', lastScan: syncEnd, nextScanAt };
}

function ResourceTypeCard({
  resourceType,
  summary,
  nowMs,
}: {
  resourceType: ResourceType;
  summary: ResourceTypeSummary | undefined;
  nowMs: number;
}) {
  const config = RESOURCE_TYPE_CONFIG[resourceType];
  const isConfigured = summary && summary.rulesCount > 0;
  const scanState = getResourceScanState(summary);

  if (!isConfigured) {
    return (
      <ResourceCard style={{ opacity: 0.6 }}>
        <Flex alignItems="center" gap={2} mb={3}>
          <ResourceIcon name={config.icon} size={24} />
          <Text bold typography="body1">
            {config.label}
          </Text>
        </Flex>
        <Flex
          flex="1"
          alignItems="center"
          justifyContent="center"
          color="text.muted"
        >
          <Text typography="body3">
            {resourceType.toUpperCase()} not configured.
          </Text>
        </Flex>
      </ResourceCard>
    );
  }

  const discovered = summary.resourcesFound || 0;
  const enrolled = summary.resourcesEnrollmentSuccess || 0;
  const enrollmentPercent = discovered > 0 ? (enrolled / discovered) * 100 : 0;
  const isFullyEnrolled = discovered > 0 && enrolled === discovered;

  const renderScanBadge = () => {
    switch (scanState.state) {
      case 'waiting':
        return null;
      case 'scanning':
        return (
          <StatusBadge
            kind="info"
            variant="filled-tonal"
            icon={SpinningSyncIcon}
          >
            Rescanning now...
          </StatusBadge>
        );
      case 'completed': {
        const remainingMs = scanState.nextScanAt.getTime() - nowMs;
        if (remainingMs > 0) {
          return (
            <Flex alignItems="center" gap={1} color="text.slightlyMuted">
              <Clock size="small" />
              <Text typography="body3">
                Next scan in{' '}
                {formatDistanceStrict(0, remainingMs, {
                  unit: 'minute',
                  roundingMethod: 'ceil',
                })}
              </Text>
            </Flex>
          );
        }
        // Countdown expired, show rescanning state while waiting for backend to confirm
        return (
          <StatusBadge
            kind="info"
            variant="filled-tonal"
            icon={SpinningSyncIcon}
          >
            Rescanning now...
          </StatusBadge>
        );
      }
    }
  };

  return (
    <ResourceCard>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <Flex alignItems="center" gap={2}>
          <ResourceIcon name={config.icon} size={24} />
          <Text bold typography="body1">
            {config.label}
          </Text>
        </Flex>
        {renderScanBadge()}
      </Flex>

      <CardDivider />

      <Text fontSize={6} bold mb={1} mt={3}>
        {discovered}
      </Text>
      <Text typography="body3" color="text.slightlyMuted" mb={3}>
        Discovered {config.instanceLabel}
      </Text>

      <Flex alignItems="center" gap={2} mb={2}>
        {isFullyEnrolled && <CircleCheck size="small" color="success.main" />}
        <Text typography="body3">
          {Math.round(enrollmentPercent)}% enrolled ({enrolled} of {discovered})
        </Text>
      </Flex>

      <ProgressBar>
        <ProgressFill
          $percent={enrollmentPercent}
          $isComplete={isFullyEnrolled}
        />
      </ProgressBar>

      {scanState.state === 'completed' &&
        scanState.nextScanAt.getTime() - nowMs > 0 && (
          <Text typography="body3" color="text.slightlyMuted" mt={2}>
            Last scan {formatRelativeDate(scanState.lastScan)}
          </Text>
        )}
    </ResourceCard>
  );
}

const CardsContainer = styled.div`
  display: flex;
  gap: ${props => props.theme.space[3]}px;

  @media (max-width: 744px) {
    flex-direction: column;
  }
`;

const ResourceCard = styled(Card)`
  flex: 1;
  padding: ${props => props.theme.space[3]}px;
  display: flex;
  flex-direction: column;
`;

const CardDivider = styled.hr`
  width: 100%;
  height: 1px;
  border: none;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  margin: 0;
`;

const ProgressBar = styled.div`
  width: 100%;
  height: 8px;
  background-color: ${props => props.theme.colors.levels.sunken};
  border-radius: 4px;
  overflow: hidden;
`;

const ProgressFill = styled.div<{ $percent: number; $isComplete: boolean }>`
  height: 100%;
  width: ${props => props.$percent}%;
  background-color: ${props =>
    props.$isComplete
      ? props.theme.colors.success.main
      : props.theme.colors.info};
  border-radius: 4px;
  transition: width 0.3s ease;
`;

const spin = keyframes`
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
`;

const SpinningSyncIcon = styled(SyncAlt)`
  animation: ${spin} 1s linear infinite;
`;

type ScanState =
  | { state: 'waiting' }
  | { state: 'scanning'; startedAt: Date }
  | { state: 'completed'; lastScan: Date; nextScanAt: Date };

function getScanState(stats: IntegrationWithSummary): ScanState {
  const summaries: ResourceTypeSummary[] = [
    stats.awsec2,
    stats.awsrds,
    stats.awseks,
    stats.azurevm,
  ].filter(Boolean);

  let latestCompleted: (ScanState & { state: 'completed' }) | null = null;
  let latestScanning: (ScanState & { state: 'scanning' }) | null = null;

  for (const summary of summaries) {
    const state = getResourceScanState(summary);
    if (state.state === 'completed') {
      if (!latestCompleted || state.lastScan > latestCompleted.lastScan) {
        latestCompleted = state;
      }
    } else if (state.state === 'scanning') {
      if (!latestScanning || state.startedAt > latestScanning.startedAt) {
        latestScanning = state;
      }
    }
  }

  if (latestScanning) {
    return latestScanning;
  }

  if (latestCompleted) {
    return latestCompleted;
  }

  return { state: 'waiting' };
}

function getStatsRefetchIntervalMs(stats: IntegrationWithSummary | undefined) {
  if (!stats) {
    return OVERDUE_REFETCH_INTERVAL_MS;
  }

  const scanState = getScanState(stats);

  if (scanState.state !== 'completed') {
    return OVERDUE_REFETCH_INTERVAL_MS;
  }

  const remainingMs = scanState.nextScanAt.getTime() - Date.now();
  return remainingMs <= 0 ? OVERDUE_REFETCH_INTERVAL_MS : remainingMs;
}
