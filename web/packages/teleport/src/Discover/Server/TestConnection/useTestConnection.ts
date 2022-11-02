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

import { useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import { openNewTab } from 'teleport/lib/util';
import TeleportContext from 'teleport/teleportContext';

import { NodeMeta } from '../../useDiscover';

import type { ConnectionDiagnostic } from 'teleport/services/agents';
import type { AgentStepProps } from '../../types';

export function useTestConnection({ ctx, props }: Props) {
  const { attempt, run } = useAttempt('');
  const [diagnosis, setDiagnosis] = useState<ConnectionDiagnostic>();

  const access = ctx.storeUser.getConnectionDiagnosticAccess();
  const canTestConnection = access.create && access.edit && access.read;

  function startSshSession(login: string) {
    const meta = props.agentMeta as NodeMeta;
    const url = cfg.getSshConnectRoute({
      clusterId: ctx.storeUser.getClusterId(),
      serverId: meta.node.id,
      login,
    });

    openNewTab(url);
  }

  function runConnectionDiagnostic(login: string) {
    const meta = props.agentMeta as NodeMeta;
    setDiagnosis(null);
    run(() =>
      ctx.agentService
        .createConnectionDiagnostic({
          resourceKind: 'node',
          resourceName: meta.node.hostname,
          sshPrincipal: login,
        })
        .then(setDiagnosis)
    );
  }

  return {
    attempt,
    startSshSession,
    logins: sortLogins((props.agentMeta as NodeMeta).node.sshLogins),
    runConnectionDiagnostic,
    diagnosis,
    nextStep: props.nextStep,
    prevStep: props.prevStep,
    canTestConnection,
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

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useTestConnection>;
