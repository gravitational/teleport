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

import { Button, ButtonBorder } from '@gravitational/design-system';
import { useMutation, useQuery } from '@tanstack/react-query';
import { formatDistanceStrict, formatDistanceToNowStrict } from 'date-fns';
import { useState, useEffect } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router';
import styled, { keyframes } from 'styled-components';

import { Card, Flex, H2, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import ButtonIcon from 'design/ButtonIcon';
import { displayDateTime } from 'design/datetime';
import {
  ArrowForward,
  ArrowLeft,
  ArrowRight,
  CircleCheck,
  Clock,
  SyncAlt,
  Warning,
} from 'design/Icon';
import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import { Status as StatusBadge } from 'design/Status';
import { HoverTooltip } from 'design/Tooltip';
import { useToastNotifications } from 'shared/components/ToastNotification';
import { pluralize } from 'shared/utils/text';

import { useTeleport } from 'teleport';
import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { shouldHideInaccessibleFeatures } from 'teleport/features';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { useNoMinWidth } from 'teleport/Main';
import {
  AzureResource,
  IntegrationKind,
  integrationService,
  IntegrationWithSummary,
  ResourceTypeSummary,
} from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';

import { ActivityTab, TaskResourceTypeFilter } from './ActivityTab';
import {
  getListResourceRoute,
  getResourceKindFromResource,
  ListResourceRouteParams,
  requiresDiscoveryRules,
} from './resources';
import {
  getAwsResourceMatchingLabels,
  getSetupAccessEnrollment,
  AwsResourceLabels,
  SetupAccessRouteState,
} from './setupAccess';

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
  const navigate = useNavigate();
  const toastNotification = useToastNotifications();
  const ctx = useTeleport();
  const canCreateAccessList = ctx.getFeatureFlags().addAccessList;
  const showSetUpAccessBtn =
    !cfg.isDashboard &&
    (canCreateAccessList || !shouldHideInaccessibleFeatures(cfg));

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

  const isAwsIntegration = stats?.subKind === IntegrationKind.AwsOidc;
  const isAzureIntegration = stats?.subKind === IntegrationKind.AzureOidc;
  const integrationName = stats?.name ?? name;
  const { ec2Enrolled, rdsEnrolled, eksEnrolled, startAccessListWizardWith } =
    getSetupAccessEnrollment(stats);

  function navigateToSetUpAccess(labels?: AwsResourceLabels) {
    let integrationKind: IntegrationKind;
    if (isAwsIntegration) {
      integrationKind = IntegrationKind.AwsOidc;
    } else if (isAzureIntegration) {
      integrationKind = IntegrationKind.AzureOidc;
    }
    const state: SetupAccessRouteState = {
      integrationName: stats.name,
      integrationKind,
      initialResourceKind: startAccessListWizardWith,
      labels,
    };
    navigate(cfg.routes.accessListNew, { state });
  }

  const setUpAccess = useMutation({
    mutationFn: async (accountId: string): Promise<AwsResourceLabels> => {
      const fetchRegions = async (enabled: boolean, resource: AwsResource) => {
        if (!enabled) {
          return undefined;
        }
        const { rules } = await integrationService.fetchIntegrationRules(
          integrationName,
          resource
        );

        const regions = rules.map(rule => rule.region).filter(Boolean);
        if (regions.some(region => region === '*' || region === 'aws-global')) {
          return ['*'];
        }
        return [...new Set(regions)];
      };

      // Ec2 is not required since there's nothing in the fetched resource
      // we will need for it.
      const [rdsRegions, eksRegions] = await Promise.all([
        fetchRegions(rdsEnrolled, AwsResource.rds),
        fetchRegions(eksEnrolled, AwsResource.eks),
      ]);

      return getAwsResourceMatchingLabels({
        accountId,
        ec2Enrolled,
        rdsRegions,
        eksRegions,
      });
    },
    onSuccess: navigateToSetUpAccess,
    // A failed fetch shouldn't stop the flow since adding filters FOR the user
    // is nice to have. A toast notification is rendered so users are aware
    // that region filters were not added at this time.
    onError: (_, accountId) => {
      let resources = '';
      if (rdsEnrolled && eksEnrolled) {
        resources = 'Database and Kubernetes';
      } else if (rdsEnrolled) {
        resources = 'Database';
      } else if (eksEnrolled) {
        resources = 'Kubernetes';
      }
      toastNotification.add({
        severity: 'warn',
        content: {
          title: 'Unable to filter by region',
          description: `${resources} access isn't limited by region. Add region labels when defining access if needed.`,
          isAutoRemovable: false,
        },
      });
      navigateToSetUpAccess(
        getAwsResourceMatchingLabels({
          accountId,
          ec2Enrolled,
          rdsRegions: rdsEnrolled ? [] : undefined,
          eksRegions: eksEnrolled ? [] : undefined,
        })
      );
    },
  });

  // handleSetUpAccess navigates to the access list wizard with labels
  // prefilled to best attempt target the resources this integration
  // discovered (e.g. match resources by aws account id and region).
  //
  // RDS and EKS labels are limited to the regions in their discovery rules,
  // so those rules are fetched first. No fetch is needed when:
  // - The integration is Azure (subscriptions and resource groups can be
  //   wildcards), an AWS organization (resources come from child accounts).
  //   No labels are prebuilt for the user.
  // - If EC2 is enrolled: nodes are matched by account ID alone (which we already have)
  //   b/c region labels are not defined in these resources.
  function handleSetUpAccess() {
    const { awsAccountId: accountId } = splitAwsIamArn(
      stats.awsoidc?.roleArn ?? ''
    );

    if (isAzureIntegration || stats.awsoidc?.organization || !accountId) {
      navigateToSetUpAccess();
      return;
    }

    if (!rdsEnrolled && !eksEnrolled) {
      navigateToSetUpAccess(
        getAwsResourceMatchingLabels({ accountId, ec2Enrolled })
      );
      return;
    }

    setUpAccess.mutate(accountId);
  }

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
            <Flex alignItems="center" gap={2} flexWrap="wrap">
              <Text bold fontSize={6}>
                {stats.name}
              </Text>
              <IntegrationScanStatus stats={stats} />
            </Flex>
            {roleArn && (
              <Text typography="body3" color="text.slightlyMuted">
                {roleArn}
              </Text>
            )}
          </Flex>
        </Flex>
        <Flex gap={2}>
          <ButtonBorder asChild>
            <RouterLink to={settingsPath}>Edit Configuration</RouterLink>
          </ButtonBorder>
          {showSetUpAccessBtn && (
            <HoverTooltip
              disabled={!!startAccessListWizardWith}
              placement="bottom"
              tipContent="No resources are enrolled yet."
            >
              <span>
                <Button
                  fill="border"
                  intent="primary"
                  disabled={setUpAccess.isPending || !startAccessListWizardWith}
                  onClick={handleSetUpAccess}
                >
                  Set Up Access
                </Button>
              </span>
            </HoverTooltip>
          )}
        </Flex>
      </Flex>

      <OverviewContent stats={stats} settingsPath={settingsPath} />
    </FeatureBox>
  );
}

