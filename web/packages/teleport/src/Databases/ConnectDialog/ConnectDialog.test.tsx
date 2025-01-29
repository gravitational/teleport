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

import { render, screen } from 'design/utils/testing';

import ConnectDialog, { Props } from './ConnectDialog';

test('correct connect command generated for postgres db', () => {
  render(<ConnectDialog {...props} dbProtocol="postgres" />);

  // --db-name flag should be required
  const expectedOutput =
    'tsh db connect aurora --db-user=<user> --db-name=<name>';

  expect(screen.getByText(expectedOutput)).toBeInTheDocument();
});

test('correct connect command generated for spanner', () => {
  render(<ConnectDialog {...props} dbName="gspanner" dbProtocol="spanner" />);

  // --db-name flag should be required
  const expectedOutput =
    'tsh db connect gspanner --db-user=<user> --db-name=<name>';

  expect(screen.getByText(expectedOutput)).toBeInTheDocument();
});

test('correct connect command generated for mysql db', () => {
  render(<ConnectDialog {...props} dbProtocol="mysql" />);

  // --db-name flag should be optional
  const expectedOutput =
    'tsh db connect aurora --db-user=<user> [--db-name=<name>]';

  expect(screen.getByText(expectedOutput)).toBeInTheDocument();
});

test('correct connect command generated for redis', () => {
  render(<ConnectDialog {...props} dbProtocol="redis" />);

  // There should be no --db-name flag
  const expectedOutput = 'tsh db connect aurora --db-user=<user>';

  expect(screen.getByText(expectedOutput)).toBeInTheDocument();
});

test('correct connect command generated for dynamodb', () => {
  render(<ConnectDialog {...props} dbProtocol="dynamodb" />);

  // Command should be `tsh proxy db --tunnel` instead of `tsh connect db`
  const expectedOutput = 'tsh proxy db --tunnel aurora --db-user=<user>';

  expect(screen.getByText(expectedOutput)).toBeInTheDocument();
});

test('correct tsh login command generated with local authType', () => {
  render(<ConnectDialog {...props} />);
  const output =
    'tsh login --proxy=localhost:443 --auth=local --user=yassine im-a-cluster';

  expect(screen.getByText(output)).toBeInTheDocument();
});

test('correct tsh login command generated with sso authType', () => {
  render(<ConnectDialog {...props} authType="sso" />);
  const output = 'tsh login --proxy=localhost:443 im-a-cluster';

  expect(screen.getByText(output)).toBeInTheDocument();
});

test('correct tsh login command generated with passwordless authType', () => {
  render(<ConnectDialog {...props} authType="passwordless" />);
  const output =
    'tsh login --proxy=localhost:443 --auth=passwordless --user=yassine im-a-cluster';

  expect(screen.getByText(output)).toBeInTheDocument();
});

const props: Props = {
  username: 'yassine',
  dbName: 'aurora',
  clusterId: 'im-a-cluster',
  onClose: () => null,
  authType: 'local',
  dbProtocol: 'postgres',
};
