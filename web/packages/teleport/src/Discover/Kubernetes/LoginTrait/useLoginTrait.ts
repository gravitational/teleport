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
import type { AgentStepProps } from '../../types';
import type { KubeMeta } from 'teleport/Discover/useDiscover';

export function useLoginTrait({ ctx, props }: Props) {
  const [user, setUser] = useState<User>();
  const { attempt, run, setAttempt, handleError } = useAttempt('processing');

  const isSsoUser = ctx.storeUser.state.authType === 'sso';
  const canEditUser = ctx.storeUser.getUserAccess().edit;

  const dynamicTraits = {
    users: user?.traits.kubeUsers || [],
    groups: user?.traits.kubeGroups || [],
  };

  // Filter out static traits from the kube resource
  // which contain both dynamic and static traits.
  const meta = props.agentMeta as KubeMeta;
  const staticTraits = {
    users: meta.kube.users.filter(u => !dynamicTraits.users.includes(u)),
    groups: meta.kube.groups.filter(g => !dynamicTraits.groups.includes(g)),
  };

  useEffect(() => {
    fetchLoginTraits();
  }, []);

  function fetchLoginTraits() {
    run(() =>
      ctx.userService.fetchUser(ctx.storeUser.getUsername()).then(setUser)
    );
  }

  // updateKubeMeta updates the meta with updated dynamic traits.
  function updateKubeMeta(dynamicTraits: Traits) {
    props.updateAgentMeta({
      ...meta,
      kube: {
        ...meta.kube,
        users: [...staticTraits.users, ...dynamicTraits.users],
        groups: [...staticTraits.groups, ...dynamicTraits.groups],
      },
    });
  }

  async function nextStep(dynamicTraits: Traits) {
    if (isSsoUser || !canEditUser) {
      props.nextStep();
      return;
    }

    updateKubeMeta(dynamicTraits);

    // Update the dynamic traits for the user in backend.
    setAttempt({ status: 'processing' });
    try {
      await ctx.userService.updateUser({
        ...user,
        traits: {
          ...user.traits,
          kubeUsers: dynamicTraits.users,
          kubeGroups: dynamicTraits.groups,
        },
      });

      await ctx.userService.applyUserTraits();
      props.nextStep();
    } catch (err) {
      handleError(err);
    }
  }

  return {
    attempt,
    nextStep,
    dynamicTraits,
    staticTraits,
    fetchLoginTraits,
    isSsoUser,
    canEditUser,
  };
}

type Props = {
  ctx: TeleportContext;
  props: AgentStepProps;
};

type Traits = {
  users: string[];
  groups: string[];
};

export type State = ReturnType<typeof useLoginTrait>;
