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

import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
  within,
} from 'design/utils/testing';
import { useToastNotifications } from 'shared/components/ToastNotification';

import { integrationService } from 'teleport/services/integrations';

import { ActivityTab } from './ActivityTab';

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
      <ActivityTab
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
      <ActivityTab
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

  await userEvent.click(
    screen.getByRole('checkbox', { name: 'Select all visible issues' })
  );
  await userEvent.click(screen.getByText('Mark as Resolved'));

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
