/**
 * Copyright 2021 Gravitational, Inc.
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
import { render, screen } from 'design/utils/testing';

import ConnectDialog, { Props } from './ConnectDialog';

test('correct connect command generated for postgres db', () => {
  render(<ConnectDialog {...props} dbProtocol="postgres" />);

  const expectedOutput =
    'tsh db connect [--db-user=<user>] [--db-name=<name>] aurora';

  expect(screen.getByText(expectedOutput)).toBeInTheDocument();
});

test('correct connect command generated for mysql db', () => {
  render(<ConnectDialog {...props} dbProtocol="mysql" />);

  const expectedOutput =
    'tsh db connect [--db-user=<user>] [--db-name=<name>] aurora';

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

test('render dialog with instructions to connect to database', () => {
  render(<ConnectDialog {...props} />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

const props: Props = {
  username: 'yassine',
  dbName: 'aurora',
  clusterId: 'im-a-cluster',
  onClose: () => null,
  authType: 'local',
  dbProtocol: 'postgres',
};
