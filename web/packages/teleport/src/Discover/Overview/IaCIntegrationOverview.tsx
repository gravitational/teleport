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
import { useState, useEffect, type ReactNode } from 'react';
import { Link as RouterLink, useParams } from 'react-router';

import { Box, Card, Flex, H2, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import { ButtonPrimaryBorder, ButtonText } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import { displayDateTime } from 'design/datetime';
import {
  ArrowLeft,
  ArrowRight,
  ChevronRight,
  CircleCross,
  Warning,
} from 'design/Icon';
import { TabBorder, useSlidingBottomBorderTabs } from 'design/Tabs';
import { HoverTooltip } from 'design/Tooltip';
import { pluralize } from 'shared/utils/text';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import {
  ContentWithSidePanel,
  InfoGuideSwitch,
  useTerraformInfoGuide,
} from 'teleport/Integrations/Enroll/Cloud/Shared/InfoGuide';
import {
  latestSyncDate,
  SummaryStatusLabel,
} from 'teleport/Integrations/shared/StatusLabel';
import { useNoMinWidth } from 'teleport/Main';
import {
  INTEGRATION_DISCOVERY_SCAN_INTERVAL_MS,
  IntegrationKind,
  integrationService,
  IntegrationWithSummary,
} from 'teleport/services/integrations';

import { ActivityTab } from './ActivityTab';
import { SettingsTab } from './SettingsTab';
import { SmallTab, SmallTabsContainer } from './SmallTabs';

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
type TabId = 'overview' | 'activity' | 'settings';

const TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'activity', label: 'Issues' },
  { id: 'settings', label: 'Settings' },
] as const;

const OVERDUE_REFETCH_INTERVAL_MS = 10 * 1000;

export function IaCIntegrationOverview() {
  const { name } = useParams<{ name: string }>();

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

  const [activeTab, setActiveTab] = useState<TabId>('overview');
  const { activeInfoGuideTab, setActiveInfoGuideTab } = useTerraformInfoGuide();
  const { borderRef, parentRef } = useSlidingBottomBorderTabs({ activeTab });
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

  const isPanelOpen = activeTab === 'settings' && !!activeInfoGuideTab;

  return (
    <FeatureBox maxWidth="1400px" pt={3}>
      <ContentWithSidePanel isPanelOpen={isPanelOpen}>
        <Flex alignItems="center" justifyContent="space-between" mb={3}>
          <Flex alignItems="center">
            <HoverTooltip placement="bottom" tipContent="Back to Integrations">
              <ButtonIcon as={RouterLink} to={cfg.routes.integrations} mr={2}>
                <ArrowLeft size="medium" />
              </ButtonIcon>
            </HoverTooltip>
            <Text bold fontSize={6} mr={2}>
              {stats.name}
            </Text>
          </Flex>
          {activeTab === 'settings' && (
            <InfoGuideSwitch
              isPanelOpen={isPanelOpen}
              activeTab={activeInfoGuideTab}
              onSwitch={setActiveInfoGuideTab}
            />
          )}
        </Flex>
        <SmallTabsContainer ref={parentRef} mb={4}>
          {TABS.map(t => (
            <SmallTab
              key={t.id}
              data-tab-id={t.id}
              selected={activeTab === t.id}
              onClick={() => setActiveTab(t.id)}
            >
              {t.label}
            </SmallTab>
          ))}
          <TabBorder ref={borderRef} />
        </SmallTabsContainer>

        {activeTab === 'overview' && (
          <OverviewTab
            stats={stats}
            onViewIssues={() => setActiveTab('activity')}
          />
        )}
        {activeTab === 'activity' && <ActivityTab stats={stats} />}
        {activeTab === 'settings' && (
          <SettingsTab
            stats={stats}
            activeInfoGuideTab={activeInfoGuideTab}
            onInfoGuideTabChange={setActiveInfoGuideTab}
          />
        )}
      </ContentWithSidePanel>
    </FeatureBox>
  );
}

function OverviewTab({
  stats,
  onViewIssues,
}: {
  stats: IntegrationWithSummary;
  onViewIssues: () => void;
}) {
  return (
    <Flex flexDirection="column" gap={4}>
      <IntegrationHealthCard stats={stats} onViewIssues={onViewIssues} />
      <IssuesCard stats={stats} onViewIssues={onViewIssues} />
    </Flex>
  );
}

