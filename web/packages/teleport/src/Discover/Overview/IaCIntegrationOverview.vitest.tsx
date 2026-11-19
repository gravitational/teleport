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

import type { ReactElement } from 'react';
import { MemoryRouter, useNavigate, useParams } from 'react-router';
import { beforeEach, describe, expect, test, vi } from 'vitest';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  ToastNotificationProvider,
  ToastNotifications,
} from 'shared/components/ToastNotification';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationDiscoveryRule,
  IntegrationKind,
  IntegrationWithSummary,
  integrationService,
  ResourceTypeSummary,
  UserTask,
} from 'teleport/services/integrations';

import {
  IaCIntegrationOverview,
  OverviewContent,
} from './IaCIntegrationOverview';
import type { SetupAccessRouteState } from './setupAccess';

vi.mock(import('react-router'), { spy: true });

const mockNavigate = vi.fn();

beforeEach(() => {
  vi.restoreAllMocks();
  mockNavigate.mockReset();
  vi.mocked(useNavigate).mockReturnValue(mockNavigate);
});

describe('Azure', () => {
  const stats = makeStats(IntegrationKind.AzureOidc, {
    azurevm: makeResourceSummary(),
  });

  test('does not render a "View Issues" button', () => {
    renderOverviewContent(
      <OverviewContent stats={stats} settingsPath="/settings/azure/name" />
    );

    expect(
      screen.queryByRole('button', { name: /View Issues/i })
    ).not.toBeInTheDocument();
  });

  test('clicking "View Resources" fetches fresh rules and navigates with the right query', async () => {
    vi.spyOn(integrationService, 'fetchIntegrationRules').mockResolvedValue({
      rules: [azureRule],
      nextKey: '',
    });

    renderOverviewContent(
      <OverviewContent stats={stats} settingsPath="/settings/azure/name" />
    );

    await userEvent.click(
      screen.getByRole('button', { name: /View Azure Resources/i })
    );

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));

    const query = getRouteQuery(mockNavigate.mock.calls[0][0] as string);
    expect(query).toContain(
      'labels["teleport.internal/subscription-id"] == "sub-1"'
    );
    expect(query).toContain('labels["teleport.internal/region"] == "eastus"');
    expect(query).toContain(
      'labels["teleport.internal/resource-group"] == "rg-1"'
    );
  });

  test('shows an error with a retry option if fetching rules fails, and does not navigate', async () => {
    const fetchRules = vi
      .spyOn(integrationService, 'fetchIntegrationRules')
      .mockRejectedValueOnce(new Error('some error'));

    renderOverviewContent(
      <OverviewContent stats={stats} settingsPath="/settings/azure/name" />
    );

    await userEvent.click(
      screen.getByRole('button', { name: /View Azure Resources/i })
    );

    expect(
      await screen.findByText(
        /Failed to load discovery rules required to filter Azure VM resources/i
      )
    ).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();

    fetchRules.mockResolvedValueOnce({ rules: [azureRule], nextKey: '' });
    await userEvent.click(screen.getByRole('button', { name: /Retry/i }));

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
    const query = getRouteQuery(mockNavigate.mock.calls[0][0] as string);
    expect(query).toContain('labels["teleport.internal/region"] == "eastus"');
  });

  test('clicking "Set Up Access" button for Azure integration, navigates to route without prefilled labels', async () => {
    const fetchRules = vi.spyOn(integrationService, 'fetchIntegrationRules');

    renderWholeOverview(stats);
    await clickSetUpAccess();

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
    expect(getSetUpAccessRouteState()).toEqual({
      integrationName: 'my-integration',
      integrationKind: IntegrationKind.AzureOidc,
      initialResourceKind: 'node',
      labels: undefined,
    });
    expect(fetchRules).not.toHaveBeenCalled();
  });
});

