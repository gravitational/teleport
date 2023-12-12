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
import { render, screen, fireEvent, act } from 'design/utils/testing';

import {
  AwsRdsDatabase,
  integrationService,
} from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';
import { TeleportProvider, getDbMeta } from 'teleport/Discover/fixtures';
import DatabaseService from 'teleport/services/databases/databases';
import * as discoveryService from 'teleport/services/discovery/discovery';

import { EnrollRdsDatabase } from './EnrollRdsDatabase';

describe('test EnrollRdsDatabase.tsx', () => {
  beforeEach(() => {
    jest
      .spyOn(DatabaseService.prototype, 'fetchDatabases')
      .mockResolvedValue({ agents: [] });
    jest
      .spyOn(userEventService, 'captureDiscoverEvent')
      .mockResolvedValue(undefined as never);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  test('without rds database result, does not attempt to fetch db servers', async () => {
    jest
      .spyOn(integrationService, 'fetchAwsRdsDatabases')
      .mockResolvedValue({ databases: [] });

    render(
      <TeleportProvider agentMeta={getDbMeta()}>
        <EnrollRdsDatabase />
      </TeleportProvider>
    );

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // No results are rendered.
    await screen.findByText(/no result/i);

    expect(integrationService.fetchAwsRdsDatabases).toHaveBeenCalledTimes(1);
    expect(DatabaseService.prototype.fetchDatabases).not.toHaveBeenCalled();
  });

  test('with rds database result, makes a fetch request for db servers', async () => {
    jest.spyOn(integrationService, 'fetchAwsRdsDatabases').mockResolvedValue({
      databases: mockAwsDbs,
    });

    render(
      <TeleportProvider agentMeta={getDbMeta()}>
        <EnrollRdsDatabase />
      </TeleportProvider>
    );

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // Rds results renders result.
    await screen.findByText(/rds-1/i);

    expect(integrationService.fetchAwsRdsDatabases).toHaveBeenCalledTimes(1);
    expect(DatabaseService.prototype.fetchDatabases).toHaveBeenCalledTimes(1);
  });

  test('auto enroll is on by default and creates discovery config', async () => {
    jest.spyOn(integrationService, 'fetchAwsRdsDatabases').mockResolvedValue({
      databases: mockAwsDbs,
    });
    jest.spyOn(discoveryService, 'createDiscoveryConfig').mockResolvedValue({
      name: '',
      discoveryGroup: '',
      aws: [],
    });
    jest
      .spyOn(DatabaseService.prototype, 'fetchDatabaseServices')
      .mockResolvedValue({ services: [] });
    jest
      .spyOn(DatabaseService.prototype, 'createDatabase')
      .mockResolvedValue({} as any);

    render(
      <TeleportProvider agentMeta={getDbMeta()}>
        <EnrollRdsDatabase />
      </TeleportProvider>
    );

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // Rds results renders result.
    await screen.findByText(/rds-1/i);

    act(() => screen.getByText('Next').click());
    await screen.findByText(/Creating Auto Discovery Config/i);
    expect(discoveryService.createDiscoveryConfig).toHaveBeenCalledTimes(1);
    expect(
      DatabaseService.prototype.fetchDatabaseServices
    ).toHaveBeenCalledTimes(1);
    expect(DatabaseService.prototype.createDatabase).not.toHaveBeenCalled();
  });

  test('auto enroll disabled, creates database', async () => {
    jest.spyOn(integrationService, 'fetchAwsRdsDatabases').mockResolvedValue({
      databases: mockAwsDbs,
    });
    jest
      .spyOn(DatabaseService.prototype, 'fetchDatabaseServices')
      .mockResolvedValue({ services: [] });
    jest
      .spyOn(DatabaseService.prototype, 'createDatabase')
      .mockResolvedValue({} as any);
    jest.spyOn(discoveryService, 'createDiscoveryConfig').mockResolvedValue({
      name: '',
      discoveryGroup: '',
      aws: [],
    });

    render(
      <TeleportProvider agentMeta={getDbMeta()}>
        <EnrollRdsDatabase />
      </TeleportProvider>
    );

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    await screen.findByText(/rds-1/i);

    // disable auto enroll
    act(() => screen.getByText(/auto-enroll all/i).click());
    expect(screen.getByText('Next')).toBeDisabled();

    act(() => screen.getByTestId('input-radio').click());
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
    region: 'us-east-2',
    subnets: ['subnet1', 'subnet2'],
  },
];
