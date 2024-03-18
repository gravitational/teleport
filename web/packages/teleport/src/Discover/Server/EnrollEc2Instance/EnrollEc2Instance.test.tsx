/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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

import { EnrollEc2Instance } from './EnrollEc2Instance';

describe('test EnrollEc2Instance.tsx', () => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'node-name',
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

  beforeEach(() => {
    jest.spyOn(ctx.nodeService, 'fetchNodes').mockResolvedValue({ agents: [] });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('a cloudshell script should be shown if there is an aws permissions error', async () => {
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

  // TODO: Fix flaky network error failure in this test
  test.skip('there should be no disabled rows if the fetchNodes response is empty', async () => {
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
