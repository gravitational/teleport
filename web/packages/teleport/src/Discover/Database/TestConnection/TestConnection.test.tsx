/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { render, screen, userEvent } from 'design/utils/testing';

import {
  ComponentWrapper,
  getDbMeta,
  getDbResourceSpec,
} from 'teleport/Discover/Fixtures/databases';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';
import { agentService } from 'teleport/services/agents';
import auth from 'teleport/services/auth/auth';
import { userEventService } from 'teleport/services/userEvent';

import { TestConnection } from './TestConnection';

beforeEach(() => {
  jest
    .spyOn(agentService, 'createConnectionDiagnostic')
    .mockResolvedValue({ id: '', success: true, message: '', traces: [] });

  jest.spyOn(auth, 'checkMfaRequired').mockResolvedValue({ required: false });

  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);
});

afterEach(() => {
  jest.restoreAllMocks();
});

test('custom db name and user is respected when defined', async () => {
  const dbMeta = getDbMeta();
  dbMeta.db.users = ['user1', '*'];
  dbMeta.db.names = ['name1', '*'];

  render(
    <ComponentWrapper
      dbMeta={dbMeta}
      resourceSpec={getDbResourceSpec(
        DatabaseEngine.MySql,
        DatabaseLocation.SelfHosted
      )}
    >
      <TestConnection />
    </ComponentWrapper>
  );

  // Test with default user and names.
  await userEvent.click(
    screen.getByRole('button', { name: /test connection/i })
  );
  expect(agentService.createConnectionDiagnostic).toHaveBeenCalledWith(
    expect.objectContaining({
      dbTester: {
        name: 'name1',
        user: 'user1',
      },
    })
  );
  expect(
    screen.getByText(/--db-user=user1 --db-name=name1/i)
  ).toBeInTheDocument();

  // Test with custom fields.
  await userEvent.click(screen.getByText('user1'));
  await userEvent.click(screen.getByText('*'));
  expect(
    screen.getByText(/--db-user=<user> --db-name=name1/i)
  ).toBeInTheDocument();

  await userEvent.type(
    screen.getByPlaceholderText(/custom-database-user-name/i),
    'custom-user'
  );

  await userEvent.click(screen.getByText('name1'));
  // The first wildcard is on screen from selecting
  // it for db users dropdown.
  await userEvent.click(screen.getAllByText('*')[1]);
  expect(
    screen.getByText(/--db-user=custom-user --db-name=<name>/i)
  ).toBeInTheDocument();

  await userEvent.type(
    screen.getByPlaceholderText(/custom-database-name/i),
    'custom-name'
  );

  await userEvent.click(screen.getByRole('button', { name: /restart test/i }));
  expect(agentService.createConnectionDiagnostic).toHaveBeenCalledWith(
    expect.objectContaining({
      dbTester: {
        name: 'custom-name',
        user: 'custom-user',
      },
    })
  );
  expect(
    screen.getByText(/--db-user=custom-user --db-name=custom-name/i)
  ).toBeInTheDocument();
});
