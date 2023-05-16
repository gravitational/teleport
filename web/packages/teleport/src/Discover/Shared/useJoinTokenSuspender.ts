/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useEffect, useState } from 'react';

import { useTeleport } from 'teleport';

import {
  ResourceKind,
  resourceKindToJoinRole,
} from 'teleport/Discover/Shared/ResourceKind';

import { useDiscover } from '../useDiscover';

import type { AgentLabel } from 'teleport/services/agents';
import type { JoinMethod, JoinToken } from 'teleport/services/joinToken';

interface SuspendResult {
  promise?: Promise<any>;
  response?: JoinToken;
  error?: Error;
}

let abortController: AbortController;
let joinTokenCache = new Map<ResourceKind, SuspendResult>();

export function clearCachedJoinTokenResult(resourceKind: ResourceKind) {
  joinTokenCache.delete(resourceKind);
}

export function useJoinTokenSuspender(
  resourceKind: ResourceKind,
  suggestedAgentMatcherLabels: AgentLabel[] = [],
  joinMethod: JoinMethod = 'token'
): {
  joinToken: JoinToken;
  reloadJoinToken: () => void;
} {
  const ctx = useTeleport();
  const { emitErrorEvent } = useDiscover();

  const [, rerender] = useState(0);

  function run() {
    abortController = new AbortController();

    const result: SuspendResult = {
      response: null,
      error: null,
      promise: ctx.joinTokenService
        .fetchJoinToken(
          {
            roles: [resourceKindToJoinRole(resourceKind)],
            method: joinMethod,
            suggestedAgentMatcherLabels,
          },
          abortController.signal
        )
        .then(token => {
          // Probably will never happen, but just in case, otherwise
          // querying for the resource can return a false positive.
          if (!token.internalResourceId) {
            throw new Error(
              'internal resource ID is required to discover the newly added resource, but none was provided'
            );
          }
          result.response = token;
        })
        .catch((error: Error) => {
          result.error = error;
          emitErrorEvent(`failed to fetch a join token: ${error.message}`);
        }),
    };

    joinTokenCache.set(resourceKind, result);

    return result;
  }

  useEffect(() => {
    return () => {
      abortController?.abort();
    };
  }, []);

  const existing = joinTokenCache.get(resourceKind);

  if (existing) {
    if (existing.error) {
      throw existing.error;
    }

    if (existing.response) {
      return {
        joinToken: existing.response,
        reloadJoinToken() {
          // Delete the cached token and force a rerender
          // so that this hook runs again and creates a new one.

          joinTokenCache.delete(resourceKind);

          rerender(c => c + 1);
        },
      };
    }

    throw existing.promise;
  }

  throw run().promise;
}
