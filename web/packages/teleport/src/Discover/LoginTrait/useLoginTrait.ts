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

import TeleportContext from 'teleport/teleportContext';

import type { User } from 'teleport/services/user';
import type { AgentStepProps } from '../types';
import type { NodeMeta } from '../useDiscover';

export function useLoginTrait({ ctx, props }: Props) {
  const [user, setUser] = useState<User>();
  const { attempt, run, setAttempt, handleError } = useAttempt('processing');
  const [staticLogins, setStaticLogins] = useState<string[]>([]);
  const [dynamicLogins, setDynamicLogins] = useState<string[]>([]);

  useEffect(() => {
    run(() =>
      ctx.userService.fetchUser(ctx.storeUser.getUsername()).then(user => {
        setUser(user);

        // Filter out dynamic logins from the node's 'sshLogins'
        // which contain both dynamic and static logins.
        const meta = props.agentMeta as NodeMeta;
        const userDefinedLogins = user.traits.logins;
        const filteredStaticLogins = meta.node.sshLogins.filter(
          login => !userDefinedLogins.includes(login)
        );

        setStaticLogins(filteredStaticLogins);
        setDynamicLogins(userDefinedLogins);
      })
    );
  }, []);

  async function nextStep(logins: string[]) {
    // Currently, fetching user for user traits does not
    // include the statically defined OS usernames, so
    // we combine it manually in memory with the logins
    // we previously fetched through querying for the
    // the newly added resource (which returns 'sshLogins' that
    // includes both dynamic + static logins).
    const meta = props.agentMeta as NodeMeta;
    props.updateAgentMeta({
      ...meta,
      node: { ...meta.node, sshLogins: [...staticLogins, ...logins] },
    });

    // Update the dynamic logins for the user in backend.
    setAttempt({ status: 'processing' });
    try {
      await ctx.userService.updateUser({
        ...user,
        traits: { ...user.traits, logins },
      });

      await ctx.userService.applyUserTraits();
      props.nextStep();
    } catch (err) {
      handleError(err);
    }
  }

  function addLogin(newLogin: string) {
    setDynamicLogins([...dynamicLogins, newLogin]);
  }

  return {
    attempt,
    nextStep,
    dynamicLogins,
    staticLogins,
    addLogin,
  };
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

export type State = ReturnType<typeof useLoginTrait>;
