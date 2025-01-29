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

import { act, fireEvent, render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';
import { ComponentWrapper } from 'teleport/Discover/Fixtures/databases';
import DatabaseService from 'teleport/services/databases/databases';
import * as discoveryService from 'teleport/services/discovery/discovery';
import { DISCOVERY_GROUP_CLOUD } from 'teleport/services/discovery/discovery';
import {
  AwsRdsDatabase,
  integrationService,
} from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';

import { EnrollRdsDatabase } from './EnrollRdsDatabase';

const defaultIsCloud = cfg.isCloud;

describe('test EnrollRdsDatabase.tsx', () => {
  let createDiscoveryConfig;
  beforeEach(() => {
    cfg.isCloud = true;
    jest
      .spyOn(DatabaseService.prototype, 'fetchDatabases')
      .mockResolvedValue({ agents: [] });
    jest
      .spyOn(DatabaseService.prototype, 'createDatabase')
      .mockResolvedValue({} as any);
    jest
      .spyOn(userEventService, 'captureDiscoverEvent')
      .mockResolvedValue(undefined as never);
    createDiscoveryConfig = jest
      .spyOn(discoveryService, 'createDiscoveryConfig')
      .mockResolvedValue({
        name: '',
        discoveryGroup: '',
        aws: [],
      });
    jest
      .spyOn(DatabaseService.prototype, 'fetchDatabaseServices')
      .mockResolvedValue({ services: [] });
    jest.spyOn(integrationService, 'fetchAwsDatabasesVpcs').mockResolvedValue({
      nextToken: '',
      vpcs: [
        {
          name: 'vpc-name',
          id: 'vpc-id',
        },
      ],
    });
  });

  afterEach(() => {
    cfg.isCloud = defaultIsCloud;
    jest.restoreAllMocks();
  });

  async function selectRegionAndVpc() {
    // select a region
    let selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown' });
    fireEvent.click(screen.getByText('us-east-2'));

    await screen.findByLabelText(/vpc id/i);

    // select a vpc
    selectEl = screen.getByText(/select a vpc id/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown' });
    fireEvent.keyDown(selectEl, { key: 'Enter' });

    await screen.findByText(/selected VPC/i);
  }

  test('without rds database result, does not attempt to fetch db servers', async () => {
    jest
      .spyOn(integrationService, 'fetchAwsRdsDatabases')
      .mockResolvedValue({ databases: [] });

    render(<Component />);

    await selectRegionAndVpc();

    expect(integrationService.fetchAwsRdsDatabases).toHaveBeenCalledTimes(1);
    expect(DatabaseService.prototype.fetchDatabases).not.toHaveBeenCalled();
  });

  test('with rds database result, makes a fetch request for db servers', async () => {
    jest.spyOn(integrationService, 'fetchAwsRdsDatabases').mockResolvedValue({
      databases: mockAwsDbs,
    });

    render(<Component />);

    await selectRegionAndVpc();

    // Rds results renders result.
    await screen.findByText(/rds-1/i);

    expect(integrationService.fetchAwsRdsDatabases).toHaveBeenCalledTimes(1);
    expect(DatabaseService.prototype.fetchDatabases).toHaveBeenCalledTimes(1);
  });

  test('auto enrolling with cloud should create discovery config', async () => {
    jest
      .spyOn(integrationService, 'fetchAwsRdsDatabases')
      .mockResolvedValue({ databases: [] });
    jest
      .spyOn(integrationService, 'fetchAllAwsRdsEnginesDatabases')
      .mockResolvedValue({
        databases: mockAwsDbs,
      });

    render(<Component />);

    await selectRegionAndVpc();

    // Toggle on auto-enroll
    act(() => screen.getByText(/auto-enroll all/i).click());

    // Rds results renders result.
    await screen.findByText(/rds-1/i);

    act(() => screen.getByText('Next').click());
    await screen.findByText(/Creating Auto Discovery Config/i);
    expect(discoveryService.createDiscoveryConfig).toHaveBeenCalledTimes(1);

    // 2D array:
    // First array is the array of calls, we are only interested in the first.
    // Second array are the parameters that this api got called with,
    // we are interested in the second parameter.
    expect(createDiscoveryConfig.mock.calls[0][1]['discoveryGroup']).toEqual(
      DISCOVERY_GROUP_CLOUD
    );

    expect(DatabaseService.prototype.createDatabase).not.toHaveBeenCalled();
  });

  test('auto enrolling with self-hosted should not create discovery config (its done on the next step)', async () => {
    cfg.isCloud = false;

    jest
      .spyOn(integrationService, 'fetchAwsRdsDatabases')
      .mockResolvedValue({ databases: [] });
    jest
      .spyOn(integrationService, 'fetchAllAwsRdsEnginesDatabases')
      .mockResolvedValue({
        databases: mockAwsDbs,
      });

    render(<Component />);

    await selectRegionAndVpc();

    // Toggle on auto-enroll
    act(() => screen.getByText(/auto-enroll all/i).click());

    // Rds results renders result.
    await screen.findByText(/rds-1/i);

    act(() => screen.getByText('Next').click());
    expect(discoveryService.createDiscoveryConfig).not.toHaveBeenCalled();
    expect(DatabaseService.prototype.createDatabase).not.toHaveBeenCalled();
  });

  test('auto enroll disabled, creates database', async () => {
    jest.spyOn(integrationService, 'fetchAwsRdsDatabases').mockResolvedValue({
      databases: mockAwsDbs,
    });

    render(<Component />);

    await selectRegionAndVpc();

    await screen.findByText(/rds-1/i);

    act(() => screen.getByRole('radio').click());

    act(() => screen.getByText('Next').click());
    await screen.findByText(/Database "rds-1" successfully registered/i);

    expect(discoveryService.createDiscoveryConfig).not.toHaveBeenCalled();
    expect(
      DatabaseService.prototype.fetchDatabaseServices
    ).toHaveBeenCalledTimes(1);
    expect(DatabaseService.prototype.createDatabase).toHaveBeenCalledTimes(1);
  });
});

const mockAwsDbs: AwsRdsDatabase[] = [
  {
    engine: 'postgres',
    name: 'rds-1',
    uri: 'endpoint-1',
    status: 'available',
    labels: [{ name: 'env', value: 'prod' }],
    accountId: 'account-id-1',
    resourceId: 'resource-id-1',
    vpcId: 'vpc-123',
    securityGroups: ['sg-1', 'sg-2'],
    region: 'us-east-2',
    subnets: ['subnet1', 'subnet2'],
  },
];

const Component = () => (
  <ComponentWrapper>
    <EnrollRdsDatabase />
  </ComponentWrapper>
);
