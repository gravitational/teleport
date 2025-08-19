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

import { useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import {
  getDatabaseProtocol,
  type ResourceSpec,
} from 'teleport/Discover/SelectResource';
import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  agentService,
  type ConnectionDiagnostic,
  type ConnectionDiagnosticRequest,
} from 'teleport/services/agents';
import auth from 'teleport/services/auth/auth';
import type { MfaChallengeResponse } from 'teleport/services/mfa';
import { DiscoverEventStatus } from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

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

  /**
   * runConnectionDiagnostic depending on the value of `mfaAuthnResponse` does the following:
   *   1) If param `mfaAuthnResponse` is undefined or null, it will check if MFA is required.
   *      - If MFA is required, it sets a flag that indicates a users
   *        MFA credentials are required, and skips the request to test connection.
   *      - If MFA is NOT required, it makes the request to test connection.
   *   2) If param `mfaAuthnResponse` is defined, it skips checking if MFA is required,
   *      and makes the request to test connection.
   *
   * The return value can be used within event handlers where you cannot depend on React state.
   */
  async function runConnectionDiagnostic(
    req: ConnectionDiagnosticRequest,
    mfaAuthnResponse?: MfaChallengeResponse
  ): Promise<{ mfaRequired: boolean }> {
    setDiagnosis(null); // reset since user's can re-test connection.
    setRanDiagnosis(true);
    setShowMfaDialog(false);

    setAttempt({ status: 'processing' });

    try {
      if (!mfaAuthnResponse) {
        const mfaReq = getMfaRequest(req, resourceSpec);
        const sessionMfa = await auth.checkMfaRequired(
          ctx.storeUser.getClusterId(),
          mfaReq
        );
        if (sessionMfa.required) {
          setShowMfaDialog(true);
          return { mfaRequired: true };
        }
      }

      const diag = await agentService.createConnectionDiagnostic({
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

    return { mfaRequired: false };
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
    /**
     * @deprecated Get clusterId from resource, for example (agentMeta as NodeMeta).node.clusterId
     * Alternatively, call useTeleport and then ctx.storeUser.getClusterId.
     *
     * Hooks should not reexport values that are already made available by other hooks.
     */
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
