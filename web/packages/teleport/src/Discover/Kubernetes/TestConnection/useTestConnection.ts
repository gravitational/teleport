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
import type { MfaAuthnResponse } from 'teleport/services/mfa';

export function useTestConnection({ ctx, props }: Props) {
  const { attempt, setAttempt, handleError } = useAttempt('');
  const [diagnosis, setDiagnosis] = useState<ConnectionDiagnostic>();
  const [showMfaDialog, setShowMfaDialog] = useState(false);

  const access = ctx.storeUser.getConnectionDiagnosticAccess();
  const canTestConnection = access.create && access.edit && access.read;

  async function runConnectionDiagnostic(
    impersonate: KubeImpersonation,
    mfaAuthnResponse?: MfaAuthnResponse
  ) {
    const meta = props.agentMeta as KubeMeta;
    setDiagnosis(null);
    setShowMfaDialog(false);
    setAttempt({ status: 'processing' });

    try {
      if (!mfaAuthnResponse) {
        const mfaReq = {
          kube: {
            cluster_name: meta.kube.name,
          },
        };
        const sessionMfa = await ctx.mfaService.isMfaRequired(mfaReq);
        if (sessionMfa.required) {
          setShowMfaDialog(true);
          return;
        }
      }

      const diag = await ctx.agentService.createConnectionDiagnostic({
        resourceKind: 'kube_cluster',
        resourceName: meta.kube.name,
        kubeImpersonation: impersonate,
        mfaAuthnResponse,
      });

      setAttempt({ status: 'success' });
      setDiagnosis(diag);
    } catch (err) {
      handleError(err);
    }
  }

  function cancelMfaDialog() {
    setAttempt({ status: '' });
    setShowMfaDialog(false);
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
    showMfaDialog,
    cancelMfaDialog,
  };
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useTestConnection>;
