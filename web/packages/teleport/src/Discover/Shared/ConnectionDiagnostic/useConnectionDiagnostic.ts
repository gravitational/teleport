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
import { DiscoverEventStatus } from 'teleport/services/userEvent';
import auth from 'teleport/services/auth/auth';
import { getDatabaseProtocol } from 'teleport/Discover/SelectResource';

import type {
  ConnectionDiagnostic,
  ConnectionDiagnosticRequest,
} from 'teleport/services/agents';
import type { MfaAuthnResponse } from 'teleport/services/mfa';
import type { ResourceSpec } from 'teleport/Discover/SelectResource';

export function useConnectionDiagnostic() {
  const ctx = useTeleport();

  const { attempt, setAttempt, handleError } = useAttempt('');
  const [diagnosis, setDiagnosis] = useState<ConnectionDiagnostic>();
  const [ranDiagnosis, setRanDiagnosis] = useState(false);
  const { emitErrorEvent, emitEvent, prevStep, nextStep, resourceSpec } =
    useDiscover();

  const access = ctx.storeUser.getConnectionDiagnosticAccess();
  const canTestConnection = access.create && access.edit && access.read;

  const [showMfaDialog, setShowMfaDialog] = useState(false);

  // runConnectionDiagnostic depending on the value of `mfaAuthnResponse` does the following:
  //   1) If param `mfaAuthnResponse` is undefined or null, it will check if MFA is required.
  //      - If MFA is required, it sets a flag that indicates a users
  //        MFA credentials are required, and skips the request to test connection.
  //      - If MFA is NOT required, it makes the request to test connection.
  //   2) If param `mfaAuthnResponse` is defined, it skips checking if MFA is required,
  //      and makes the request to test connection.
  async function runConnectionDiagnostic(
    req: ConnectionDiagnosticRequest,
    mfaAuthnResponse?: MfaAuthnResponse
  ) {
    setDiagnosis(null); // reset since user's can re-test connection.
    setRanDiagnosis(true);
    setShowMfaDialog(false);

    setAttempt({ status: 'processing' });

    try {
      if (!mfaAuthnResponse) {
        const mfaReq = getMfaRequest(req, resourceSpec);
        const sessionMfa = await auth.checkMfaRequired(mfaReq);
        if (sessionMfa.required) {
          setShowMfaDialog(true);
          return;
        }
      }

      const diag = await ctx.agentService.createConnectionDiagnostic({
        ...req,
        mfaAuthnResponse,
      });

      setAttempt({ status: 'success' });
      setDiagnosis(diag);

      // The request may succeed, but the connection
      // test itself can fail:
      if (!diag.success) {
        // Append all possible errors:
        const errors = diag.traces
          .filter(trace => trace.status === 'failed')
          .map(
            trace => `[${trace.traceType}] ${trace.error} (${trace.details})`
          )
          .join('\n');
        emitErrorEvent(`diagnosis returned with errors: ${errors}`);
      } else {
        emitEvent({ stepStatus: DiscoverEventStatus.Success });
      }
    } catch (err) {
      handleError(err);
      emitErrorEvent(err.message);
    }
  }

  function cancelMfaDialog() {
    setAttempt({ status: '' });
    setShowMfaDialog(false);
  }

  const { username, authType } = ctx.storeUser.state;

  return {
    attempt,
    runConnectionDiagnostic,
    diagnosis,
    nextStep: () => {
      if (!ranDiagnosis) {
        emitEvent({ stepStatus: DiscoverEventStatus.Skipped });
      }
      // else either a failed or success event would've been
      // already sent for each test connection, so we don't need
      // to send anything here.
      nextStep();
    },
    prevStep,
    canTestConnection,
    username,
    authType,
    clusterId: ctx.storeUser.getClusterId(),
    showMfaDialog,
    cancelMfaDialog,
  };
}

function getMfaRequest(
  req: ConnectionDiagnosticRequest,
  resourceSpec: ResourceSpec
) {
  switch (req.resourceKind) {
    case 'node':
      return {
        node: {
          login: req.sshPrincipal,
          node_name: req.resourceName,
        },
      };

    case 'db':
      const state = resourceSpec.dbMeta;
      return {
        database: {
          service_name: req.resourceName,
          protocol: getDatabaseProtocol(state.engine),
          name: req.dbTester?.name,
          username: req.dbTester?.user,
        },
      };

    case 'kube_cluster':
      return {
        kube: {
          cluster_name: req.resourceName,
        },
      };
  }
}

export type State = ReturnType<typeof useConnectionDiagnostic>;
