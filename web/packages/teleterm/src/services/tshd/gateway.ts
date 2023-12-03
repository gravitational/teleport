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

import { GatewayCLICommand } from './types';

/**
 * getCliCommandArgs returns a Node.js-compatible array with args.
 *
 * In Go, os.exec.Cmd.Args includes argv0 as the first element. Node expects the args array to
 * include just the args.
 */
export function getCliCommandArgs(cliCommand: GatewayCLICommand): string[] {
  const [, ...args] = cliCommand.argsList;
  return args;
}

/**
 * getCliCommandArgv0 returns argv0 from the args list, as documented by os.exec.Cmd.Args.
 * We are safe to use this as the presentational command name.
 */
export function getCliCommandArgv0(cliCommand: GatewayCLICommand): string {
  return cliCommand.argsList[0];
}

/**
 * getCliCommandEnv converts from os.exec.Cmd.Env format to a record of strings that Node expects.
 *
 * ['FOO=bar', 'BAZ=quux'] -> { FOO: 'bar', BAZ: 'quux; }
 */
export function getCliCommandEnv(
  cliCommand: GatewayCLICommand
): Record<string, string> {
  return Object.fromEntries(
    cliCommand.envList.map(nameEqualsValue => nameEqualsValue.split('='))
  );
}
