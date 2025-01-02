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
import type { JoinToken } from 'teleport/services/joinToken';
import TeleportContext from 'teleport/teleportContext';

export default function useAddApp(ctx: TeleportContext) {
  const { attempt, run } = useAttempt('');
  const user = ctx.storeUser.state.username;
  const version = ctx.storeUser.state.cluster.authVersion;
  const isAuthTypeLocal = !ctx.storeUser.isSso();
  const isEnterprise = ctx.isEnterprise;
  const [automatic, setAutomatic] = useState(isEnterprise);
  const [token, setToken] = useState<JoinToken>();

  useEffect(() => {
    createToken();
  }, []);

  function createToken() {
    return run(() =>
      ctx.joinTokenService.fetchJoinToken({ roles: ['App'] }).then(setToken)
    );
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
  };
}

export type State = ReturnType<typeof useAddApp>;
