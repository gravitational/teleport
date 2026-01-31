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
import { formatDistanceToNowStrict } from 'date-fns';
import { useState, type ReactNode } from 'react';
import { useParams } from 'react-router';
import { Link as RouterLink } from 'react-router-dom';

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
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
import { pluralize } from 'shared/utils/text';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import {
  InfoGuideSwitch,
  type InfoGuideTab,
} from 'teleport/Integrations/Enroll/Cloud/Aws/EnrollAws';
import { SummaryStatusLabel } from 'teleport/Integrations/shared/StatusLabel';
import { useNoMinWidth } from 'teleport/Main';
import {
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

export function IaCIntegrationOverview() {
  const { name } = useParams<{ name: string }>();

  const {
    data: stats,
    error,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ['integrationStats', name],
    queryFn: () => integrationService.fetchIntegrationStats(name),
    enabled: !!name,
  });

  const [activeTab, setActiveTab] = useState<TabId>('overview');
  const [activeInfoGuideTab, setActiveInfoGuideTab] =
    useState<InfoGuideTab>('terraform');
  const { infoGuideConfig } = useInfoGuide();
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

  return (
    <FeatureBox maxWidth="1400px" pt={3}>
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
            currentConfig={infoGuideConfig}
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

  const hasIssues = stats.unresolvedUserTasks > 0;
  const lastScanDate = getIntegrationLastScan(stats);
  const lastScanText = formatRelativeDate(lastScanDate);
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
          <Text typography="body3">{lastScanText}</Text>
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

function getIntegrationLastScan(
  stats: IntegrationWithSummary
): Date | undefined {
  const lastScan = Math.max(
    getTimestamp(stats.awsec2?.discoverLastSync),
    getTimestamp(stats.awsrds?.discoverLastSync),
    getTimestamp(stats.awseks?.discoverLastSync),
    getTimestamp(stats.rolesAnywhereProfileSync?.syncEndTime)
  );

  return lastScan ? new Date(lastScan) : undefined;
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
  const total = ec2Count + rdsCount + eksCount;

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

  return `${total} Discovered (${parts.join(', ')})`;
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
