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

import { within } from '@testing-library/react';

import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { Task } from 'teleport/Integrations/status/AwsOidc/Tasks/Task';
import { integrationService } from 'teleport/services/integrations';
import TeleportContext from 'teleport/teleportContext';

test('renders ec2 impacts', async () => {
  const ctx = new TeleportContext();
  jest.spyOn(integrationService, 'fetchUserTask').mockResolvedValue({
    name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
    taskType: 'discover-ec2',
    state: 'OPEN',
    integration: '',
    lastStateChange: '2025-02-11T20:32:19.482607921Z',
    issueType: 'ec2-ssm-invocation-failure',
    title: 'ec2 ssm invocation failure',
    description:
      'Teleport failed to access the SSM Agent to auto enroll the instance.\nSome instances failed to communicate with the AWS Systems Manager service to execute the install script.\n\nUsually this happens when:\n\n**Missing policies**\n\nThe IAM Role used by the integration might be missing some required permissions.\nEnsure the following actions are allowed in the IAM Role used by the integration:\n- `ec2:DescribeInstances`\n- `ssm:DescribeInstanceInformation`\n- `ssm:GetCommandInvocation`\n- `ssm:ListCommandInvocations`\n- `ssm:SendCommand`\n\n**SSM Document is invalid**\n\nTeleport uses an SSM Document to run an installation script.\nIf the document is changed or removed, it might no longer work.',
    discoverEks: undefined,
    discoverRds: undefined,
    discoverEc2: {
      region: 'us-east-2',
      accountId: undefined,
      ssmDocument: undefined,
      installerScript: undefined,
      instances: {
        'i-016e32a5882f5ee81': {
          instance_id: 'i-016e32a5882f5ee81',
          resourceUrl: '',
          name: undefined,
          invocationUrl: undefined,
          discoveryConfig: undefined,
          discoveryGroup: undefined,
          syncTime: undefined,
        },
        'i-065818031835365cc': {
          instance_id: 'i-065818031835365cc',
          resourceUrl: '',
          name: 'aws-test',
          invocationUrl: undefined,
          discoveryConfig: undefined,
          discoveryGroup: undefined,
          syncTime: undefined,
        },
      },
    },
  });

  render(
    <ContextProvider ctx={ctx}>
      <Task name="task-001" close={() => {}} />
    </ContextProvider>
  );

  await screen.findByText('Details');

  expect(getTableCellContents()).toEqual({
    header: ['Instance ID', 'Instance Name'],
    rows: [
      ['i-016e32a5882f5ee81', ''],
      ['i-065818031835365cc', 'aws-test'],
    ],
  });

  jest.resetAllMocks();
});

test('renders eks impacts', async () => {
  const ctx = new TeleportContext();
  jest.spyOn(integrationService, 'fetchUserTask').mockResolvedValue({
    name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
    taskType: 'discover-eks',
    state: 'OPEN',
    integration: 'integration-001',
    lastStateChange: '2025-02-11T20:32:19.482607921Z',
    issueType: 'eks-failure',
    title: 'eks failure',
    description:
      'Only EKS Clusters whose status is active can be automatically enrolled into teleport.\n',
    discoverEc2: undefined,
    discoverRds: undefined,
    discoverEks: {
      accountId: undefined,
      region: undefined,
      appAutoDiscover: false,
      clusters: {
        'i-016e32a5882f5ee81': {
          name: 'i-016e32a5882f5ee81',
          resourceUrl: '',
          discoveryConfig: undefined,
          discoveryGroup: undefined,
          syncTime: undefined,
        },
        'i-065818031835365cc': {
          name: 'i-065818031835365cc',
          resourceUrl: '',
          discoveryConfig: undefined,
          discoveryGroup: undefined,
          syncTime: undefined,
        },
      },
    },
  });

  render(
    <ContextProvider ctx={ctx}>
      <Task name="task-001" close={() => {}} />
    </ContextProvider>
  );

  await screen.findByText('Details');

  expect(getTableCellContents()).toEqual({
    header: ['Name'],
    rows: [['i-016e32a5882f5ee81'], ['i-065818031835365cc']],
  });
  jest.resetAllMocks();
});

test('renders rds impacts', async () => {
  const ctx = new TeleportContext();
  jest.spyOn(integrationService, 'fetchUserTask').mockResolvedValue({
    name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
    taskType: 'discover-rds',
    state: 'OPEN',
    integration: 'integration-001',
    lastStateChange: '2025-02-11T20:32:19.482607921Z',
    issueType: 'rds-failure',
    title: 'rds failure',
    description:
      'The Teleport Database Service uses [IAM authentication](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html) to communicate with RDS.\n',
    discoverEks: undefined,
    discoverEc2: undefined,
    discoverRds: {
      accountId: undefined,
      region: undefined,
      databases: {
        'i-016e32a5882f5ee81': {
          name: 'i-016e32a5882f5ee81',
          resourceUrl: '',
          isCluster: undefined,
          engine: undefined,
          discoveryConfig: undefined,
          discoveryGroup: undefined,
          syncTime: undefined,
        },
        'i-065818031835365cc': {
          name: 'i-065818031835365cc',
          resourceUrl: '',
          isCluster: undefined,
          engine: undefined,
          discoveryConfig: undefined,
          discoveryGroup: undefined,
          syncTime: undefined,
        },
      },
    },
  });

  render(
    <ContextProvider ctx={ctx}>
      <Task name="task-001" close={() => {}} />
    </ContextProvider>
  );

  await screen.findByText('Details');

  expect(getTableCellContents()).toEqual({
    header: ['Name'],
    rows: [['i-016e32a5882f5ee81'], ['i-065818031835365cc']],
  });
  jest.resetAllMocks();
});

function getTableCellContents() {
  const [header, ...rows] = screen.getAllByRole('row');
  return {
    header: within(header)
      .getAllByRole('columnheader')
      .map(cell => cell.textContent),
    rows: rows.map(row =>
      within(row)
        .getAllByRole('cell')
        .map(cell => cell.textContent)
    ),
  };
}