describe('AWS', () => {
  const awsAccountId = '123456789012';
  const awsRoleArn = `arn:aws:iam::${awsAccountId}:role/teleport-discovery`;
  const awsRule: IntegrationDiscoveryRule = {
    resourceType: 'ec2',
    region: 'us-west-2',
    labelMatcher: [],
    discoveryConfig: 'dc-1',
    lastSync: 0,
  };

  const stats = makeStats(IntegrationKind.AwsOidc, {
    awsoidc: { roleArn: awsRoleArn } as any,
    unresolvedUserTasks: 3,
    userTasks: [ec2Task, rdsTask, eksTask],
    awsec2: makeResourceSummary({
      resourcesEnrollmentSuccess: 3,
      resourcesFound: 4,
      unresolvedUserTasks: 1,
    }),
    awsrds: makeResourceSummary({
      resourcesEnrollmentSuccess: 5,
      resourcesFound: 6,
      unresolvedUserTasks: 1,
    }),
    awseks: makeResourceSummary({
      resourcesEnrollmentSuccess: 7,
      resourcesFound: 8,
      unresolvedUserTasks: 1,
    }),
  });

  test('only renders "View Issues" on cards with unresolved issues', () => {
    renderOverviewContent(
      <OverviewContent
        stats={{
          ...stats,
          // Not all EC2 instances are enrolled, but there are no issues to view
          // so the button is not rendered.
          awsec2: { ...stats.awsec2, unresolvedUserTasks: 0 },
        }}
        settingsPath="/settings/aws/name"
      />
    );

    expect(
      screen.getAllByRole('button', { name: /View Issues/i })
    ).toHaveLength(2);
  });

  // In OverviewContent, the cards render in order ec2, rds, eks.
  test.each([
    {
      index: 0,
      filter: 'EC2',
      shown: ec2Task.title,
      hidden: [rdsTask.title, eksTask.title],
    },
    {
      index: 1,
      filter: 'RDS',
      shown: rdsTask.title,
      hidden: [ec2Task.title, eksTask.title],
    },
    {
      index: 2,
      filter: 'EKS',
      shown: eksTask.title,
      hidden: [ec2Task.title, rdsTask.title],
    },
  ])(
    'clicking "View Issues" on card $index filters activity by $filter',
    async ({ index, filter, shown, hidden }) => {
      renderOverviewContent(
        <OverviewContent stats={stats} settingsPath="/settings/aws/name" />
      );

      const viewIssuesButtons = screen.getAllByRole('button', {
        name: /View Issues/i,
      });
      await userEvent.click(viewIssuesButtons[index]);

      expect(screen.getByText(`Resource Type: ${filter}`)).toBeVisible();
      expect(screen.getByText(shown)).toBeVisible();
      for (const title of hidden) {
        expect(screen.queryByText(title)).not.toBeInTheDocument();
      }
    }
  );

  test.each([
    {
      name: /View EC2 Resources/i,
      resourceType: 'ec2',
      resourceKind: 'node',
      expected: `labels["teleport.dev/account-id"] == "${awsAccountId}"`,
      expectedFetchCalls: [],
    },
    {
      name: /View RDS Resources/i,
      resourceType: 'rds',
      resourceKind: 'db',
      expected:
        'labels["teleport.internal/discovery-integration-name"] == "my-integration"',
      expectedFetchCalls: [],
    },
    {
      name: /View EKS Resources/i,
      resourceType: 'eks',
      resourceKind: 'kube_cluster',
      expected: `labels["account-id"] == "${awsAccountId}"`,
      expectedFetchCalls: [['my-integration', 'eks']],
    },
  ])(
    'clicking "$name" navigates with the right query',
    async ({
      name,
      resourceType,
      resourceKind,
      expected,
      expectedFetchCalls,
    }) => {
      const fetchRules = vi
        .spyOn(integrationService, 'fetchIntegrationRules')
        .mockResolvedValue({
          rules: [{ ...awsRule, resourceType }],
          nextKey: '',
        });

      renderOverviewContent(
        <OverviewContent stats={stats} settingsPath="/settings/aws/name" />
      );

      await userEvent.click(screen.getByRole('button', { name }));

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
      expect(fetchRules.mock.calls).toEqual(expectedFetchCalls);
      const route = mockNavigate.mock.calls[0][0] as string;
      expect(getRouteQuery(route)).toContain(expected);
      expect(route).toContain(`kinds=${resourceKind}`);
    }
  );

  test('clicking view ec2 resources for AWS org does not set the account ID', async () => {
    const fetchRules = vi
      .spyOn(integrationService, 'fetchIntegrationRules')
      .mockResolvedValue({ rules: [awsRule], nextKey: '' });

    renderOverviewContent(
      <OverviewContent
        stats={{
          ...stats,
          awsoidc: { roleArn: awsRoleArn, organization: {} } as any,
        }}
        settingsPath="/settings/aws/name"
      />
    );

    await userEvent.click(
      screen.getByRole('button', { name: /View EC2 Resources/i })
    );

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
    const query = getRouteQuery(mockNavigate.mock.calls[0][0] as string);
    expect(query).toContain('exists(labels["teleport.dev/account-id"])');
    expect(query).not.toContain(awsAccountId);
    expect(fetchRules).not.toHaveBeenCalled();
  });

  test('shows an error with a retry option if fetching rules fails, and does not navigate', async () => {
    const fetchRules = vi
      .spyOn(integrationService, 'fetchIntegrationRules')
      .mockRejectedValueOnce(new Error('some error'));

    renderOverviewContent(
      <OverviewContent stats={stats} settingsPath="/settings/aws/name" />
    );

    await userEvent.click(
      screen.getByRole('button', { name: /View EKS Resources/i })
    );

    expect(
      await screen.findByText(
        /Failed to load discovery rules required to filter EKS resources/i
      )
    ).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();

    // Retry re-attempts the same resource type that failed.
    fetchRules.mockResolvedValueOnce({
      rules: [{ ...awsRule, resourceType: 'eks' }],
      nextKey: '',
    });
    await userEvent.click(screen.getByRole('button', { name: /Retry/i }));

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
    expect(fetchRules).toHaveBeenLastCalledWith('my-integration', 'eks');
    expect(mockNavigate.mock.calls[0][0]).toContain('kinds=kube_cluster');
  });

  describe('Set Up Access Flow', () => {
    const legacyLabelsWithoutRegion = {
      'account-id': awsAccountId,
      'teleport.dev/cloud': 'AWS',
    };

    test('RDS and EKS are limited to their regions (as labels) from their discovery rules', async () => {
      const fetchRules = vi
        .spyOn(integrationService, 'fetchIntegrationRules')
        .mockImplementation(async (_, resourceType) => ({
          rules:
            resourceType === 'rds'
              ? [
                  { ...awsRule, resourceType, region: 'us-west-2' },
                  { ...awsRule, resourceType, region: 'us-east-1' },
                  { ...awsRule, resourceType, region: 'us-west-2' },
                ]
              : [{ ...awsRule, resourceType, region: 'eu-west-1' }],
          nextKey: '',
        }));

      renderWholeOverview(stats);
      await clickSetUpAccess();

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
      expect(fetchRules).toHaveBeenCalledTimes(2);
      expect(fetchRules).toHaveBeenCalledWith('my-integration', 'rds');
      expect(fetchRules).toHaveBeenCalledWith('my-integration', 'eks');
      expect(getSetUpAccessRouteState()).toEqual({
        integrationName: 'my-integration',
        integrationKind: IntegrationKind.AwsOidc,
        initialResourceKind: 'node',
        labels: {
          node_labels: { 'teleport.dev/account-id': awsAccountId },
          db_labels: {
            ...legacyLabelsWithoutRegion,
            region: ['us-west-2', 'us-east-1'],
          },
          kubernetes_labels: {
            ...legacyLabelsWithoutRegion,
            region: ['eu-west-1'],
          },
        },
      });
    });

    test.each(['*', 'aws-global'])(
      'does not limit labels by region if a rule has "%s" (wildcard)',
      async region => {
        vi.spyOn(
          integrationService,
          'fetchIntegrationRules'
        ).mockImplementation(async (_, resourceType) => ({
          rules: [
            { ...awsRule, resourceType, region: 'us-west-2' },
            { ...awsRule, resourceType, region },
          ],
          nextKey: '',
        }));

        renderWholeOverview(stats);
        await clickSetUpAccess();

        await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
        const { labels } = getSetUpAccessRouteState();
        expect(labels.db_labels).toEqual(legacyLabelsWithoutRegion);
        expect(labels.kubernetes_labels).toEqual(legacyLabelsWithoutRegion);
      }
    );

    test('rendering warning dialog and navigates without region labels if fetching rules fails', async () => {
      vi.spyOn(integrationService, 'fetchIntegrationRules').mockRejectedValue(
        new Error('some error')
      );

      renderWholeOverview(
        makeStats(IntegrationKind.AwsOidc, {
          awsoidc: { roleArn: awsRoleArn } as any,
          awsrds: makeResourceSummary(),
        })
      );
      await clickSetUpAccess();

      expect(
        await screen.findByText('Unable to filter by region')
      ).toBeInTheDocument();
      expect(
        screen.getByText(/^Database access isn't limited by region/)
      ).toBeInTheDocument();
      await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
      expect(getSetUpAccessRouteState()).toEqual({
        integrationName: 'my-integration',
        integrationKind: IntegrationKind.AwsOidc,
        initialResourceKind: 'db',
        labels: { db_labels: legacyLabelsWithoutRegion },
      });
    });

    test('not fetching rules if only EC2 is enrolled', async () => {
      const fetchRules = vi.spyOn(integrationService, 'fetchIntegrationRules');

      renderWholeOverview(
        makeStats(IntegrationKind.AwsOidc, {
          awsoidc: { roleArn: awsRoleArn } as any,
          awsec2: makeResourceSummary(),
        })
      );
      await clickSetUpAccess();

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
      expect(getSetUpAccessRouteState()).toEqual({
        integrationName: 'my-integration',
        integrationKind: IntegrationKind.AwsOidc,
        initialResourceKind: 'node',
        labels: { node_labels: { 'teleport.dev/account-id': awsAccountId } },
      });
      expect(fetchRules).not.toHaveBeenCalled();
    });

    test('does not prefill labels for an AWS organization', async () => {
      const fetchRules = vi.spyOn(integrationService, 'fetchIntegrationRules');

      renderWholeOverview({
        ...stats,
        awsoidc: { roleArn: awsRoleArn, organization: {} } as any,
      });
      await clickSetUpAccess();

      await waitFor(() => expect(mockNavigate).toHaveBeenCalledTimes(1));
      expect(getSetUpAccessRouteState()).toEqual({
        integrationName: 'my-integration',
        integrationKind: IntegrationKind.AwsOidc,
        initialResourceKind: 'node',
        labels: undefined,
      });
      expect(fetchRules).not.toHaveBeenCalled();
    });

    test('button is disabled if nothing is enrolled', async () => {
      renderWholeOverview(
        makeStats(IntegrationKind.AwsOidc, {
          awsoidc: { roleArn: awsRoleArn } as any,
        })
      );

      expect(
        await screen.findByRole('button', { name: 'Set Up Access' })
      ).toBeDisabled();
    });
  });
});

function renderOverviewContent(ui: ReactElement) {
  return render(<ToastNotificationProvider>{ui}</ToastNotificationProvider>);
}

function renderWholeOverview(stats: IntegrationWithSummary) {
  const ctx = createTeleportContext();
  vi.mocked(useParams).mockReturnValue({
    name: stats.name,
    type: stats.subKind,
  });
  vi.spyOn(integrationService, 'fetchIntegrationStats').mockResolvedValue(
    stats
  );

  return renderOverviewContent(
    <ContextProvider ctx={ctx}>
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <IaCIntegrationOverview />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
        <ToastNotifications />
      </MemoryRouter>
    </ContextProvider>
  );
}

async function clickSetUpAccess() {
  await userEvent.click(
    await screen.findByRole('button', { name: 'Set Up Access' })
  );
}

function getSetUpAccessRouteState(): SetupAccessRouteState {
  const [route, options] = mockNavigate.mock.calls[0];
  expect(route).toBe(cfg.routes.accessListNew);
  return options.state;
}

function getRouteQuery(route: string): string {
  return new URLSearchParams(route.split('?')[1]).get('query') ?? '';
}

function makeResourceSummary(
  overrides: Partial<ResourceTypeSummary> = {}
): ResourceTypeSummary {
  return {
    rulesCount: 1,
    resourcesFound: 5,
    resourcesEnrollmentFailed: 0,
    resourcesEnrollmentSuccess: 5,
    discoverLastSync: 0,
    syncEnd: undefined,
    syncStart: undefined,
    pollIntervalSeconds: undefined,
    unresolvedUserTasks: undefined,
    ecsDatabaseServiceCount: undefined,
    ...overrides,
  };
}

const emptyResourceSummary = makeResourceSummary({
  rulesCount: 0,
  resourcesFound: 0,
  resourcesEnrollmentSuccess: 0,
});

function makeStats(
  subKind: string,
  overrides: Partial<IntegrationWithSummary> = {}
): IntegrationWithSummary {
  return {
    name: 'my-integration',
    subKind,
    unresolvedUserTasks: 0,
    userTasks: [],
    awsra: {} as any,
    awsoidc: {} as any,
    awsec2: emptyResourceSummary,
    awsrds: emptyResourceSummary,
    awseks: emptyResourceSummary,
    azurevm: emptyResourceSummary,
    rolesAnywhereProfileSync: {} as any,
    ...overrides,
  };
}

const azureRule: IntegrationDiscoveryRule = {
  resourceType: 'vm',
  region: 'eastus',
  labelMatcher: [],
  subscriptions: ['sub-1'],
  resourceGroups: ['rg-1'],
  discoveryConfig: 'dc-1',
  lastSync: 0,
};

function makeUserTask(
  taskType: string,
  title: string,
  overrides: Partial<UserTask> = {}
): UserTask {
  return {
    name: `task-${taskType}`,
    taskType,
    state: 'OPEN',
    issueType: `${taskType}-issue`,
    title,
    integration: 'my-integration',
    lastStateChange: '2025-10-24T12:30:00Z',
    ...overrides,
  };
}

const ec2Task = makeUserTask('discover-ec2', 'EC2 failure');
const rdsTask = makeUserTask('discover-rds', 'RDS failure');
const eksTask = makeUserTask('discover-eks', 'EKS failure');
