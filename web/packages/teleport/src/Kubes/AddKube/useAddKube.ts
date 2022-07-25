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

import TeleportContext from 'teleport/teleportContext';

import type { JoinToken } from 'teleport/services/joinToken';

export default function useAddKube(ctx: TeleportContext) {
  const { attempt, run } = useAttempt('');
  const [token, setToken] = useState<JoinToken>();
  const version = ctx.storeUser.state.cluster.authVersion;

  function createToken() {
    return run(() =>
      ctx.joinTokenService.fetchJoinToken(['Kube']).then(setToken)
    );
  }

  return {
    createToken,
    attempt,
    token,
    version,
  };
}

export type State = ReturnType<typeof useAddKube>;
