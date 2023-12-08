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

import { useEffect, useState } from 'react';

import { useTeleport } from 'teleport';

import {
  ResourceKind,
  resourceKindToJoinRole,
} from 'teleport/Discover/Shared/ResourceKind';

import { useDiscover } from '../useDiscover';

import type { ResourceLabel } from 'teleport/services/agents';
import type { JoinMethod, JoinToken } from 'teleport/services/joinToken';

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

export function useJoinTokenSuspender(
  resourceKinds: ResourceKind[],
  suggestedAgentMatcherLabels: ResourceLabel[] = [],
  joinMethod: JoinMethod = 'token'
): {
  joinToken: JoinToken;
  reloadJoinToken: () => void;
} {
  const ctx = useTeleport();
  const { emitErrorEvent } = useDiscover();

  const [, rerender] = useState(0);

  const kindsKey = resourceKinds.sort().join();

  function run() {
    abortController = new AbortController();

    const result: SuspendResult = {
      response: null,
      error: null,
      promise: ctx.joinTokenService
        .fetchJoinToken(
          {
            roles: resourceKinds.map(resourceKindToJoinRole),
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
