/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContext from 'teleport/teleportContext';
import cfg from 'teleport/config';

import type { JoinToken, JoinRule } from 'teleport/services/joinToken';

export default function useAddNode(ctx: TeleportContext) {
  const { attempt, run } = useAttempt('');
  const { attempt: iamAttempt, run: iamRun } = useAttempt('');
  const isEnterprise = ctx.isEnterprise;
  const version = ctx.storeUser.state.cluster.authVersion;
  const user = ctx.storeUser.state.username;
  const isAuthTypeLocal = !ctx.storeUser.isSso();
  const [method, setMethod] = useState<JoinMethod>('iam');
  const [token, setToken] = useState<JoinToken>();
  const [iamJoinToken, setIamJoinToken] = useState<JoinToken>();

  function createJoinToken() {
    return run(() =>
      ctx.joinTokenService.fetchJoinToken(['Node'], 'token').then(setToken)
    );
  }

  function createIamJoinToken(rules: JoinRule) {
    return iamRun(() =>
      ctx.joinTokenService
        .fetchJoinToken(['Node'], 'iam', [rules])
        .then(setIamJoinToken)
    );
  }

  return {
    isEnterprise,
    createJoinToken,
    method,
    setMethod,
    attempt,
    version,
    user,
    isAuthTypeLocal,
    token,
    iamJoinToken,
    createIamJoinToken,
    iamAttempt,
  };
}

export function createBashCommand(tokenId: string, method?: JoinMethod) {
  const param = method === 'iam' ? '?method=iam' : '';
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(
    tokenId
  )}${param})"`;
}

export type JoinMethod = 'automatic' | 'manual' | 'iam';

export type State = ReturnType<typeof useAddNode>;
