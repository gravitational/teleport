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

import useAttempt from 'shared/hooks/useAttemptNext';

import { ResourceLabel } from 'teleport/services/agents';
import type { JoinToken, JoinTokenRequest } from 'teleport/services/joinToken';
import { useV1Fallback } from 'teleport/services/version/unsupported';
import TeleportContext from 'teleport/teleportContext';

export default function useAddApp(ctx: TeleportContext) {
  const { attempt, run } = useAttempt('');
  const user = ctx.storeUser.state.username;
  const version = ctx.storeUser.state.cluster.authVersion;
  const isAuthTypeLocal = !ctx.storeUser.isSso();
  const isEnterprise = ctx.isEnterprise;
  const [automatic, setAutomatic] = useState(isEnterprise);
  const [token, setToken] = useState<JoinToken>();
  const [labels, setLabels] = useState<ResourceLabel[]>([]);

  // TODO(kimlisa): DELETE IN 19.0
  const { tryV1Fallback } = useV1Fallback();

  useEffect(() => {
    // We don't want to create token on first render
    // which defaults to the automatic tab because
    // user may want to add labels.
    if (!automatic) {
      setLabels([]);
      // When switching to manual tab, token can be re-used
      // if token was already generated from automatic tab.
      if (!token) {
        createToken();
      }
    }
  }, [automatic]);

  async function fetchJoinToken() {
    const req: JoinTokenRequest = { roles: ['App'], suggestedLabels: labels };
    let resp: JoinToken;
    try {
      resp = await ctx.joinTokenService.fetchJoinTokenV2(req);
    } catch (err) {
      resp = await tryV1Fallback({
        kind: 'create-join-token',
        err,
        req,
        ctx,
      });
    }
    return resp;
  }

  function createToken() {
    return run(() => fetchJoinToken().then(setToken));
  }

  return {
    user,
    version,
    createToken,
    attempt,
    automatic,
    setAutomatic,
    isAuthTypeLocal,
    isEnterprise,
    token,
    labels,
    setLabels,
  };
}

export type State = ReturnType<typeof useAddApp>;