function IntegrationHealthCard({
  stats,
  onViewIssues,
}: {
  stats: IntegrationWithSummary;
  onViewIssues: () => void;
}) {
  const [isConfigOpen, setIsConfigOpen] = useState(false);
  const [nowMs, setNowMs] = useState(Date.now);

  useEffect(() => {
    const interval = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(interval);
    };
  }, []);

  const hasIssues = stats.unresolvedUserTasks > 0;
  const lastScanDate = latestSyncDate(stats);
  const lastScanText = formatRelativeDate(lastScanDate);
  const nextScanText = formatTimeUntilNextScan(lastScanDate, nowMs);
  const configDetails = getIntegrationConfigDetails(stats);

  return (
    <Card p={4}>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <H2>Integration Health</H2>
      </Flex>

      <Flex flexDirection="column" gap={2}>
        <StatusRow label="Status:">
          <Flex alignItems="center" gap={2} flexWrap="wrap">
            <SummaryStatusLabel summary={stats} />
            {hasIssues && (
              <>
                <Text typography="body3">
                  ({stats.unresolvedUserTasks}{' '}
                  {pluralize(stats.unresolvedUserTasks, 'Issue')} detected)
                </Text>
                <ButtonText
                  intent="primary"
                  size="small"
                  onClick={onViewIssues}
                >
                  <Flex alignItems="center" gap={1}>
                    View Issues <ArrowRight color="text.brand" size={16} />
                  </Flex>
                </ButtonText>
              </>
            )}
          </Flex>
        </StatusRow>

        <StatusRow label="Last verified:">
          <Flex alignItems="center" gap={2} flexWrap="wrap">
            <Text typography="body3">{lastScanText}</Text>
            {lastScanDate && nextScanText && (
              <Text typography="body3" color="text.slightlyMuted">
                {nextScanText}
              </Text>
            )}
          </Flex>
        </StatusRow>

        <StatusRow label="Resources:">
          <Text typography="body3">{formatResourceCounts(stats)}</Text>
        </StatusRow>
      </Flex>

      <Box mt={3} ml={-2}>
        <Flex
          alignItems="center"
          onClick={() => setIsConfigOpen(v => !v)}
          role="button"
          tabIndex={0}
          style={{ cursor: 'pointer' }}
        >
          <ButtonIcon
            aria-label={
              isConfigOpen
                ? 'Collapse configuration details'
                : 'Expand configuration details'
            }
            onClick={e => {
              e.stopPropagation();
              setIsConfigOpen(v => !v);
            }}
          >
            <ChevronRight
              size={16}
              style={{
                // point right for closed, down for open
                transform: isConfigOpen ? 'rotate(90deg)' : undefined,
                transition: 'transform 150ms ease',
              }}
            />
          </ButtonIcon>
          <Text typography="body3">Configuration details</Text>
        </Flex>

        {isConfigOpen && (
          <Box mt={1} ml={5}>
            {configDetails.map(detail => (
              <KeyValue
                key={`${detail.label}-${detail.value}`}
                label={detail.label}
                value={detail.value}
              />
            ))}
            <KeyValue label="Last scan:" value={lastScanText} />
          </Box>
        )}
      </Box>
    </Card>
  );
}

function IssuesCard({
  stats,
  onViewIssues,
}: {
  stats: IntegrationWithSummary;
  onViewIssues: () => void;
}) {
  const issueCount = stats.unresolvedUserTasks;
  const issues = stats.userTasks ?? [];

  return (
    <Card p={4}>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <Flex alignItems="center" gap={2}>
          <Warning size={18} color="text.slightlyMuted" />
          <H2>Issues ({issueCount})</H2>
        </Flex>
        {issueCount > 0 && (
          <ButtonPrimaryBorder size="small" onClick={onViewIssues}>
            View Issues
          </ButtonPrimaryBorder>
        )}
      </Flex>

      <Flex flexDirection="column" gap={2}>
        {issues.map(issue => (
          <IssueItem key={issue.name} text={issue.title} />
        ))}
      </Flex>
    </Card>
  );
}

