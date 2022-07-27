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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import { DiscoverContext } from '../discoverContext';
import { AgentStepProps } from '../types';

import type { JoinToken } from 'teleport/services/joinToken';

export function useDownloadScript({ ctx, props }: Props) {
  const { attempt, run } = useAttempt('');
  const [joinToken, setJoinToken] = useState<JoinToken>();

  useEffect(() => {
    run(() =>
      ctx.joinTokenService.fetchJoinToken(['Node'], 'token').then(setJoinToken)
    );
  }, []);

  return {
    attempt,
    joinToken,
    nextStep: props.nextStep,
  };
}

type Props = {
  ctx: DiscoverContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useDownloadScript>;
