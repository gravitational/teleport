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

import TeleportContext from 'teleport/teleportContext';

import { KubeMeta } from '../../useDiscover';

import type {
  ConnectionDiagnostic,
  KubeImpersonation,
} from 'teleport/services/agents';
import type { AgentStepProps } from '../../types';

export function useTestConnection({ ctx, props }: Props) {
  const { attempt, run } = useAttempt('');
  const [diagnosis, setDiagnosis] = useState<ConnectionDiagnostic>();

  const access = ctx.storeUser.getConnectionDiagnosticAccess();
  const canTestConnection = access.create && access.edit && access.read;

  function runConnectionDiagnostic(impersonate: KubeImpersonation) {
    const meta = props.agentMeta as KubeMeta;
    setDiagnosis(null);
    run(() =>
      ctx.agentService
        .createConnectionDiagnostic({
          resourceKind: 'kube_cluster',
          resourceName: meta.kube.name,
          kubeImpersonation: impersonate,
        })
        .then(setDiagnosis)
    );
  }

  const { username, authType, cluster } = ctx.storeUser.state;

  return {
    attempt,
    runConnectionDiagnostic,
    diagnosis,
    nextStep: props.nextStep,
    prevStep: props.prevStep,
    canTestConnection,
    kube: (props.agentMeta as KubeMeta).kube,
    username,
    authType,
    clusterId: cluster.clusterId,
  };
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useTestConnection>;