function StatusRow(props: { label: string; children: ReactNode }) {
  return (
    <Flex gap={2} flexWrap="wrap" alignItems="center">
      <Text typography="body2">{props.label}</Text>
      {props.children}
    </Flex>
  );
}

function KeyValue(props: { label: string; value: string }) {
  return (
    <Flex gap={2} flexWrap="wrap" mb={1}>
      <Text typography="body3">{props.label}</Text>
      <Text typography="body3">{props.value}</Text>
    </Flex>
  );
}

function IssueItem(props: { text: string }) {
  return (
    <Flex gap={2} alignItems="flex-start">
      <Box mt="2px">
        <CircleCross size={18} color="error.main" />
      </Box>
      <Text typography="body3">{props.text}</Text>
    </Flex>
  );
}

function getIntegrationConfigDetails(
  stats: IntegrationWithSummary
): Array<{ label: string; value: string }> {
  if (stats.subKind === IntegrationKind.AwsOidc) {
    const details = [
      { label: 'Role ARN:', value: stats.awsoidc?.roleArn || '-' },
      { label: 'Issuer S3 bucket:', value: stats.awsoidc?.issuerS3Bucket },
      { label: 'Issuer S3 prefix:', value: stats.awsoidc?.issuerS3Prefix },
      { label: 'Audience:', value: stats.awsoidc?.audience },
    ];

    return details.filter(detail => detail.value);
  }

  if (stats.subKind === IntegrationKind.AwsRa) {
    const profileSync = stats.awsra?.profileSyncConfig;
    const details = [
      { label: 'Trust anchor ARN:', value: stats.awsra?.trustAnchorARN || '-' },
      {
        label: 'Profile sync enabled:',
        value: profileSync ? (profileSync.enabled ? 'Yes' : 'No') : '-',
      },
      { label: 'Profile ARN:', value: profileSync?.profileArn },
      { label: 'Role ARN:', value: profileSync?.roleArn },
      {
        label: 'Profile name filters:',
        value: profileSync?.filters?.length
          ? profileSync.filters.join(', ')
          : undefined,
      },
    ];

    return details.filter(detail => detail.value);
  }

  return [];
}

function formatResourceCounts(stats: IntegrationWithSummary): string {
  const ec2Count = stats.awsec2?.resourcesFound || 0;
  const rdsCount = stats.awsrds?.resourcesFound || 0;
  const eksCount = stats.awseks?.resourcesFound || 0;
  const azureVmCount = stats.azurevm?.resourcesFound || 0;
  const total = ec2Count + rdsCount + eksCount + azureVmCount;

  if (total === 0) {
    return 'No resources discovered';
  }

  const parts: string[] = [];
  if (ec2Count > 0) {
    parts.push(`EC2: ${ec2Count}`);
  }
  if (rdsCount > 0) {
    parts.push(`RDS: ${rdsCount}`);
  }
  if (eksCount > 0) {
    parts.push(`EKS: ${eksCount}`);
  }
  if (azureVmCount > 0) {
    parts.push(`Azure VMs: ${azureVmCount}`);
  }

  return `${total} Discovered (${parts.join(', ')})`;
}

function formatTimeUntilNextScan(
  lastScanDate: Date | undefined,
  nowMs: number
): string {
  if (!lastScanDate) {
    return '';
  }

  const nextScanAt =
    lastScanDate.getTime() + INTEGRATION_DISCOVERY_SCAN_INTERVAL_MS;
  const remainingMs = nextScanAt - nowMs;

  if (remainingMs <= 0) {
    return '';
  }

  return `Next scan expected in ${formatDistanceStrict(0, remainingMs, {
    unit: 'minute',
    roundingMethod: 'ceil',
  })}`;
}

function getStatsRefetchIntervalMs(stats: IntegrationWithSummary | undefined) {
  const lastScanDate = stats ? latestSyncDate(stats) : undefined;

  if (!lastScanDate) {
    return OVERDUE_REFETCH_INTERVAL_MS;
  }

  const remainingMs =
    lastScanDate.getTime() +
    INTEGRATION_DISCOVERY_SCAN_INTERVAL_MS -
    Date.now();

  return remainingMs <= 0 ? OVERDUE_REFETCH_INTERVAL_MS : remainingMs;
}
