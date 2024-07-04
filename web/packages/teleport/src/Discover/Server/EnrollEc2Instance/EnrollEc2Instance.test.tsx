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
import {
  render,
  screen,
  fireEvent,
  act,
  userEvent,
} from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import {
  Ec2InstanceConnectEndpoint,
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
  NodeMeta,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { Node } from 'teleport/services/nodes';

import {
  DiscoverEvent,
  DiscoverEventStatus,
  userEventService,
} from 'teleport/services/userEvent';
import * as discoveryApi from 'teleport/services/discovery/discovery';
import { DEFAULT_DISCOVERY_GROUP_NON_CLOUD } from 'teleport/services/discovery/discovery';

import { EnrollEc2Instance } from './EnrollEc2Instance';

const defaultIsCloud = cfg.isCloud;

describe('test EnrollEc2Instance.tsx', () => {
  afterEach(() => {
    cfg.isCloud = defaultIsCloud;
    jest.restoreAllMocks();
  });

  const selectedRegion = 'us-west-1';

  async function selectARegion({
    waitForSelfHosted,
    waitForTable,
  }: {
    waitForTable?: boolean;
    waitForSelfHosted?: boolean;
  }) {
    const regionSelectorElement = screen.getByLabelText(/aws region/i);
    fireEvent.focus(regionSelectorElement);
    fireEvent.keyDown(regionSelectorElement, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText(selectedRegion));

    if (waitForTable) {
      return await screen.findAllByText(/My EC2 Box 1/i);
    }

    if (waitForSelfHosted) {
      return await screen.findAllByText(/create a join token/i);
    }
  }

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
    await selectARegion({});

    // Wait for results to be listed.
    await screen.findAllByText(
      /We were unable to list your EC2 instances. Run the command below/i
    );

    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(ctx.nodeService.fetchNodes).not.toHaveBeenCalled();
  });

  test('single instance, an instance that is already enrolled should be disabled', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    jest
      .spyOn(ctx.nodeService, 'fetchNodes')
      .mockResolvedValue({ agents: mockFetchedNodes });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForSelfHosted: true });

    // toggle off auto enroll, to test the table.
    await userEvent.click(screen.getByText(/auto-enroll all/i));
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

  test('single instance, there should be no disabled rows if the fetchNodes response is empty', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForSelfHosted: true });

    // toggle off auto enroll
    await userEvent.click(screen.getByText(/auto-enroll all/i));
    await screen.findAllByText(/My EC2 Box 1/i);

    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(ctx.nodeService.fetchNodes).toHaveBeenCalledTimes(1);

    // There should be no disabled rows.
    expect(
      screen.queryAllByTitle(
        'This EC2 instance is already enrolled and is a part of this cluster'
      )
    ).toHaveLength(0);
  });

  test('self-hosted, auto discover toggling', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForSelfHosted: true });

    // default toggler should be checked.
    expect(screen.getByTestId('toggle')).toBeChecked();
    expect(screen.queryByText(/My EC2 Box 1/i)).not.toBeInTheDocument();
    expect(screen.getByText(/next/i, { selector: 'button' })).toBeEnabled();

    // toggle off auto enroll, should render table.
    await userEvent.click(screen.getByText(/auto-enroll all/i));
    expect(screen.getByTestId('toggle')).not.toBeChecked();
    expect(screen.getByText(/next/i, { selector: 'button' })).toBeDisabled();

    await screen.findAllByText(/My EC2 Box 1/i);

    // toggle it back on.
    await userEvent.click(screen.getByText(/auto-enroll all/i));
    expect(screen.getByTestId('toggle')).toBeChecked();
  });

  test('cloud, auto discover toggling', async () => {
    cfg.isCloud = true;

    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForTable: true });

    // default toggler should be checked.
    expect(screen.queryByText(/create a join token/i)).not.toBeInTheDocument();
    expect(screen.getByTestId('toggle')).toBeChecked();
    expect(screen.getByText(/next/i, { selector: 'button' })).toBeEnabled();

    // toggle off auto enroll
    await userEvent.click(screen.getByText(/auto-enroll all/i));
    await screen.findAllByText(/My EC2 Box 1/i);
    expect(screen.getByTestId('toggle')).not.toBeChecked();
    expect(screen.getByText(/next/i, { selector: 'button' })).toBeDisabled();

    // toggle it back on.
    await userEvent.click(screen.getByText(/auto-enroll all/i));
    expect(screen.getByTestId('toggle')).toBeChecked();
  });

  test('self-hosted, auto discover without existing endpoints', async () => {
    const { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    jest
      .spyOn(integrationService, 'fetchAwsEc2InstanceConnectEndpoints')
      .mockResolvedValue({ endpoints: [], dashboardLink: '' });

    const createDiscoveryConfig = jest
      .spyOn(discoveryApi, 'createDiscoveryConfig')
      .mockResolvedValue({
        name: 'discovery-cfg',
        discoveryGroup: '',
        aws: [],
      });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForSelfHosted: true });

    await userEvent.click(screen.getByText(/next/i, { selector: 'button' }));
    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledWith(
      discoverCtx.agentMeta.awsIntegration.name,
      { region: selectedRegion, nextToken: '' }
    );
    expect(createDiscoveryConfig.mock.calls[0][1]['discoveryGroup']).toBe(
      DEFAULT_DISCOVERY_GROUP_NON_CLOUD
    );
    expect(discoverCtx.nextStep).toHaveBeenCalledTimes(1);
  });

  test('self-hosted, auto discover without all existing endpoints, creates node resource', async () => {
    const { ctx, discoverCtx } = getMockedContexts();
    (discoverCtx.agentMeta as NodeMeta).ec2Ices = endpoints;

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    jest
      .spyOn(integrationService, 'fetchAwsEc2InstanceConnectEndpoints')
      .mockResolvedValue({ endpoints, dashboardLink: '' });

    jest.spyOn(discoveryApi, 'createDiscoveryConfig').mockResolvedValue({
      name: 'discovery-cfg',
      discoveryGroup: '',
      aws: [],
    });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForSelfHosted: true });

    await userEvent.click(screen.getByText(/next/i, { selector: 'button' }));
    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(discoveryApi.createDiscoveryConfig).toHaveBeenCalledTimes(1);
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(discoverCtx.emitEvent).toHaveBeenCalledWith(
      { stepStatus: DiscoverEventStatus.Skipped },
      {
        eventName: DiscoverEvent.EC2DeployEICE,
      }
    );

    await screen.findByText(/created teleport node/i);
    expect(ctx.nodeService.createNode).toHaveBeenCalledTimes(1);
  });

  test('cloud, auto discover with all existing created endpoints and no auto discovery config', async () => {
    cfg.isCloud = true;

    let { ctx, discoverCtx } = getMockedContexts();

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    jest
      .spyOn(integrationService, 'fetchAwsEc2InstanceConnectEndpoints')
      .mockResolvedValue({
        endpoints,
        dashboardLink: '',
      });

    const createDiscoveryConfig = jest
      .spyOn(discoveryApi, 'createDiscoveryConfig')
      .mockResolvedValue({
        name: 'discovery-cfg',
        discoveryGroup: '',
        aws: [],
      });

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForTable: true });

    await userEvent.click(screen.getByText(/next/i, { selector: 'button' }));
    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledWith(
      discoverCtx.agentMeta.awsIntegration.name,
      { region: selectedRegion, nextToken: '' }
    );
    expect(createDiscoveryConfig.mock.calls[0][1]['discoveryGroup']).toBe(
      discoveryApi.DISCOVERY_GROUP_CLOUD
    );
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(discoverCtx.emitEvent).toHaveBeenCalledWith(
      { stepStatus: DiscoverEventStatus.Skipped },
      {
        eventName: DiscoverEvent.EC2DeployEICE,
      }
    );
  });

  test('cloud, auto discover with all existing created endpoints, with already set discovery config', async () => {
    cfg.isCloud = true;

    let { ctx, discoverCtx } = getMockedContexts(true /* withAutoDiscovery */);

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    jest
      .spyOn(integrationService, 'fetchAwsEc2InstanceConnectEndpoints')
      .mockResolvedValue({
        endpoints: [
          {
            name: 'endpoint-1',
            state: 'create-complete',
            dashboardLink: '',
            subnetId: 'subnet-1',
            vpcId: 'vpc-1',
          },
          {
            name: 'endpoint-2',
            state: 'create-complete',
            dashboardLink: '',
            subnetId: 'subnet-2',
            vpcId: 'vpc-2',
          },
          {
            name: 'endpoint-3',
            state: 'create-complete',
            dashboardLink: '',
            subnetId: 'subnet-3',
            vpcId: 'vpc-3',
          },
        ],
        dashboardLink: '',
      });

    jest.spyOn(discoveryApi, 'createDiscoveryConfig').mockResolvedValue({
      name: 'discovery-cfg',
      discoveryGroup: '',
      aws: [],
    });

    jest.spyOn(ctx.nodeService, 'createNode').mockResolvedValue({} as any);

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForTable: true });

    await userEvent.click(screen.getByText(/next/i, { selector: 'button' }));
    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledWith(
      discoverCtx.agentMeta.awsIntegration.name,
      { region: selectedRegion, nextToken: '' }
    );
    expect(discoveryApi.createDiscoveryConfig).not.toHaveBeenCalled();
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(ctx.nodeService.createNode).not.toHaveBeenCalled();

    expect(discoverCtx.emitEvent).toHaveBeenCalledWith(
      { stepStatus: DiscoverEventStatus.Skipped },
      {
        eventName: DiscoverEvent.EC2DeployEICE,
      }
    );

    await screen.findByText(/All endpoints required are created/i);
  });

  test('cloud, with partially created endpoints, with already set discovery config', async () => {
    cfg.isCloud = true;
    jest.useFakeTimers();

    const { ctx, discoverCtx } = getMockedContexts(
      true /* withAutoDiscovery */
    );

    jest
      .spyOn(integrationService, 'fetchAwsEc2Instances')
      .mockResolvedValue({ instances: mockEc2Instances });

    const fetchEndpoints = jest
      .spyOn(integrationService, 'fetchAwsEc2InstanceConnectEndpoints')
      .mockResolvedValueOnce({
        endpoints: [
          {
            name: 'endpoint-1',
            state: 'create-complete',
            dashboardLink: '',
            subnetId: 'subnet-1',
            vpcId: 'vpc-1',
          },
          {
            name: 'endpoint-2',
            state: 'create-in-progress', // <-- should trigger polling
            dashboardLink: '',
            subnetId: 'subnet-2',
            vpcId: 'vpc-2',
          },
          {
            name: 'endpoint-3',
            state: 'create-complete',
            dashboardLink: '',
            subnetId: 'subnet-3',
            vpcId: 'vpc-3',
          },
        ],
        dashboardLink: '',
      })
      .mockResolvedValueOnce({
        endpoints: [
          {
            name: 'endpoint-2',
            state: 'create-complete', // <-- should stop polling
            dashboardLink: '',
            subnetId: 'subnet-2',
            vpcId: 'vpc-2',
          },
        ],
        dashboardLink: '',
      });
    jest.spyOn(discoveryApi, 'createDiscoveryConfig').mockResolvedValue({
      name: 'discovery-cfg',
      discoveryGroup: '',
      aws: [],
    });
    jest.spyOn(ctx.nodeService, 'createNode').mockResolvedValue({} as any);

    renderEc2Instances(ctx, discoverCtx);
    await selectARegion({ waitForTable: true });

    // Test it's polling.
    fireEvent.click(screen.getByText(/next/i, { selector: 'button' }));
    await screen.findByText(/this may take a few minutes/i);

    expect(integrationService.fetchAwsEc2Instances).toHaveBeenCalledTimes(1);
    expect(discoveryApi.createDiscoveryConfig).not.toHaveBeenCalled();
    expect(ctx.nodeService.createNode).not.toHaveBeenCalled();
    expect(discoverCtx.nextStep).not.toHaveBeenCalled();
    expect(discoverCtx.emitEvent).toHaveBeenCalledWith(
      { stepStatus: DiscoverEventStatus.Skipped },
      {
        eventName: DiscoverEvent.EC2DeployEICE,
      }
    );
    expect(fetchEndpoints).toHaveBeenCalledTimes(1);
    fetchEndpoints.mockClear();

    // advance timer to call the endpoint with completed state
    await act(async () => jest.advanceTimersByTime(10000));
    await screen.findByText(/All endpoints required are created/i);
    expect(fetchEndpoints).toHaveBeenCalledTimes(1);

    jest.useRealTimers();
  });
});

