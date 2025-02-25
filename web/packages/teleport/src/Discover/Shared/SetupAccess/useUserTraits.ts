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

import { Option } from 'teleport/Discover/Shared/SelectCreatable';
import {
  useDiscover,
  type AppMeta,
  type DbMeta,
  type KubeMeta,
  type NodeMeta,
} from 'teleport/Discover/useDiscover';
import { arrayStrDiff } from 'teleport/lib/util';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import {
  ExcludeUserField,
  type User,
  type UserTraits,
} from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import { ResourceKind } from '../ResourceKind';

// useUserTraits handles:
//  - retrieving the latest user (for the dynamic traits) from the backend
//  - extracting traits into static traits (role-defined) and dynamic traits (user-defined)
//  - updating user in the backend with the latest dynamic traits
//  - updating the dynamic traits for our in-memory resource meta object
//  - provides utility function that makes data objects (type Option) for react-select component
export function useUserTraits() {
  const ctx = useTeleport();
  const {
    emitErrorEvent,
    agentMeta,
    resourceSpec,
    updateAgentMeta,
    nextStep: next,
    prevStep,
  } = useDiscover();

  const [user, setUser] = useState<User>();
  const { attempt, run, setAttempt, handleError } = useAttempt('processing');

  const isSsoUser = ctx.storeUser.state.authType === 'sso';
  const canEditUser = ctx.storeUser.getUserAccess().edit;
  const dynamicTraits = initUserTraits(user);
  const wantAutoDiscover = !!agentMeta.autoDiscovery;

  // Filter out static traits from the resource that we
  // queried in a prior step where we discovered the newly connected resource.
  // The resource itself contains traits that define both
  // dynamic (user-defined) and static (role-defined) traits.
  let staticTraits = initUserTraits();
  switch (resourceSpec.kind) {
    case ResourceKind.Kubernetes:
      if (!wantAutoDiscover) {
        const kube = (agentMeta as KubeMeta).kube;
        staticTraits.kubeUsers = arrayStrDiff(
          kube.users,
          dynamicTraits.kubeUsers
        );
        staticTraits.kubeGroups = arrayStrDiff(
          kube.groups,
          dynamicTraits.kubeGroups
        );
      }
      break;

    case ResourceKind.Server:
      if (!wantAutoDiscover) {
        const node = (agentMeta as NodeMeta).node;
        staticTraits.logins = arrayStrDiff(
          node.sshLogins,
          dynamicTraits.logins
        );
      }
      break;

    case ResourceKind.Database:
      if (!wantAutoDiscover) {
        const db = (agentMeta as DbMeta).db;
        staticTraits.databaseUsers = arrayStrDiff(
          db.users,
          dynamicTraits.databaseUsers
        );
        staticTraits.databaseNames = arrayStrDiff(
          db.names,
          dynamicTraits.databaseNames
        );
      }
      break;

    // Note: specific to AWS CLI access
    case ResourceKind.Application:
      if (resourceSpec.appMeta?.awsConsole) {
        const { awsRoles } = (agentMeta as AppMeta).app;
        staticTraits.awsRoleArns = arrayStrDiff(
          awsRoles.map(r => r.arn),
          dynamicTraits.awsRoleArns
        );
        break;
      }
      throw new Error(
        `resource kind is application, but there is no handler defined`
      );

    default:
      throw new Error(`resource kind ${resourceSpec.kind} is not handled`);
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
  function onProceed(
    traitOpts: Partial<Record<Trait, Option[]>>,
    numStepsToIncrement?: number
  ) {
    switch (resourceSpec.kind) {
      case ResourceKind.Kubernetes:
        let newDynamicKubeUsers = new Set<string>();
        traitOpts.kubeUsers.forEach(o => {
          if (!staticTraits.kubeUsers.includes(o.value)) {
            newDynamicKubeUsers.add(o.value);
          }
        });

        let newDynamicKubeGroups = new Set<string>();
        traitOpts.kubeGroups.forEach(o => {
          if (!staticTraits.kubeGroups.includes(o.value)) {
            newDynamicKubeGroups.add(o.value);
          }
        });

        nextStep(
          {
            kubeUsers: [...newDynamicKubeUsers],
            kubeGroups: [...newDynamicKubeGroups],
          },
          numStepsToIncrement
        );
        break;

      case ResourceKind.Server:
        const newDynamicLogins = new Set<string>();
        traitOpts.logins.forEach(o => {
          if (!staticTraits.logins.includes(o.value)) {
            newDynamicLogins.add(o.value);
          }
        });

        nextStep({ logins: [...newDynamicLogins] }, numStepsToIncrement);
        break;

      case ResourceKind.Database:
        let newDynamicDbUsers = new Set<string>();
        traitOpts.databaseUsers.forEach(o => {
          if (!staticTraits.databaseUsers.includes(o.value)) {
            newDynamicDbUsers.add(o.value);
          }
        });

        let newDynamicDbNames = new Set<string>();
        traitOpts.databaseNames.forEach(o => {
          if (!staticTraits.databaseNames.includes(o.value)) {
            newDynamicDbNames.add(o.value);
          }
        });

        nextStep(
          {
            databaseUsers: [...newDynamicDbUsers],
            databaseNames: [...newDynamicDbNames],
          },
          numStepsToIncrement
        );
        break;

      case ResourceKind.Application:
        if (resourceSpec.appMeta?.awsConsole) {
          let newDynamicArns = new Set<string>();
          traitOpts.awsRoleArns.forEach(o => {
            if (!staticTraits.awsRoleArns.includes(o.value)) {
              newDynamicArns.add(o.value);
            }
          });

          nextStep(
            {
              awsRoleArns: [...newDynamicArns],
            },
            numStepsToIncrement
          );
          break;
        }
        throw new Error(
          `resource kind is application, but there is no handler defined`
        );
      default:
        throw new Error(`resource kind ${resourceSpec.kind} is not handled`);
    }
  }

  // updateResourceMetaDynamicTraits updates the in memory
  // meta with the updated dynamic traits.
  function updateResourceMetaDynamicTraits(
    newDynamicTraits: Partial<UserTraits>
  ) {
    let meta = agentMeta;
    switch (resourceSpec.kind) {
      case ResourceKind.Kubernetes:
        const kube = (meta as KubeMeta).kube;
        updateAgentMeta({
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
        updateAgentMeta({
          ...meta,
          node: {
            ...node,
            sshLogins: [...staticTraits.logins, ...newDynamicTraits.logins],
          },
        });
        break;

      case ResourceKind.Database:
        const db = (meta as DbMeta).db;
        updateAgentMeta({
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

      case ResourceKind.Application:
        if (resourceSpec.appMeta?.awsConsole) {
          const app = (meta as AppMeta).app;
          const arns = [
            ...staticTraits.awsRoleArns,
            ...newDynamicTraits.awsRoleArns,
          ];
          const awsRoles = arns.map(arn => {
            const { arnResourceName, awsAccountId } = splitAwsIamArn(arn);
            return {
              name: arnResourceName,
              arn,
              display: arnResourceName,
              accountId: awsAccountId,
            };
          });
          updateAgentMeta({
            ...meta,
            app: {
              ...app,
              awsRoles,
            },
          });
          break;
        }
        throw new Error(
          `resource kind is application, but there is no handler defined`
        );

      default:
        throw new Error(`resource kind ${resourceSpec.kind} is not handled`);
    }
  }

  async function nextStep(
    newDynamicTraits: Partial<UserTraits>,
    numStepsToSkip?: number
  ) {
    if (isSsoUser || !canEditUser) {
      next(numStepsToSkip);
      return;
    }

    // Update resources with the new dynamic traits.
    updateResourceMetaDynamicTraits(newDynamicTraits);
    setAttempt({ status: 'processing' });
    try {
      await ctx.userService
        .updateUser(
          {
            ...user,
            traits: {
              ...user.traits,
              ...newDynamicTraits,
            },
          },
          ExcludeUserField.AllTraits
        )
        .catch((error: Error) => {
          emitErrorEvent(`error updating user traits: ${error.message}`);
          throw error;
        });

      await ctx.userService.reloadUser().catch((error: Error) => {
        emitErrorEvent(`error applying new user traits: ${error.message}`);
        throw error;
      });

      next(numStepsToSkip);
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
  if (
    resourceSpec.kind === ResourceKind.Database &&
    (agentMeta as DbMeta).serviceDeployedMethod !== 'auto'
  ) {
    onPrev = prevStep;
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
    resourceSpec,
    agentMeta,
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
  wantAutoDiscover?: boolean;
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
