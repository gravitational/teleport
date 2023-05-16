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

import React, { useState, useEffect } from 'react';
import { Box } from 'design';

import {
  SelectCreatable,
  Option,
} from 'teleport/Discover/Shared/SelectCreatable';
import {
  useUserTraits,
  SetupAccessWrapper,
} from 'teleport/Discover/Shared/SetupAccess';

import type { AgentStepProps } from '../../types';
import type { State } from 'teleport/Discover/Shared/SetupAccess';

export default function Container(props: AgentStepProps) {
  const state = useUserTraits(props);
  return <SetupAccess {...state} />;
}

export function SetupAccess(props: State) {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    ...restOfProps
  } = props;
  const [groupInputValue, setGroupInputValue] = useState('');
  const [selectedGroups, setSelectedGroups] = useState<Option[]>([]);

  const [userInputValue, setUserInputValue] = useState('');
  const [selectedUsers, setSelectedUsers] = useState<Option[]>([]);

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
    onProceed({ kubeGroups: selectedGroups, kubeUsers: selectedUsers });
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
    >
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
