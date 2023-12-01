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

import { useConnectionDiagnostic } from 'teleport/Discover/Shared';

import { DbMeta } from '../../useDiscover';

import type { AgentStepProps } from '../../types';
import type { MfaAuthnResponse } from 'teleport/services/mfa';

/**
 * @deprecated Refactor Discover/Database/TestConnection away from the container component
 * pattern. See https://github.com/gravitational/teleport/pull/34952.
 */
export function useTestConnection(props: AgentStepProps) {
  const { runConnectionDiagnostic, ...connectionDiagnostic } =
    useConnectionDiagnostic();

  function testConnection(
    { name, user }: { name: string; user: string },
    mfaResponse?: MfaAuthnResponse
  ) {
    runConnectionDiagnostic(
      {
        resourceKind: 'db',
        resourceName: props.agentMeta.resourceName,
        dbTester: {
          name,
          user,
        },
      },
      mfaResponse
    );
  }

  const { engine } = props.resourceSpec.dbMeta;
  return {
    ...connectionDiagnostic,
    testConnection,
    db: (props.agentMeta as DbMeta).db,
    dbEngine: engine,
  };
}

export type State = ReturnType<typeof useTestConnection>;
