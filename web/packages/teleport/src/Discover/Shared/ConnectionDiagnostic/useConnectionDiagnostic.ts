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
import { getDatabaseProtocol } from 'teleport/Discover/Database/resources';

import type {
  ConnectionDiagnostic,
  ConnectionDiagnosticRequest,
} from 'teleport/services/agents';
import type { IsMfaRequiredRequest } from 'teleport/services/mfa';
import type { Database } from 'teleport/Discover/Database/resources';

export function useConnectionDiagnostic() {
  const ctx = useTeleport();

  const { attempt, setAttempt, handleError } = useAttempt('');
  const [diagnosis, setDiagnosis] = useState<ConnectionDiagnostic>();
  const [ranDiagnosis, setRanDiagnosis] = useState(false);
  const { emitErrorEvent, emitEvent, prevStep, nextStep, resourceState } =
    useDiscover();

  const access = ctx.storeUser.getConnectionDiagnosticAccess();
  const canTestConnection = access.create && access.edit && access.read;

  const [showMfaDialog, setShowMfaDialog] = useState(false);

  // runConnectionDiagnostic will initially make a call to check if
  // resource target requires MFA authentication. After this initial
  // check depending on if user successfully authenticated or not (
  // determined by the presence of the token field), will make a call
  // to test connection.
  //
  // Each test connection request will require a MFA check since the
  // fields for the request may have changed eg: for nodes, the login
  // field may change.
  async function runConnectionDiagnostic(
    req: ConnectionDiagnosticRequest,
    privilegeTokenId: string
  ) {
    setDiagnosis(null); // reset since user's can re-test connection.
    setRanDiagnosis(true);
    setShowMfaDialog(false);

    setAttempt({ status: 'processing' });

    try {
      if (!privilegeTokenId) {
        const mfaReq = getMfaRequest(req, resourceState);
        const sessionMfa = await ctx.mfaService.isMfaRequired(mfaReq);
        if (sessionMfa.required) {
          setShowMfaDialog(true);
          return;
        }
      }

      const diag = await ctx.agentService.createConnectionDiagnostic({
        ...req,
        privilegeTokenId,
      });

      setAttempt({ status: 'success' });
      setDiagnosis(diag);

      // The request may succeed, but the connection
      // test itself can fail:
      if (!diag.success) {
        // Append all possible errors:
        const errors: string[] = [];
        diag.traces.forEach(trace => {
          if (trace.status === 'failed') {
            errors.push(
              `[${trace.traceType}] ${trace.error} (${trace.details})`
            );
          }
        });
        emitErrorEvent(`testing failed: ${errors.join('\n')}`);
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

export function getMfaRequest(
  req: ConnectionDiagnosticRequest,
  resourceState: any
) {
  let mfaReq: IsMfaRequiredRequest = {} as any;
  switch (req.resourceKind) {
    case 'node':
      mfaReq = {
        node: {
          login: req.sshPrincipal,
          name: req.resourceName,
        },
      };
      break;
    case 'db':
      const state = resourceState as Database;
      mfaReq = {
        database: {
          serviceName: req.resourceName,
          protocol: getDatabaseProtocol(state.engine),
          name: req.dbTester?.name,
          username: req.dbTester?.user,
        },
      };
      break;
    case 'kube_cluster':
      mfaReq = {
        kube: {
          name: req.resourceName,
        },
      };
      break;
  }

  return mfaReq;
}

export type State = ReturnType<typeof useConnectionDiagnostic>;
