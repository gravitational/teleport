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

import { arrayStrDiff } from 'teleport/lib/util';
import useTeleport from 'teleport/useTeleport';
import { Option } from 'teleport/Discover/Shared/SelectCreatable';
import { useDiscover } from 'teleport/Discover/useDiscover';

import { ResourceKind } from '../ResourceKind';

import type { DbMeta, KubeMeta, NodeMeta } from 'teleport/Discover/useDiscover';
import type { User, UserTraits } from 'teleport/services/user';
import type { AgentStepProps } from '../../types';

// useUserTraits handles:
//  - retrieving the latest user (for the dynamic traits) from the backend
//  - extracting traits into static traits (role-defined) and dynamic traits (user-defined)
//  - updating user in the backend with the latest dynamic traits
//  - updating the dynamic traits for our in-memory resource meta object
//  - provides utility function that makes data objects (type Option) for react-select component
export function useUserTraits(props: AgentStepProps) {
  const ctx = useTeleport();
  const { emitErrorEvent } = useDiscover();

  const [user, setUser] = useState<User>();
  const { attempt, run, setAttempt, handleError } = useAttempt('processing');

  const isSsoUser = ctx.storeUser.state.authType === 'sso';
  const canEditUser = ctx.storeUser.getUserAccess().edit;
  const dynamicTraits = initUserTraits(user);

  // Filter out static traits from the resource that we
  // queried in a prior step where we discovered the newly connected resource.
  // The resource itself contains traits that define both
  // dynamic (user-defined) and static (role-defined) traits.
  let meta = props.agentMeta;
  let staticTraits = initUserTraits();
  switch (props.resourceSpec.kind) {
    case ResourceKind.Kubernetes:
      const kube = (meta as KubeMeta).kube;
      staticTraits.kubeUsers = arrayStrDiff(
        kube.users,
        dynamicTraits.kubeUsers
      );
      staticTraits.kubeGroups = arrayStrDiff(
        kube.groups,
        dynamicTraits.kubeGroups
      );
      break;

    case ResourceKind.Server:
      const node = (meta as NodeMeta).node;
      staticTraits.logins = arrayStrDiff(node.sshLogins, dynamicTraits.logins);
      break;

    case ResourceKind.Database:
      const db = (meta as DbMeta).db;
      staticTraits.databaseUsers = arrayStrDiff(
        db.users,
        dynamicTraits.databaseUsers
      );
      staticTraits.databaseNames = arrayStrDiff(
        db.names,
        dynamicTraits.databaseNames
      );
      break;

    default:
      throw new Error(
        `useUserTraits.ts:statiTraits: resource kind ${props.resourceSpec.kind} is not handled`
      );
  }

  useEffect(() => {
    fetchUserTraits();
  }, [ctx.storeUser, ctx.userService, run]);

  function fetchUserTraits() {
    run(() =>
      ctx.userService
        .fetchUser(ctx.storeUser.getUsername())
        .then(setUser)
        .catch((error: Error) => {
          emitErrorEvent(`error fetching user traits: ${error.message}`);
          throw error;
        })
    );
  }

  // onProceed deduplicates and removes static traits from the list of traits
  // before updating user in the backend.
  function onProceed(traitOpts: Partial<Record<Trait, Option[]>>) {
    switch (props.resourceSpec.kind) {
      case ResourceKind.Kubernetes:
        const newDynamicKubeUsers = new Set<string>();
        traitOpts.kubeUsers.forEach(o => {
          if (!staticTraits.kubeUsers.includes(o.value)) {
            newDynamicKubeUsers.add(o.value);
          }
        });

        const newDynamicKubeGroups = new Set<string>();
        traitOpts.kubeGroups.forEach(o => {
          if (!staticTraits.kubeGroups.includes(o.value)) {
            newDynamicKubeGroups.add(o.value);
          }
        });

        nextStep({
          kubeUsers: [...newDynamicKubeUsers],
          kubeGroups: [...newDynamicKubeGroups],
        });
        break;

      case ResourceKind.Server:
        const newDynamicLogins = new Set<string>();
        traitOpts.logins.forEach(o => {
          if (!staticTraits.logins.includes(o.value)) {
            newDynamicLogins.add(o.value);
          }
        });

        nextStep({ logins: [...newDynamicLogins] });
        break;

      case ResourceKind.Database:
        const newDynamicDbUsers = new Set<string>();
        traitOpts.databaseUsers.forEach(o => {
          if (!staticTraits.databaseUsers.includes(o.value)) {
            newDynamicDbUsers.add(o.value);
          }
        });

        const newDynamicDbNames = new Set<string>();
        traitOpts.databaseNames.forEach(o => {
          if (!staticTraits.databaseNames.includes(o.value)) {
            newDynamicDbNames.add(o.value);
          }
        });

        nextStep({
          databaseUsers: [...newDynamicDbUsers],
          databaseNames: [...newDynamicDbNames],
        });
        break;

      default:
        throw new Error(
          `useUserTrait.ts:onProceed: resource kind ${props.resourceSpec.kind} is not handled`
        );
    }
  }

  // updateResourceMetaDynamicTraits updates the in memory
  // meta with the updated dynamic traits.
  function updateResourceMetaDynamicTraits(
    newDynamicTraits: Partial<UserTraits>
  ) {
    let meta = props.agentMeta;
    switch (props.resourceSpec.kind) {
      case ResourceKind.Kubernetes:
        const kube = (meta as KubeMeta).kube;
        props.updateAgentMeta({
          ...meta,
          kube: {
            ...kube,
            users: [...staticTraits.kubeUsers, ...newDynamicTraits.kubeUsers],
            groups: [
              ...staticTraits.kubeGroups,
              ...newDynamicTraits.kubeGroups,
            ],
          },
        });
        break;

      case ResourceKind.Server:
        const node = (meta as NodeMeta).node;
        props.updateAgentMeta({
          ...meta,
          node: {
            ...node,
            sshLogins: [...staticTraits.logins, ...newDynamicTraits.logins],
          },
        });
        break;

      case ResourceKind.Database:
        const db = (meta as DbMeta).db;
        props.updateAgentMeta({
          ...meta,
          db: {
            ...db,
            users: [
              ...staticTraits.databaseUsers,
              ...newDynamicTraits.databaseUsers,
            ],
            names: [
              ...staticTraits.databaseNames,
              ...newDynamicTraits.databaseNames,
            ],
          },
        });
        break;

      default:
        throw new Error(
          `useUserTraits.ts:updateResourceMetaDynamicTraits: resource kind ${props.resourceSpec.kind} is not handled`
        );
    }
  }

  async function nextStep(newDynamicTraits: Partial<UserTraits>) {
    if (isSsoUser || !canEditUser) {
      props.nextStep();
      return;
    }

    // Update resources with the new dynamic traits.
    updateResourceMetaDynamicTraits(newDynamicTraits);
    setAttempt({ status: 'processing' });
    try {
      await ctx.userService
        .updateUser({
          ...user,
          traits: {
            ...user.traits,
            ...newDynamicTraits,
          },
        })
        .catch((error: Error) => {
          emitErrorEvent(`error updating user traits: ${error.message}`);
          throw error;
        });

      await ctx.userService.applyUserTraits().catch((error: Error) => {
        emitErrorEvent(`error applying new user traits: ${error.message}`);
        throw error;
      });

      props.nextStep();
    } catch (err) {
      handleError(err);
    }
  }

  const getSelectableOptions = (trait: Trait) => {
    return initSelectedOptionsHelper({ trait, dynamicTraits });
  };

  function getFixedOptions(trait: Trait): Option[] {
    return initSelectedOptionsHelper({ trait, staticTraits });
  }

  function initSelectedOptions(trait: Trait): Option[] {
    return initSelectedOptionsHelper({ trait, staticTraits, dynamicTraits });
  }

  // Only allow kind database's to be able to go back from
  // this step. The prev screen for databases's atm are either
  // IamPolicy or MutualTls, which is mostly an informational
  // step.
  // For server and kubernetes, the prev screen is the download
  // script which wouldn't make sense to go back to.
  let onPrev;
  if (props.resourceSpec.kind === ResourceKind.Database) {
    onPrev = props.prevStep;
  }

  return {
    attempt,
    onProceed,
    onPrev,
    fetchUserTraits,
    isSsoUser,
    canEditUser,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    dynamicTraits,
    staticTraits,
    resourceSpec: props.resourceSpec,
  };
}

function initUserTraits(user?: User): UserTraits {
  return {
    logins: user?.traits.logins || [],
    databaseUsers: user?.traits.databaseUsers || [],
    databaseNames: user?.traits.databaseNames || [],
    kubeUsers: user?.traits.kubeUsers || [],
    kubeGroups: user?.traits.kubeGroups || [],
    windowsLogins: user?.traits.windowsLogins || [],
    awsRoleArns: user?.traits.awsRoleArns || [],
  };
}

export function initSelectedOptionsHelper({
  trait,
  staticTraits,
  dynamicTraits,
}: {
  trait: Trait;
  staticTraits?: UserTraits;
  dynamicTraits?: UserTraits;
}): Option[] {
  let fixedOptions = [];
  if (staticTraits) {
    fixedOptions = staticTraits[trait].map(l => ({
      value: l,
      label: l,
      isFixed: true,
    }));
  }

  let options = [];
  if (dynamicTraits) {
    options = dynamicTraits[trait].map(l => ({
      value: l,
      label: l,
      isFixed: false,
    }));
  }

  return [...fixedOptions, ...options];
}

type Trait = keyof UserTraits;
export type State = ReturnType<typeof useUserTraits>;