export function OverviewContent({
  stats,
  settingsPath,
}: {
  stats: IntegrationWithSummary;
  settingsPath: string;
}) {
  const navigate = useNavigate();

  const [resourceTypeFilter, setResourceTypeFilter] =
    useState<TaskResourceTypeFilter>('All');

  const hasIssues = stats.unresolvedUserTasks > 0;
  const isAzure = stats.subKind === IntegrationKind.AzureOidc;
  const isAwsOrganization = Boolean(stats.awsoidc?.organization);

  const viewResources = useMutation({
    mutationFn: async (resourceType: AwsResource | AzureResource) => {
      if (!requiresDiscoveryRules(resourceType, isAwsOrganization)) {
        // Some resource types does not require discovery rules to
        // build the list resource route.
        return [];
      }
      const { rules } = await integrationService.fetchIntegrationRules(
        stats.name,
        resourceType
      );
      return rules;
    },
    onSuccess: (rules, resourceType) => {
      let params: ListResourceRouteParams = {
        resource: AzureResource.vm,
        rules,
      };
      if (resourceType !== AzureResource.vm) {
        const { awsAccountId: accountId } = splitAwsIamArn(
          stats.awsoidc?.roleArn ?? ''
        );
        params = {
          resource: resourceType,
          integrationName: stats.name,
          isOrganization: isAwsOrganization,
          accountId,
          rules,
        };
      }
      navigate(getListResourceRoute(params));
    },
  });

  return (
    <Flex flexDirection="column" gap={4}>
      {viewResources.isError && (
        <Danger
          mb={4}
          dismissible
          secondaryAction={{
            content: 'Retry',
            onClick: () => viewResources.mutate(viewResources.variables),
          }}
        >
          Failed to load discovery rules required to filter{' '}
          {getResourceKindFromResource(viewResources.variables)} resources:{' '}
          {viewResources.error.message}
        </Danger>
      )}
      <CardsContainer>
        {isAzure ? (
          <ResourceTypeCard
            resourceType="azurevm"
            summary={stats.azurevm}
            settingsPath={settingsPath}
            onViewResources={() => viewResources.mutate(AzureResource.vm)}
            viewResourceBtnDisabled={viewResources.isPending}
          />
        ) : (
          <>
            <ResourceTypeCard
              resourceType="ec2"
              summary={stats.awsec2}
              settingsPath={settingsPath}
              onViewResources={() => viewResources.mutate(AwsResource.ec2)}
              onViewIssues={() => setResourceTypeFilter('EC2')}
              viewResourceBtnDisabled={viewResources.isPending}
            />
            <ResourceTypeCard
              resourceType="rds"
              summary={stats.awsrds}
              settingsPath={settingsPath}
              onViewResources={() => viewResources.mutate(AwsResource.rds)}
              onViewIssues={() => setResourceTypeFilter('RDS')}
              viewResourceBtnDisabled={viewResources.isPending}
            />
            <ResourceTypeCard
              resourceType="eks"
              summary={stats.awseks}
              settingsPath={settingsPath}
              onViewResources={() => viewResources.mutate(AwsResource.eks)}
              onViewIssues={() => setResourceTypeFilter('EKS')}
              viewResourceBtnDisabled={viewResources.isPending}
            />
          </>
        )}
      </CardsContainer>
      <IntegrationHealthCard
        stats={stats}
        hasIssues={hasIssues}
        resourceTypeFilter={resourceTypeFilter}
        onResourceTypeFilterChange={setResourceTypeFilter}
      />
    </Flex>
  );
}

