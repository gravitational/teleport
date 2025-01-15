/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useEffect, useState } from 'react';

import { useTeleport } from 'teleport';
import {
  ResourceKind,
  resourceKindToJoinRole,
} from 'teleport/Discover/Shared/ResourceKind';
import type { ResourceLabel } from 'teleport/services/agents';
import type { JoinMethod, JoinToken } from 'teleport/services/joinToken';
import { useV1Fallback } from 'teleport/services/version/unsupported';

import { useDiscover } from '../useDiscover';

interface SuspendResult {
  promise?: Promise<any>;
  response?: JoinToken;
  error?: Error;
}

let abortController: AbortController;
let joinTokenCache = new Map<string, SuspendResult>();

export function clearCachedJoinTokenResult(resourceKinds: ResourceKind[]) {
  joinTokenCache.delete(resourceKinds.sort().join());
}

export function useJoinTokenSuspender({
  resourceKinds,
  suggestedAgentMatcherLabels = [],
  joinMethod = 'token',
  suggestedLabels = [],
}: {
  resourceKinds: ResourceKind[];
  /**
   * labels used for the agent that will be created
   * using a join token (eg: db agent)
   */
  suggestedAgentMatcherLabels?: ResourceLabel[];
  joinMethod?: JoinMethod;
  /**
   * labels for a non-agent resource that will be created
   * using a join token (currently only can be applied to server resource kind).
   */
  suggestedLabels?: ResourceLabel[];
}): {
  joinToken: JoinToken;
  reloadJoinToken: () => void;
} {
  const ctx = useTeleport();
  const { emitErrorEvent } = useDiscover();

  const [, rerender] = useState(0);

  // TODO(kimlisa): DELETE IN 19.0
  const { tryV1Fallback } = useV1Fallback();

  const kindsKey = resourceKinds.sort().join();

  function run() {
    abortController = new AbortController();

    async function fetchJoinToken() {
      const req = {
        roles: resourceKinds.map(resourceKindToJoinRole),
        method: joinMethod,
        suggestedAgentMatcherLabels,
        suggestedLabels,
      };

      let resp: JoinToken;
      try {
        resp = await ctx.joinTokenService.fetchJoinTokenV2(
          req,
          abortController.signal
        );
      } catch (err) {
        resp = await tryV1Fallback({
          kind: 'create-join-token',
          err,
          req,
          abortSignal: abortController.signal,
          ctx,
        });
      }
      return resp;
    }

    const result: SuspendResult = {
      response: null,
      error: null,
      promise: fetchJoinToken()
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

    joinTokenCache.set(kindsKey, result);

    return result;
  }

  useEffect(() => {
    return () => {
      abortController?.abort();
    };
  }, []);

  const existing = joinTokenCache.get(kindsKey);

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

          joinTokenCache.delete(kindsKey);

          rerender(c => c + 1);
        },
      };
    }

    throw existing.promise;
  }

  throw run().promise;
}
