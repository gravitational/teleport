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

import {
  GatewayTargetUri,
  isAppUri,
  isDatabaseUri,
  isKubeUri,
  routing,
} from 'teleterm/ui/uri';

import { GatewayCLICommand } from './types';

/**
 * getCliCommandArgs returns a Node.js-compatible array with args.
 *
 * In Go, os.exec.Cmd.Args includes argv0 as the first element. Node expects the args array to
 * include just the args.
 */
export function getCliCommandArgs(cliCommand: GatewayCLICommand): string[] {
  const [, ...args] = cliCommand.args;
  return args;
}

/**
 * getCliCommandArgv0 returns argv0 from the args list, as documented by os.exec.Cmd.Args.
 * We are safe to use this as the presentational command name.
 */
export function getCliCommandArgv0(cliCommand: GatewayCLICommand): string {
  return cliCommand.args[0];
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
    cliCommand.env.map(nameEqualsValue => nameEqualsValue.split('='))
  );
}

/**
 * getTargetNameFromUri extracts the name of the gateway target from the target URI.
 *
 * Defaults to the target URI itself if the target URI doesn't seem to match any of the supported
 * URI types.
 *
 * If possible, the target name should be acquired from the gateway object itself.
 * getTargetNameFromUri is reserved for situations where a gateway is not available, but we still
 * want to display a pretty name in the UI.
 */
export function getTargetNameFromUri(targetUri: GatewayTargetUri): string {
  return (
    routing.parseDbUri(targetUri)?.params['dbId'] ||
    routing.parseKubeUri(targetUri)?.params['kubeId'] ||
    routing.parseAppUri(targetUri)?.params['appId'] ||
    targetUri
  );
}

/**
 * getGatewayTargetUriKind is used when the callsite needs to distinguish between different kinds
 * of targets that gateways support when given only its target URI.
 */
export function getGatewayTargetUriKind(
  targetUri: string
): 'db' | 'kube' | 'app' {
  if (isDatabaseUri(targetUri)) {
    return 'db';
  }

  if (isKubeUri(targetUri)) {
    return 'kube';
  }

  if (isAppUri(targetUri)) {
    return 'app';
  }

  // TODO(ravicious): Optimally we'd use `targetUri satisfies never` here to have a type error when
  // DocumentGateway['targetUri'] is changed.
  //
  // However, at the moment that field is essentially of type string, so there's not much we can do
  // with regards to type safety.
}