function IntegrationHealthCard({
  stats,
  hasIssues,
  resourceTypeFilter,
  onResourceTypeFilterChange,
}: {
  stats: IntegrationWithSummary;
  hasIssues: boolean;
  resourceTypeFilter: TaskResourceTypeFilter;
  onResourceTypeFilterChange: (filter: TaskResourceTypeFilter) => void;
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

      {hasIssues && (
        <ActivityTab
          stats={stats}
          targetedResourceTypeFilter={resourceTypeFilter}
          onResourceTypeFilterChange={onResourceTypeFilterChange}
          showResourceTypeFilter={stats.subKind === IntegrationKind.AwsOidc}
        />
      )}
    </Card>
  );
}

function IntegrationScanStatus({ stats }: { stats: IntegrationWithSummary }) {
  const [nowMs, setNowMs] = useState(Date.now);

  useEffect(() => {
    const interval = window.setInterval(() => {
      setNowMs(Date.now());
    }, 30_000);

    return () => {
      window.clearInterval(interval);
    };
  }, []);

  const scanState = getScanState(stats);

  if (scanState.state === 'waiting') {
    return null;
  }

  if (
    scanState.state === 'scanning' ||
    scanState.nextScanAt.getTime() <= nowMs
  ) {
    return (
      <StatusBadge kind="info" variant="filled-tonal" icon={SpinningSyncIcon}>
        Rescanning now...
      </StatusBadge>
    );
  }

  const remainingMs = scanState.nextScanAt.getTime() - nowMs;

  return (
    <Flex alignItems="center" gap={2} color="text.slightlyMuted">
      <Flex alignItems="center" gap={1}>
        <Clock size="small" />
        <Text typography="body3">
          Next scan in{' '}
          {formatDistanceStrict(0, remainingMs, {
            unit: 'minute',
            roundingMethod: 'ceil',
          })}
        </Text>
      </Flex>
      <Text typography="body3">
        · Last scan {formatRelativeDate(scanState.lastScan)}
      </Text>
    </Flex>
  );
}

type ResourceType = 'ec2' | 'rds' | 'eks' | 'azurevm';

const RESOURCE_TYPE_CONFIG: Record<
  ResourceType,
  {
    icon: ResourceIconName;
    label: string;
    instanceLabel: string;
    resourceType: string;
  }
