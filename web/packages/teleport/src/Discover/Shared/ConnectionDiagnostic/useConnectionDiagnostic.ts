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

import useTeleport from 'teleport/useTeleport';
import { useDiscover } from 'teleport/Discover/useDiscover';

import type {
  ConnectionDiagnostic,
  ConnectionDiagnosticRequest,
} from 'teleport/services/agents';
import type { AgentStepProps } from '../../types';

export function useConnectionDiagnostic(props: AgentStepProps) {
  const ctx = useTeleport();

  const { attempt, run } = useAttempt('');
  const [diagnosis, setDiagnosis] = useState<ConnectionDiagnostic>();
  const { emitErrorEvent } = useDiscover();

  const access = ctx.storeUser.getConnectionDiagnosticAccess();
  const canTestConnection = access.create && access.edit && access.read;

  function runConnectionDiagnostic(req: ConnectionDiagnosticRequest) {
    setDiagnosis(null); // reset since user's can re-test connection.
    run(() =>
      ctx.agentService
        .createConnectionDiagnostic(req)
        .then(diag => {
          setDiagnosis(diag);

          // The request may succeed, but the connection
          // test itself can fail:
          if (!diag.success) {
            // Append all possible errors:
            const errors: string[] = [];
            diag.traces.forEach(trace =>
              errors.push(
                `[${trace.traceType}] ${trace.error} (${trace.details})`
              )
            );
            emitErrorEvent(`testing failed: ${errors.join('\n')}`);
          }
        })
        .catch((error: Error) => {
          emitErrorEvent(error.message);
          throw error;
        })
    );
  }

  const { username, authType } = ctx.storeUser.state;

  return {
    attempt,
    runConnectionDiagnostic,
    diagnosis,
    nextStep: () => props.nextStep(),
    prevStep: props.prevStep,
    canTestConnection,
    username,
    authType,
    clusterId: ctx.storeUser.getClusterId(),
  };
}

export type State = ReturnType<typeof useConnectionDiagnostic>;
