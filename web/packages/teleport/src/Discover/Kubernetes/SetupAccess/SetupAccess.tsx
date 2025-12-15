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

import React, { useEffect, useState } from 'react';

import { Box, Text } from 'design';

import {
  Option,
  SelectCreatable,
} from 'teleport/Discover/Shared/SelectCreatable';
import {
  SetupAccessWrapper,
  useUserTraits,
  type State,
} from 'teleport/Discover/Shared/SetupAccess';

export default function Container() {
  const state = useUserTraits();
  return <SetupAccess {...state} />;
}

export function SetupAccess(props: State) {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    agentMeta,
    ...restOfProps
  } = props;
  const [groupInputValue, setGroupInputValue] = useState('');
  const [selectedGroups, setSelectedGroups] = useState<Option[]>([]);

  const [userInputValue, setUserInputValue] = useState('');
  const [selectedUsers, setSelectedUsers] = useState<Option[]>([]);

  const wantAutoDiscover = !!agentMeta.autoDiscovery;
  useEffect(() => {
    if (props.attempt.status === 'success') {
      setSelectedGroups(initSelectedOptions('kubeGroups'));
      setSelectedUsers(initSelectedOptions('kubeUsers'));
    }
  }, [props.attempt.status, initSelectedOptions]);

  function handleGroupKeyDown(event: React.KeyboardEvent) {
    if (!groupInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setSelectedGroups([
          ...selectedGroups,
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
        setSelectedUsers([
          ...selectedUsers,
          { value: userInputValue, label: userInputValue },
        ]);
        setUserInputValue('');
        event.preventDefault();
    }
  }

  function handleOnProceed() {
    let numStepsToIncrement;
    // Skip test connection since test connection currently
    // only supports one resource testing and auto enrolling
    // enrolls resources > 1.
    if (wantAutoDiscover) {
      numStepsToIncrement = 2;
    }
    onProceed(
      { kubeGroups: selectedGroups, kubeUsers: selectedUsers },
      numStepsToIncrement
    );
  }

  const hasTraits = selectedGroups.length > 0 || selectedUsers.length > 0;
  const canAddTraits = !props.isSsoUser && props.canEditUser;
  const headerSubtitle =
    'Allow access from your Kubernetes user and groups to interact with your Kubernetes Clusters.';

  return (
    <SetupAccessWrapper
      {...restOfProps}
      headerSubtitle={headerSubtitle}
      traitKind="Kubernetes"
      traitDescription="users and groups"
      hasTraits={hasTraits}
      onProceed={handleOnProceed}
      wantAutoDiscover={wantAutoDiscover}
    >
      {wantAutoDiscover && (
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
          isClearable={selectedGroups.some(v => !v.isFixed)}
          onInputChange={input => setGroupInputValue(input)}
          onKeyDown={handleGroupKeyDown}
          placeholder="Start typing groups and press enter"
          value={selectedGroups}
          isDisabled={!canAddTraits}
          onChange={(value, action) => {
            if (action.action === 'clear') {
              setSelectedGroups(getFixedOptions('kubeGroups'));
            } else {
              setSelectedGroups(value || []);
            }
          }}
          options={getSelectableOptions('kubeGroups')}
          autoFocus
        />
      </Box>
      <Box mb={2}>
        Kubernetes Users
        <SelectCreatable
          inputValue={userInputValue}
          isClearable={selectedUsers.some(v => !v.isFixed)}
          onInputChange={setUserInputValue}
          onKeyDown={handleUserKeyDown}
          placeholder="Start typing users and press enter"
          value={selectedUsers}
          isDisabled={!canAddTraits}
          onChange={(value, action) => {
            if (action.action === 'clear') {
              setSelectedUsers(getFixedOptions('kubeUsers'));
            } else {
              setSelectedUsers(value || []);
            }
          }}
          options={getSelectableOptions('kubeUsers')}
        />
      </Box>
    </SetupAccessWrapper>
  );
}