> = {
  ec2: {
    icon: 'awsec2',
    label: 'EC2 Discovered Instances',
    instanceLabel: 'instances',
    resourceType: 'EC2',
  },
  rds: {
    icon: 'awsrds',
    label: 'RDS Discovered Instances',
    instanceLabel: 'instances',
    resourceType: 'RDS',
  },
  azurevm: {
    icon: 'azure',
    label: 'Azure Discovered VMs',
    instanceLabel: 'VMs',
    resourceType: 'Azure',
  },
  eks: {
    icon: 'eks',
    label: 'EKS Discovered Clusters',
    instanceLabel: 'clusters',
    resourceType: 'EKS',
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
  settingsPath,
  onViewIssues,
  onViewResources,
  viewResourceBtnDisabled = false,
}: {
  resourceType: ResourceType;
  summary: ResourceTypeSummary | undefined;
  settingsPath: string;
  onViewIssues?(): void;
  onViewResources(): void;
  viewResourceBtnDisabled?: boolean;
}) {
  const navigate = useNavigate();
  const config = RESOURCE_TYPE_CONFIG[resourceType];
  const isConfigured = summary && summary.rulesCount > 0;

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
          flexDirection={'column'}
          gap={1}
        >
          <Text typography="body3" color="text.muted">
            {resourceType.toUpperCase()} not configured.
          </Text>
          <ButtonBorder
            color="text.main"
            onClick={() => navigate(settingsPath)}
          >
            <Flex gap={2}>
              Configure {resourceType.toUpperCase()} <ArrowRight />
            </Flex>
          </ButtonBorder>
        </Flex>
      </ResourceCard>
    );
  }

  const discovered = summary.resourcesFound || 0;
  const enrolled = summary.resourcesEnrollmentSuccess || 0;
  const hasIssues = (summary.unresolvedUserTasks ?? 0) > 0;

  return (
    <ResourceCard>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <Flex alignItems="center" gap={2}>
          <ResourceIcon name={config.icon} size={24} />
          <Text bold typography="body1">
            {config.label}
          </Text>
        </Flex>
      </Flex>

      <CardDivider />

      <Text fontSize={1} bold mb={1} mt={3}>
        What's been found
      </Text>
      <Flex gap={1} alignItems={'center'}>
        {discovered > 0 ? (
          <CircleCheck size="small" color="success.main" />
        ) : (
          <Warning size="small" color="warning.main" />
        )}{' '}
        <Text typography="body3">
          Discovered: <span css={{ fontWeight: 600 }}>{discovered}</span>
        </Text>
      </Flex>

      <Flex
        alignItems={'center'}
        mb={3}
        gap={2}
        justifyContent={'space-between'}
      >
        <Flex gap={1}>
          {enrolled > 0 && enrolled === discovered ? (
            <CircleCheck size="small" color="success.main" />
          ) : (
            <Warning size="small" color="warning.main" />
          )}
          <Text typography="body3">
            Enrolled:{' '}
            <span css={{ fontWeight: 600 }}>
              {enrolled} of {discovered}
            </span>
          </Text>
        </Flex>
        {resourceType !== 'azurevm' && hasIssues && (
          <Button
            fill="minimal"
            size="sm"
            px={1}
            py={0}
            onClick={() => onViewIssues()}
          >
            <Flex gap={1}>
              <Text>View Issues</Text>
              <ArrowForward />
            </Flex>
          </Button>
        )}
      </Flex>

      <CardDivider />
      <Flex mt={3}></Flex>
      <Flex justifyContent={'flex-end'} mt="auto">
        <HoverTooltip
          tipContent={
            enrolled === 0
              ? `No ${config.resourceType} Resources are enrolled`
              : undefined
          }
        >
          <Button
            onClick={onViewResources}
            disabled={viewResourceBtnDisabled || enrolled === 0}
            fill="minimal"
            size="sm"
            px={1}
            py={0}
          >
            <Flex gap={1}>
              <Text>View {config.resourceType} Resources</Text>
              <ArrowForward />
            </Flex>
          </Button>
        </HoverTooltip>
      </Flex>
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
  border: 1px solid ${props => props.theme.colors.interactive.tonal.neutral[2]};
`;

const CardDivider = styled.hr`
  width: 100%;
  height: 1px;
  border: none;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  margin: 0;
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
