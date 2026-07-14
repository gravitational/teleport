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

import { makeAppGateway } from 'teleterm/services/tshd/testHelpers';

import {
  getCliCommandArgs,
  getCliCommandEnv,
  getTargetSubresourceName,
} from './gateway';
import { GatewayCLICommand } from './types';

describe('getCliCommandArgs', () => {
  it("extracts Node.js-style args from cliCommand's args", () => {
    const cliCommand = makeCliCommand();

    const args = getCliCommandArgs(cliCommand);

    expect(args).toEqual([cliCommand.args[1]]);
  });
});

describe('getCliCommandEnv', () => {
  it('converts Go-style env into a record', () => {
    const cliCommand = makeCliCommand();

    const env = getCliCommandEnv(cliCommand);

    expect(env.foo).toBe('bar');
    expect(env.baz).toBe('quux');
  });
});

describe('getTargetSubresourceName', () => {
  test.each<{ name: string; value: string; expected: string | undefined }>([
    { name: 'empty string becomes undefined', value: '', expected: undefined },
    { name: 'a target port passes through', value: '1234', expected: '1234' },
    {
      name: 'a database name passes through',
      value: 'postgres',
      expected: 'postgres',
    },
  ])('$name', ({ value, expected }) => {
    const gateway = makeAppGateway({ targetSubresourceName: value });
    expect(getTargetSubresourceName(gateway)).toBe(expected);
  });
});

const makeCliCommand = (): GatewayCLICommand => {
  return {
    path: '/Users/foo/Applications/psql.app/MacOS/psql',
    args: ['psql', 'localhost:1337'],
    env: ['foo=bar', 'baz=quux'],
    preview: 'foo=bar baz=quux psql localhost:1337',
  };
};
