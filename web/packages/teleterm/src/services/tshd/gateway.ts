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
