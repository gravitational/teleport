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

import { useConnectionDiagnostic } from 'teleport/Discover/Shared';

import { KubeMeta } from '../../useDiscover';

import type { KubeImpersonation } from 'teleport/services/agents';
import type { AgentStepProps } from '../../types';
import type { MfaAuthnResponse } from 'teleport/services/mfa';

export function useTestConnection(props: AgentStepProps) {
  const { runConnectionDiagnostic, ...connectionDiagnostic } =
    useConnectionDiagnostic();

  function testConnection(
    impersonate: KubeImpersonation,
    mfaResponse?: MfaAuthnResponse
  ) {
    runConnectionDiagnostic(
      {
        resourceKind: 'kube_cluster',
        resourceName: props.agentMeta.resourceName,
        kubeImpersonation: impersonate,
      },
      mfaResponse
    );
  }

  return {
    ...connectionDiagnostic,
    testConnection,
    kube: (props.agentMeta as KubeMeta).kube,
  };
}

export type State = ReturnType<typeof useTestConnection>;
