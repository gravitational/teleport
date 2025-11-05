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

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { Alert, Box, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import {
  TraitsEditor,
  traitsToTraitsOption,
  type TraitsOption,
} from 'shared/components/TraitsEditor';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { useTeleport } from 'teleport';
import { ResourcesResponse } from 'teleport/services/agents';
import auth from 'teleport/services/auth';
import userService, {
  ExcludeUserField,
  ResetToken,
  type CreateUserVariables,
  type User,
} from 'teleport/services/user';
import { GetUsersQueryKey } from 'teleport/services/user/hooks';

import UserTokenLink from './../UserTokenLink';

interface UserAddEditProps {
  user: User;
  isNew: boolean;
  onClose: () => void;
  modifyFetchedData: React.Dispatch<
    React.SetStateAction<ResourcesResponse<User>>
  >;
}

export function UserAddEdit({
  onClose,
  isNew,
  user,
  modifyFetchedData,
}: UserAddEditProps) {
  const ctx = useTeleport();

  const queryClient = useQueryClient();

  const createUser = useMutation({
    mutationFn: async (variables: CreateUserVariables) => {
      const mfaResponse =
        await auth.getMfaChallengeResponseForAdminAction(true);

      const user = await userService.createUser({ ...variables, mfaResponse });

      const token = await ctx.userService.createResetPasswordToken(
        user.name,
        'invite',
        mfaResponse
      );

      return {
        user,
        token,
      };
    },
    onSuccess: ({ token, user }) => {
      setToken(token);

      queryClient.setQueryData(GetUsersQueryKey, previous => {
        if (!previous) {
          return [];
        }

        return [user, ...previous];
      });
    },
  });

  const updateUser = useMutation({
    mutationFn: userService.updateUser,
    onSuccess: data => {
      queryClient.setQueryData(GetUsersQueryKey, previous => {
        if (!previous) {
          return [];
        }

        return [data, ...previous.filter(i => i.name !== data.name)];
      });
    },
  });

  const [name, setName] = useState(user.name);
  const [token, setToken] = useState<ResetToken>(null);
  const [selectedRoles, setSelectedRoles] = useState<Option[]>(
    user.roles.map(r => ({
      value: r,
      label: r,
    }))
  );
  const [configuredTraits, setConfiguredTraits] = useState<TraitsOption[]>(() =>
    traitsToTraitsOption(user.allTraits)
  );

  function onChangeName(name = '') {
    setName(name);
  }

  function onChangeRoles(roles = [] as Option[]) {
    setSelectedRoles(roles);
  }

  async function onSave() {
    const traitsToSave = {};

    for (const traitKV of configuredTraits) {
      traitsToSave[traitKV.traitKey.value] = traitKV.traitValues.map(
        t => t.value
      );
    }

    const u: User = {
      name,
      roles: selectedRoles.map(r => r.value),
      allTraits: traitsToSave,
    };

    if (isNew) {
      const createdUser = (await createUser.mutateAsync({
        user: u,
        excludeUserField: ExcludeUserField.Traits,
      })) as { user: User; token: ResetToken };

      // We have to update the user list on the clientside for the change to be visible immediately
      // without needing to refresh.
      modifyFetchedData(p => {
        return {
          ...p,
          agents: [createdUser.user, ...p.agents],
        };
      });

      return;
    }

    const updatedUser = (await updateUser.mutateAsync({
      user: u,
      excludeUserField: ExcludeUserField.Traits,
    })) as User;

    // We have to update the user on the clientside for the change to be visible immediately
    // without needing to refresh.
    modifyFetchedData(p => {
      const index = p.agents.findIndex(a => a.name === updatedUser.name);
      if (index >= 0) {
        const newUsers = [...p.agents];
        newUsers[index] = updatedUser;
        return {
          ...p,
          agents: newUsers,
        };
      }
    });

    onClose();
  }

  function save(validator) {
    if (!validator.validate()) {
      return;
    }

    onSave();
  }

  async function fetchRoles(search: string): Promise<string[]> {
    const { items } = await ctx.resourceService.fetchRoles({
      search,
      limit: 50,
    });
    return items.map(r => r.name);
  }

  if (createUser.isSuccess && isNew && token) {
    return <UserTokenLink onClose={onClose} token={token} asInvite={true} />;
  }

  const isLoading = createUser.isPending || updateUser.isPending;
  const hasError = createUser.isError || updateUser.isError;
  const errorMessage = createUser.error?.message || updateUser.error?.message;

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({
            maxWidth: '700px',
            width: '100%',
            height: '100%',
            maxHeight: '600px',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <DialogHeader>
            <DialogTitle>{isNew ? 'Create User' : 'Edit User'}</DialogTitle>
          </DialogHeader>
          <DialogContent overflow={'auto'}>
            {hasError && <Alert kind="danger" children={errorMessage} />}
            <Box maxWidth={690}>
              <FieldInput
                mr={2}
                label="Username"
                rule={requiredField('Username is required')}
                placeholder="Username"
                autoFocus
                value={name}
                onChange={e => onChangeName(e.target.value)}
                readonly={!isNew}
                disabled={isLoading}
              />
              <FieldSelectAsync
                mr={2}
                menuPosition="fixed"
                label="User Roles"
                rule={requiredField('At least one role is required')}
                placeholder="Click to select roles"
                isSearchable
                isMulti
                isClearable={false}
                value={selectedRoles}
                onChange={values => onChangeRoles(values as Option[])}
                noOptionsMessage={() => 'No roles found'}
                loadOptions={async input => {
                  const roles = await fetchRoles(input);
                  return roles.map(r => ({ value: r, label: r }));
                }}
                elevated={true}
                isDisabled={isLoading}
              />
              <TraitsEditor
                isLoading={isLoading}
                configuredTraits={configuredTraits}
                setConfiguredTraits={setConfiguredTraits}
              />
            </Box>
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={isLoading}
              onClick={() => save(validator)}
            >
              Save
            </ButtonPrimary>
            <ButtonSecondary disabled={isLoading} onClick={onClose}>
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}
