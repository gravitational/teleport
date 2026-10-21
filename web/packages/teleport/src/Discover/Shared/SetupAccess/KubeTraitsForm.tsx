/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useState } from 'react';

import { Box, Text } from 'design';

import {
  type Option,
  SelectCreatable,
} from 'teleport/Discover/Shared/SelectCreatable';

type KubeTraitsFormProps = {
  // Selected kube groups.
  kubeGroups: Option[];
  // Selected kube users.
  kubeUsers: Option[];

  // Fixed refers to traits inherited from roles.
  // They cannot be modified/deleted by the user.
  fixedKubeGroups?: Option[];
  fixedKubeUsers?: Option[];

  // Selectables refer to dynamic traits already stored on the user.
  // User's can modify/delete them.
  selectableKubeGroups?: Option[];
  selectableKubeUsers?: Option[];

  disabled?: boolean;
  showAutoDiscoveryHint?: boolean;
  onKubeGroupsChange(groups: Option[]): void;
  onKubeUsersChange(users: Option[]): void;
};

export function KubeTraitsForm({
  kubeGroups,
  kubeUsers,
  fixedKubeGroups = [],
  fixedKubeUsers = [],
  selectableKubeGroups = [],
  selectableKubeUsers = [],
  disabled = false,
  showAutoDiscoveryHint = false,
  onKubeGroupsChange,
  onKubeUsersChange,
}: KubeTraitsFormProps) {
  const [groupInputValue, setGroupInputValue] = useState('');
  const [userInputValue, setUserInputValue] = useState('');

  function handleGroupKeyDown(event: React.KeyboardEvent) {
    if (!groupInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        onKubeGroupsChange([
          ...kubeGroups,
          { value: groupInputValue, label: groupInputValue },
        ]);
        setGroupInputValue('');
        event.preventDefault();
    }
  }

  function handleUserKeyDown(event: React.KeyboardEvent) {
    if (!userInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        onKubeUsersChange([
          ...kubeUsers,
          { value: userInputValue, label: userInputValue },
        ]);
        setUserInputValue('');
        event.preventDefault();
    }
  }

  return (
    <>
      {showAutoDiscoveryHint && (
        <Text mb={3}>
          Since auto-discovery is enabled, make sure to include all Kubernetes
          users and groups that will be used to connect to the discovered
          clusters.
        </Text>
      )}
      <Box mb={4}>
        Kubernetes Groups
        <SelectCreatable
          inputValue={groupInputValue}
          isClearable={kubeGroups.some(v => !v.isFixed)}
          onInputChange={setGroupInputValue}
          onKeyDown={handleGroupKeyDown}
          placeholder="Start typing groups and press enter"
          value={kubeGroups}
          isDisabled={disabled}
          onChange={(value, action) => {
            onKubeGroupsChange(
              action.action === 'clear' ? fixedKubeGroups : value || []
            );
          }}
          options={selectableKubeGroups}
          autoFocus
        />
      </Box>
      <Box mb={2}>
        Kubernetes Users
        <SelectCreatable
          inputValue={userInputValue}
          isClearable={kubeUsers.some(v => !v.isFixed)}
          onInputChange={setUserInputValue}
          onKeyDown={handleUserKeyDown}
          placeholder="Start typing users and press enter"
          value={kubeUsers}
          isDisabled={disabled}
          onChange={(value, action) => {
            onKubeUsersChange(
              action.action === 'clear' ? fixedKubeUsers : value || []
            );
          }}
          options={selectableKubeUsers}
        />
      </Box>
    </>
  );
}
