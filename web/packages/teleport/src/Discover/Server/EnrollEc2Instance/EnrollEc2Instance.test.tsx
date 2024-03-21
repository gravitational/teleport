/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, fireEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import {
  IntegrationKind,
  IntegrationStatusCode,
  integrationService,
} from 'teleport/services/integrations';
import { createTeleportContext } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';
import TeleportContext from 'teleport/teleportContext';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { Node } from 'teleport/services/nodes';

import { userEventService } from 'teleport/services/userEvent';

import { EnrollEc2Instance } from './EnrollEc2Instance';

describe('test EnrollEc2Instance.tsx', () => {
  beforeEach(() => {
    jest.restoreAllMocks();
  });

  test('a cloudshell script should be shown if there is an aws permissions error', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockRejectedValue(
        new Error('StatusCode: 403, RequestID: operation error')
      );
    // Prevent noise in the test output caused by the error.
    jest.spyOn(console, 'error').mockImplementation();

    renderEc2Instances(ctx, discoverCtx);

    // Selects a region
    const regionSelectorElement = screen.getByLabelText(/aws region/i);
    fireEvent.focus(regionSelectorElement);
    fireEvent.keyDown(regionSelectorElement, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-west-1'));

    // Wait for results to be listed.
    await screen.findAllByText(
      /We were unable to list your EC2 instances. Run the command below/i
    );

    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(ctx.nodeService.fetchNodes).not.toHaveBeenCalled();
  });

  test('an instance that is already enrolled should be disabled', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    jest
      .spyOn(ctx.nodeService, 'fetchNodes')
      .mockResolvedValue({ agents: mockFetchedNodes });

    renderEc2Instances(ctx, discoverCtx);

    // Selects a region
    const regionSelectorElement = screen.getByLabelText(/aws region/i);
    fireEvent.focus(regionSelectorElement);
    fireEvent.keyDown(regionSelectorElement, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-west-1'));

    // Wait for results to be listed.
    await screen.findAllByText(/My EC2 Box 1/i);

    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(ctx.nodeService.fetchNodes).toHaveBeenCalledTimes(1);

    // Get the disabled table rows.
    const disabledRowElements = screen
      .getAllByTitle(
        'This EC2 instance is already enrolled and is a part of this cluster'
      )
      // Only select the radio elements, this is to prevent duplicates since every
      // column in the row will have the title we're querying for.
      .filter(el => el.innerHTML.includes('type="radio"'))
      // Get the row that the radio element is in.
      .map(el => el.closest('tr'));

    // Expect the disabled row to be EC2 Box 2.
    expect(disabledRowElements[0].innerHTML).toContain('My EC2 Box 2');
    // There should only be one disabled row.
    expect(disabledRowElements).toHaveLength(1);
  });

  test('there should be no disabled rows if the fetchNodes response is empty', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    renderEc2Instances(ctx, discoverCtx);

    // Selects a region
    const regionSelectorElement = screen.getByLabelText(/aws region/i);
    fireEvent.focus(regionSelectorElement);
    fireEvent.keyDown(regionSelectorElement, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-west-1'));

    // Wait for results to be listed.
    await screen.findAllByText(/My EC2 Box 1/i);

    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(ctx.nodeService.fetchNodes).toHaveBeenCalledTimes(1);

    // There should be no disabled rows.
    expect(
      screen.queryAllByTitle(
        'This EC2 instance is already enrolled and is a part of this cluster'
      )[0]
    ).toBeUndefined();
  });
});

function getMockedContexts() {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'node-name',
      awsRegion: 'us-west-1',
      agentMatcherLabels: [],
      db: {} as any,
      selectedAwsRdsDb: {} as any,
      node: {} as any,
      awsIntegration: {
        kind: IntegrationKind.AwsOidc,
        name: 'test-oidc',
        resourceType: 'integration',
        spec: {
          roleArn: 'arn-123',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      },
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {} as any,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  jest.spyOn(ctx.nodeService, 'fetchNodes').mockResolvedValue({ agents: [] });
  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);

  return { ctx, discoverCtx };
}

function renderEc2Instances(
  ctx: TeleportContext,
  discoverCtx: DiscoverContextState
) {
  return render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'server' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <EnrollEc2Instance />
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

const mockEc2Instances: Node[] = [
  {
    id: 'ec2-instance-1',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-1',
    labels: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-1' },
      { name: 'Name', value: 'My EC2 Box 1' },
    ],
    addr: 'ec2.1.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    awsMetadata: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-1',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-2',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-2',
    labels: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-2' },
      { name: 'Name', value: 'My EC2 Box 2' },
    ],
    addr: 'ec2.2.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    awsMetadata: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-2',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-3',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-3',
    labels: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-3' },
      { name: 'Name', value: 'My EC2 Box 3' },
    ],
    addr: 'ec2.3.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    awsMetadata: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-3',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-4',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-4',
    labels: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-4' },
      { name: 'Name', value: 'My EC2 Box 4' },
    ],
    addr: 'ec2.4.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    awsMetadata: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-4',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-5',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-5',
    labels: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-5' },
      { name: 'Name', value: 'My EC2 Box 5' },
    ],
    addr: 'ec2.5.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    awsMetadata: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-5',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
];

const mockFetchedNodes: Node[] = [
  {
    id: 'ec2-instance-2',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-2',
    labels: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-2' },
      { name: 'Name', value: 'My EC2 Box 2' },
    ],
    addr: 'ec2.2.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
  },
];
