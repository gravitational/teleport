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

import { QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';

import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
  within,
} from 'design/utils/testing';
import { useToastNotifications } from 'shared/components/ToastNotification';

import {
  integrationService,
  type UserTask,
} from 'teleport/services/integrations';

import { ActivityTab, type TaskResourceTypeFilter } from './ActivityTab';

jest.mock(
  '../../../../shared/components/ToastNotification/ToastNotificationContext',
  () => {
    const originalContext = jest.requireActual(
      '../../../../shared/components/ToastNotification/ToastNotificationContext'
    );
    return {
      ...originalContext,
      useToastNotifications: jest.fn(),
    };
  }
);

afterEach(() => {
  jest.clearAllMocks();
});

test('hides and ignores the resource type filter for Azure', () => {
  (useToastNotifications as jest.Mock).mockReturnValue({
    add: jest.fn(),
  });

  render(
    <QueryClientProvider client={testQueryClient}>
      <ControlledActivityTab
        initialFilter="RDS"
        showResourceTypeFilter={false}
        stats={{
          name: 'azure-integration',
          userTasks: [
            {
              name: 'task-azure',
              taskType: 'discover-azure-vm',
              state: 'OPEN',
              issueType: 'azure-vm-enrollment-error',
              title: 'Azure VM failure',
              integration: 'azure-integration',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
          ],
        }}
      />
    </QueryClientProvider>
  );

  expect(screen.queryByText(/Resource Type:/)).not.toBeInTheDocument();
  expect(screen.getByText('Azure VM failure')).toBeVisible();
});

test('filters AWS issues by resource type', () => {
  (useToastNotifications as jest.Mock).mockReturnValue({
    add: jest.fn(),
  });

  render(
    <QueryClientProvider client={testQueryClient}>
      <ControlledActivityTab
        initialFilter="RDS"
        stats={{
          name: 'aws-integration',
          userTasks: [
            {
              name: 'task-ec2',
              taskType: 'discover-ec2',
              state: 'OPEN',
              issueType: 'ec2-ssm-invocation-failure',
              title: 'EC2 failure',
              integration: 'aws-integration',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
            {
              name: 'task-rds',
              taskType: 'discover-rds',
              state: 'OPEN',
              issueType: 'rds-agent-not-connecting',
              title: 'RDS failure',
              integration: 'aws-integration',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
          ],
        }}
      />
    </QueryClientProvider>
  );

  expect(screen.getByText('Resource Type: RDS')).toBeVisible();
  expect(screen.getByText('RDS failure')).toBeVisible();
  expect(screen.queryByText('EC2 failure')).not.toBeInTheDocument();
});

test('opens issue drawer and resolves a task', async () => {
  (useToastNotifications as jest.Mock).mockReturnValue({
    add: jest.fn(),
  });

  jest.spyOn(integrationService, 'resolveUserTask').mockResolvedValue({
    name: 'task-1',
    taskType: 'discover-ec2',
    state: 'RESOLVED',
    issueType: 'ec2-ssm-invocation-failure',
    title: 'EC2 failure',
    integration: 'aws-integration-org-foo',
    lastStateChange: '2025-10-24T12:30:00Z',
  });

  jest.spyOn(integrationService, 'fetchUserTask').mockResolvedValueOnce({
    name: 'task-1',
    taskType: 'discover-ec2',
    state: 'OPEN',
    issueType: 'ec2-ssm-invocation-failure',
    title: 'EC2 failure',
    integration: 'aws-integration-org-foo',
    lastStateChange: '2025-10-24T12:30:00Z',
    description: 'Details',
    discoverEc2: {
      instances: {},
      account_id: '123456789012',
      region: 'us-east-1',
      ssm_document: '',
      installer_script: '',
    },
    discoverEks: {
      clusters: {},
      account_id: '',
      region: '',
      app_auto_discover: false,
    },
    discoverRds: { databases: {}, account_id: '', region: '' },
  });

  render(
    <QueryClientProvider client={testQueryClient}>
      <ControlledActivityTab
        stats={{
          name: 'aws-integration-org-foo',
          userTasks: [
            {
              name: 'task-1',
              taskType: 'discover-ec2',
              state: 'OPEN',
              issueType: 'ec2-ssm-invocation-failure',
              title: 'EC2 failure',
              integration: 'aws-integration-org-foo',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
          ],
        }}
      />
    </QueryClientProvider>
  );

  await userEvent.click(screen.getByText('Details'));
  await screen.findByRole('heading', { name: 'EC2 failure' });

  const resolveButtons = screen.getAllByText('Mark as Resolved');
  await userEvent.click(resolveButtons[resolveButtons.length - 1]);

  await waitFor(() =>
    expect(integrationService.resolveUserTask).toHaveBeenCalledWith('task-1')
  );
});

test('shows Azure integration identity in the issue drawer', async () => {
  (useToastNotifications as jest.Mock).mockReturnValue({
    add: jest.fn(),
  });

  jest.spyOn(integrationService, 'fetchUserTask').mockResolvedValueOnce({
    name: 'task-azure',
    taskType: 'discover-azure-vm',
    state: 'OPEN',
    issueType: 'azure-vm-subscription-list-denied',
    title: 'Cannot access Azure subscriptions',
    integration: 'azure-integration',
    lastStateChange: '2025-10-24T12:30:00Z',
    description: 'Details',
    discoverAzureVm: {
      instances: {},
      subscription_id: '',
      resource_group: '',
      region: '',
      tenant_id: 'azure-tenant-id',
      client_id: 'azure-client-id',
    },
  });

  render(
    <QueryClientProvider client={testQueryClient}>
      <ControlledActivityTab
        stats={{
          name: 'azure-integration',
          userTasks: [
            {
              name: 'task-azure',
              taskType: 'discover-azure-vm',
              state: 'OPEN',
              issueType: 'azure-vm-subscription-list-denied',
              title: 'Cannot access Azure subscriptions',
              integration: 'azure-integration',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
          ],
        }}
      />
    </QueryClientProvider>
  );

  await userEvent.click(screen.getByText('Details'));
  await screen.findByRole('heading', {
    name: 'Cannot access Azure subscriptions',
  });

  expect(screen.getByText('Tenant ID:')).toBeVisible();
  expect(screen.getByText('azure-tenant-id')).toBeVisible();
  expect(screen.getByText('Client ID:')).toBeVisible();
  expect(screen.getByText('azure-client-id')).toBeVisible();
});

test('keeps failed tasks selected when bulk resolve partially fails', async () => {
  const addToast = jest.fn();
  (useToastNotifications as jest.Mock).mockReturnValue({
    add: addToast,
  });

  jest
    .spyOn(integrationService, 'resolveUserTask')
    .mockResolvedValueOnce({
      name: 'task-1',
      taskType: 'discover-ec2',
      state: 'RESOLVED',
      issueType: 'ec2-ssm-invocation-failure',
      title: 'EC2 failure',
      integration: 'aws-integration-org-foo',
      lastStateChange: '2025-10-24T12:30:00Z',
    })
    .mockRejectedValueOnce(new Error('bad'));

  render(
    <QueryClientProvider client={testQueryClient}>
      <ControlledActivityTab
        stats={{
          name: 'aws-integration-org-foo',
          userTasks: [
            {
              name: 'task-1',
              taskType: 'discover-ec2',
              state: 'OPEN',
              issueType: 'ec2-ssm-invocation-failure',
              title: 'EC2 failure',
              integration: 'aws-integration-org-foo',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
            {
              name: 'task-2',
              taskType: 'discover-eks',
              state: 'OPEN',
              issueType: 'eks-join-failure',
              title: 'EKS failure',
              integration: 'aws-integration-org-foo',
              lastStateChange: '2025-10-23T10:00:00Z',
            },
          ],
        }}
      />
    </QueryClientProvider>
  );

  expect(screen.queryByText('Mark as Resolved')).not.toBeInTheDocument();

  // fireEvent avoids focusing the checkbox; its blur handler constructs a
  // PointerEvent, which jsdom doesn't implement.
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Select all visible issues' })
  );
  await userEvent.click(await screen.findByText('Mark as Resolved'));

  const dialog = await screen.findByTestId('dialogbox');
  await userEvent.click(
    within(dialog).getByRole('button', { name: 'Mark as Resolved' })
  );

  await waitFor(() => {
    expect(integrationService.resolveUserTask).toHaveBeenCalledTimes(2);
  });

  expect(
    await screen.findByText('1 issue failed to resolve: bad')
  ).toBeInTheDocument();
  expect(screen.getByText('1 selected')).toBeInTheDocument();
});

test('clicking on "mark as resolved" button only resolves tasks that is visibly selected (hidden ones from filters are ignored)', async () => {
  (useToastNotifications as jest.Mock).mockReturnValue({ add: jest.fn() });

  jest.spyOn(integrationService, 'resolveUserTask').mockResolvedValue({
    name: 'task-ec2',
    taskType: 'discover-ec2',
    state: 'RESOLVED',
    issueType: 'ec2-ssm-invocation-failure',
    title: 'EC2 failure',
    integration: 'aws-integration',
    lastStateChange: '2025-10-24T12:30:00Z',
  });

  render(
    <QueryClientProvider client={testQueryClient}>
      <ControlledActivityTab
        stats={{
          name: 'aws-integration',
          userTasks: [
            {
              name: 'task-ec2',
              taskType: 'discover-ec2',
              state: 'OPEN',
              issueType: 'ec2-ssm-invocation-failure',
              title: 'EC2 failure',
              integration: 'aws-integration',
              lastStateChange: '2025-10-24T12:30:00Z',
            },
            {
              name: 'task-rds',
              taskType: 'discover-rds',
              state: 'OPEN',
              issueType: 'rds-agent-not-connecting',
              title: 'RDS failure',
              integration: 'aws-integration',
              lastStateChange: '2025-10-23T12:30:00Z',
            },
            {
              name: 'task-eks',
              taskType: 'discover-eks',
              state: 'OPEN',
              issueType: 'eks-join-failure',
              title: 'EKS failure',
              integration: 'aws-integration',
              lastStateChange: '2025-10-22T12:30:00Z',
            },
          ],
        }}
      />
    </QueryClientProvider>
  );

  // Checkbox all issues.
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Select issue EC2 failure' })
  );
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Select issue RDS failure' })
  );
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Select issue EKS failure' })
  );

  // Filter only EC2 so the RDS and EKS tasks are hidden.
  await userEvent.click(screen.getByText('Resource Type: All'));
  await userEvent.click(screen.getByText('EC2'));

  await userEvent.click(screen.getByText('Mark as Resolved'));
  const dialog = await screen.findByTestId('dialogbox');
  await userEvent.click(
    within(dialog).getByRole('button', { name: 'Mark as Resolved' })
  );

  await waitFor(() =>
    expect(integrationService.resolveUserTask).toHaveBeenCalledTimes(1)
  );

  // Only the issue visibly rendered is resolved.
  expect(integrationService.resolveUserTask).toHaveBeenCalledWith('task-ec2');
});

function ControlledActivityTab({
  stats,
  initialFilter = 'All',
  showResourceTypeFilter,
}: {
  stats: { name: string; userTasks?: UserTask[] };
  initialFilter?: TaskResourceTypeFilter;
  showResourceTypeFilter?: boolean;
}) {
  const [targetedResourceTypeFilter, setTargetedResourceTypeFilter] =
    useState(initialFilter);

  return (
    <ActivityTab
      stats={stats}
      targetedResourceTypeFilter={targetedResourceTypeFilter}
      onResourceTypeFilterChange={setTargetedResourceTypeFilter}
      showResourceTypeFilter={showResourceTypeFilter}
    />
  );
}
