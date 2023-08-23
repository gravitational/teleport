/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { getCliCommandArgs, getCliCommandEnv } from './gateway';
import { GatewayCLICommand } from './types';

describe('getCliCommandArgs', () => {
  it("extracts Node.js-style args from cliCommand's argsList", () => {
    const cliCommand = makeCliCommand();

    const args = getCliCommandArgs(cliCommand);

    expect(args).toEqual([cliCommand.argsList[1]]);
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

const makeCliCommand = (): GatewayCLICommand => {
  return {
    path: '/Users/foo/Applications/psql.app/MacOS/psql',
    argsList: ['psql', 'localhost:1337'],
    envList: ['foo=bar', 'baz=quux'],
    preview: 'foo=bar baz=quux psql localhost:1337',
  };
};
