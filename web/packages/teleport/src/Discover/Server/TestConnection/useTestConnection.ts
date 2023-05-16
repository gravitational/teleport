/**
 * Copyright 2022 Gravitational, Inc.
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

import cfg from 'teleport/config';
import { openNewTab } from 'teleport/lib/util';
import { useConnectionDiagnostic } from 'teleport/Discover/Shared';

import { NodeMeta } from '../../useDiscover';

import type { AgentStepProps } from '../../types';
import type { MfaAuthnResponse } from 'teleport/services/mfa';

export function useTestConnection(props: AgentStepProps) {
  const { runConnectionDiagnostic, ...connectionDiagnostic } =
    useConnectionDiagnostic();

  function startSshSession(login: string) {
    const meta = props.agentMeta as NodeMeta;
    const url = cfg.getSshConnectRoute({
      clusterId: connectionDiagnostic.clusterId,
      serverId: meta.node.id,
      login,
    });

    openNewTab(url);
  }

  function testConnection(login: string, mfaResponse?: MfaAuthnResponse) {
    runConnectionDiagnostic(
      {
        resourceKind: 'node',
        resourceName: props.agentMeta.resourceName,
        sshPrincipal: login,
      },
      mfaResponse
    );
  }

  return {
    ...connectionDiagnostic,
    testConnection,
    logins: sortLogins((props.agentMeta as NodeMeta).node.sshLogins),
    startSshSession,
  };
}

// sort logins by making 'root' as the first in the list.
const sortLogins = (logins: string[]) => {
  const noRoot = logins.filter(l => l !== 'root').sort();
  if (noRoot.length === logins.length) {
    return logins;
  }
  return ['root', ...noRoot];
};

export type State = ReturnType<typeof useTestConnection>;