function getMockedContexts(withAutoDiscovery = false) {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'node-name',
      awsRegion: 'us-west-1',
      agentMatcherLabels: [],
      db: {} as any,
      selectedAwsRdsDb: {} as any,
      node: mockFetchedNodes[0],
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
      autoDiscovery: withAutoDiscovery
        ? {
            config: { name: '', discoveryGroup: '', aws: [] },
            requiredVpcsAndSubnets: {},
          }
        : undefined,
    },
    currentStep: 0,
    nextStep: jest.fn(),
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {} as any,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: jest.fn(),
    eventState: null,
  };

  jest.spyOn(ctx.nodeService, 'fetchNodes').mockResolvedValue({ agents: [] });
  jest
    .spyOn(ctx.nodeService, 'createNode')
    .mockResolvedValue(mockFetchedNodes[0]);
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
      vpcId: 'vpc-1',
      integration: 'test',
      subnetId: 'subnet-1',
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
      vpcId: 'vpc-2',
      integration: 'test',
      subnetId: 'subnet-2',
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
      vpcId: 'vpc-1',
      integration: 'test',
      subnetId: 'subnet-2',
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
      vpcId: 'vpc-2',
      integration: 'test',
      subnetId: 'subnet-2',
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
      vpcId: 'vpc-3',
      integration: 'test',
      subnetId: 'subnet-3',
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
    awsMetadata: {
      instanceId: 'some-id',
      accountId: '',
      region: 'us-east-1',
      vpcId: '',
      integration: '',
      subnetId: '',
    },
  },
];

const endpoints: Ec2InstanceConnectEndpoint[] = [
  {
    name: 'endpoint-1',
    state: 'create-complete',
    dashboardLink: '',
    subnetId: 'subnet-1',
    vpcId: 'vpc-1',
  },
  {
    name: 'endpoint-2',
    state: 'create-complete',
    dashboardLink: '',
    subnetId: 'subnet-2',
    vpcId: 'vpc-2',
  },
  {
    name: 'endpoint-3',
    state: 'create-complete',
    dashboardLink: '',
    subnetId: 'subnet-3',
    vpcId: 'vpc-3',
  },
];
