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

import type { User } from 'teleport/services/user';
import type { AgentStepProps } from '../types';

export function useLoginTrait({ ctx, props }: Props) {
  const [user, setUser] = useState<User>();
  const { attempt, run, setAttempt, handleError } = useAttempt('processing');
  const [logins, setLogins] = useState<string[]>([]);

  useEffect(() => {
    run(() =>
      ctx.userService.fetchUser(ctx.username).then(user => {
        setUser(user);
        setLogins(user.traits.logins);
      })
    );
  }, []);

  function nextStep(logins: string[]) {
    setAttempt({ status: 'processing' });
    ctx.userService
      .updateUser({
        ...user,
        traits: { ...user.traits, logins },
      })
      .then(props.nextStep)
      .catch(handleError);
  }

  function addLogin(login: string) {
    setLogins([...logins, login]);
  }

  return {
    attempt,
    nextStep,
    logins,
    addLogin,
  };
}

type Props = {
  ctx: DiscoverContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useLoginTrait>;
