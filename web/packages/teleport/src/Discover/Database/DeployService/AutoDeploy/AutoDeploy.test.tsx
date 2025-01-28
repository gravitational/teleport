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

import { MemoryRouter } from 'react-router';

import { act, fireEvent, render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';
import { ResourceKind } from 'teleport/Discover/Shared';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { SHOW_HINT_TIMEOUT } from 'teleport/Discover/Shared/useShowHint';
import {
  DbMeta,
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  AwsRdsDatabase,
  IntegrationAwsOidc,
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
  Regions,
} from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';
import TeleportContext from 'teleport/teleportContext';

import { AutoDeploy } from './AutoDeploy';

const mockDbLabels = [{ name: 'env', value: 'prod' }];

const integrationName = 'aws-oidc-integration';
const region: Regions = 'us-east-2';
const awsoidcRoleName = 'role-arn';

const mockAwsRdsDb: AwsRdsDatabase = {
  engine: 'postgres',
  name: 'rds-1',
  uri: 'endpoint-1',
  status: 'available',
  labels: mockDbLabels,
  accountId: 'account-id-1',
  resourceId: 'resource-id-1',
  vpcId: 'vpc-123',
  securityGroups: ['sg-1', 'sg-2'],
  region: region,
  subnets: ['subnet1', 'subnet2'],
};

const mocKIntegration: IntegrationAwsOidc = {
  kind: IntegrationKind.AwsOidc,
  name: integrationName,
  resourceType: 'integration',
  spec: {
    roleArn: `arn:aws:iam::123456789012:role/${awsoidcRoleName}`,
    issuerS3Bucket: '',
    issuerS3Prefix: '',
  },
  statusCode: IntegrationStatusCode.Running,
};

describe('test AutoDeploy.tsx', () => {
  jest.useFakeTimers();

  beforeEach(() => {
    jest.spyOn(integrationService, 'fetchAwsSubnets').mockResolvedValue({
      nextToken: '',
      subnets: [
        {
          name: 'subnet-name',
          id: 'subnet-id',
          availabilityZone: 'subnet-az',
        },
      ],
    });
    jest.spyOn(integrationService, 'fetchSecurityGroups').mockResolvedValue({
      nextToken: '',
      securityGroups: [
        {
          name: 'sg-name',
          id: 'sg-id',
          description: 'sg-desc',
          inboundRules: [],
          outboundRules: [],
        },
      ],
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  async function waitForSubnetsAndSecurityGroups() {
    await screen.findByText('sg-id');
    await screen.findByText('subnet-id');
  }

  test('clicking button renders command', async () => {
    const { teleCtx, discoverCtx } = getMockedContexts();

    renderAutoDeploy(teleCtx, discoverCtx);
    await waitForSubnetsAndSecurityGroups();

    fireEvent.click(screen.getByText(/generate command/i));

    expect(screen.getByText(/copy\/paste/i)).toBeInTheDocument();
    expect(
      screen.getByText(
        /integrationName=aws-oidc-integration&awsRegion=us-east-2&role=role-arn&taskRole=TeleportDatabaseAccess/i
      )
    ).toBeInTheDocument();
  });

  test('invalid role name', async () => {
    const { teleCtx, discoverCtx } = getMockedContexts();

    renderAutoDeploy(teleCtx, discoverCtx);
    await waitForSubnetsAndSecurityGroups();

    expect(
      screen.queryByText(/name can only contain/i)
    ).not.toBeInTheDocument();

    // add invalid characters in role name
    const inputEl = screen.getByPlaceholderText(/TeleportDatabaseAccess/i);
    fireEvent.change(inputEl, { target: { value: 'invalidname!@#!$!%' } });

    fireEvent.click(screen.getByText(/generate command/i));
    expect(screen.getByText(/name can only contain/i)).toBeInTheDocument();

    // change back to valid name
    fireEvent.change(inputEl, { target: { value: 'llama' } });
    expect(
      screen.queryByText(/name can only contain/i)
    ).not.toBeInTheDocument();
  });

  test('deploy hint states', async () => {
    const { teleCtx, discoverCtx } = getMockedContexts();

    renderAutoDeploy(teleCtx, discoverCtx);
    await waitForSubnetsAndSecurityGroups();

    fireEvent.click(screen.getByText(/Deploy Teleport Service/i));

    // select required subnet
    expect(
      screen.getByText(/one subnet selection is required/i)
    ).toBeInTheDocument();
    fireEvent.click(screen.getByTestId(/subnet-id/i));

    fireEvent.click(screen.getByText(/Deploy Teleport Service/i));

    // select required sg
    expect(
      screen.getByText(/one security group selection is required/i)
    ).toBeInTheDocument();
    fireEvent.click(screen.getByTestId(/sg-id/i));

    fireEvent.click(screen.getByText(/Deploy Teleport Service/i));

    // test initial loading state
    await screen.findByText(
      /Teleport is currently deploying a Database Service/i
    );

    // test waiting state
    act(() => jest.advanceTimersByTime(SHOW_HINT_TIMEOUT + 1));

    expect(
      screen.getByText(
        /We're still in the process of creating your Database Service/i
      )
    ).toBeInTheDocument();

    // test success state
    jest.spyOn(teleCtx.databaseService, 'fetchDatabases').mockResolvedValue({
      agents: [{} as any], // the result doesn't matter, just need size one array.
    });

    act(() => jest.advanceTimersByTime(TEST_PING_INTERVAL + 1));
    await screen.findByText(/Successfully created/i);
  });
});

const TEST_PING_INTERVAL = 1000 * 60 * 5; // 5 minutes

function getMockedContexts() {
  const teleCtx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'db1',
      awsRegion: region,
      awsIntegration: mocKIntegration,
      selectedAwsRdsDb: mockAwsRdsDb,
      agentMatcherLabels: mockDbLabels,
    } as DbMeta,
    currentStep: 0,
    nextStep: jest.fn(x => x),
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      dbMeta: {
        location: DatabaseLocation.Aws,
        engine: DatabaseEngine.AuroraMysql,
      },
    } as any,
    viewConfig: null,
    exitFlow: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: jest.fn(x => x),
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  jest
    .spyOn(integrationService, 'deployDatabaseServices')
    .mockResolvedValue('dashboard-url');

  jest.spyOn(teleCtx.databaseService, 'fetchDatabases').mockResolvedValue({
    agents: [],
  });

  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);

  return { teleCtx, discoverCtx };
}

function renderAutoDeploy(
  ctx: TeleportContext,
  discoverCtx: DiscoverContextState
) {
  return render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'database' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <PingTeleportProvider
              interval={TEST_PING_INTERVAL}
              resourceKind={ResourceKind.Database}
            >
              <AutoDeploy />
            </PingTeleportProvider>
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
