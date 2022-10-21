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

import React, { useState } from 'react';
import styled from 'styled-components';
import { Text, Box, Indicator } from 'design';
import * as Icons from 'design/Icon';

import useTeleport from 'teleport/useTeleport';
import {
  SelectCreatable,
  Option,
} from 'teleport/Discover/Shared/SelectCreatable';
import { AccessInfo } from 'teleport/Discover/Shared/AccessInfo';

import {
  Header,
  HeaderSubtitle,
  ActionButtons,
  ButtonBlueText,
} from '../../Shared';

import { useLoginTrait, State } from './useLoginTrait';

import type { AgentStepProps } from '../../types';

const resourceName = 'Kubernetes';
const traitDescription = 'users and groups';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useLoginTrait({ ctx, props });

  return <LoginTrait {...state} />;
}

export function LoginTrait({
  attempt,
  nextStep,
  dynamicTraits,
  staticTraits,
  fetchLoginTraits,
  canEditUser,
  isSsoUser,
}: State) {
  const [groupInputValue, setGroupInputValue] = useState('');
  const [groups, setGroups] = useState<Option[]>([]);

  const [userInputValue, setUserInputValue] = useState('');
  const [users, setUsers] = useState<Option[]>([]);

  React.useEffect(() => {
    if (!dynamicTraits || !staticTraits) return;

    const fixedGroups = staticTraits.groups.map(l => ({
      value: l,
      label: l,
      isFixed: true,
    }));
    const groups = dynamicTraits.groups.map(l => ({
      value: l,
      label: l,
    }));
    setGroups([...fixedGroups, ...groups]);

    const fixedUsers = staticTraits.users.map(l => ({
      value: l,
      label: l,
      isFixed: true,
    }));
    const users = dynamicTraits.users.map(l => ({
      value: l,
      label: l,
    }));
    setUsers([...fixedUsers, ...users]);
  }, [dynamicTraits, staticTraits]);

  const hasUsersOrGroups = groups.length > 0 || users.length > 0;
  const canAddLoginTraits = !isSsoUser && canEditUser;

  function onProceed() {
    // Take out duplications and static users and groups.
    const newDynamicUsers = new Set<string>();
    users.forEach(o => {
      if (!staticTraits.users.includes(o.value)) {
        newDynamicUsers.add(o.value);
      }
    });

    const newDynamicGroups = new Set<string>();
    groups.forEach(o => {
      if (!staticTraits.groups.includes(o.value)) {
        newDynamicGroups.add(o.value);
      }
    });

    nextStep({ users: [...newDynamicUsers], groups: [...newDynamicGroups] });
  }

  function handleGroupKeyDown(event: React.KeyboardEvent) {
    if (!groupInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setGroups([
          ...groups,
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
        setUsers([...users, { value: userInputValue, label: userInputValue }]);
        setUserInputValue('');
        event.preventDefault();
    }
  }

  let $content;
  switch (attempt.status) {
    case 'failed':
      $content = (
        <>
          <Text my={3}>
            <Icons.Warning ml={1} mr={2} color="danger" />
            Encountered Error: {attempt.statusText}
          </Text>
          <ButtonBlueText ml={1} onClick={fetchLoginTraits}>
            Retry
          </ButtonBlueText>
        </>
      );
      break;

    case 'processing':
      $content = (
        <Box mt={4} textAlign="center" height="70px" width="300px">
          <Indicator />
        </Box>
      );
      break;

    case 'success':
      if (isSsoUser && !hasUsersOrGroups) {
        $content = (
          <AccessInfo
            accessKind="ssoUserAndNoTraits"
            resourceName={resourceName}
            traitDesc={traitDescription}
          />
        );
      } else if (!canAddLoginTraits && !hasUsersOrGroups) {
        $content = (
          <AccessInfo
            accessKind="noAccessAndNoTraits"
            resourceName={resourceName}
            traitDesc={traitDescription}
          />
        );
      } else {
        $content = (
          <>
            <StyledBox>
              <Box mb={4}>
                Kubernetes Groups
                <SelectCreatable
                  inputValue={groupInputValue}
                  isClearable={groups.some(v => !v.isFixed)}
                  onInputChange={input => setGroupInputValue(input)}
                  onKeyDown={handleGroupKeyDown}
                  placeholder="Start typing groups and press enter"
                  value={groups}
                  isDisabled={!canAddLoginTraits}
                  onChange={(value, action) => {
                    if (action.action === 'clear') {
                      setGroups(
                        staticTraits.groups.map(l => ({
                          label: l,
                          value: l,
                          isFixed: true,
                        }))
                      );
                    } else {
                      setGroups(value || []);
                    }
                  }}
                  options={dynamicTraits.groups.map(l => ({
                    value: l,
                    label: l,
                  }))}
                />
              </Box>
              <Box mb={2}>
                Kubernetes Users
                <SelectCreatable
                  inputValue={userInputValue}
                  isClearable={users.some(v => !v.isFixed)}
                  onInputChange={input => setUserInputValue(input)}
                  onKeyDown={handleUserKeyDown}
                  placeholder="Start typing users and press enter"
                  value={users}
                  isDisabled={!canAddLoginTraits}
                  onChange={(value, action) => {
                    if (action.action === 'clear') {
                      setUsers(
                        staticTraits.users.map(l => ({
                          label: l,
                          value: l,
                          isFixed: true,
                        }))
                      );
                    } else {
                      setUsers(value || []);
                    }
                  }}
                  options={dynamicTraits.users.map(l => ({
                    value: l,
                    label: l,
                  }))}
                />
              </Box>
            </StyledBox>
            {!isSsoUser && !canEditUser && (
              <AccessInfo
                accessKind="noAccessButHasTraits"
                resourceName={resourceName}
                traitDesc={traitDescription}
              />
            )}
            {isSsoUser && (
              <AccessInfo
                accessKind="ssoUserButHasTraits"
                resourceName={resourceName}
                traitDesc={traitDescription}
              />
            )}
          </>
        );
      }

      break;
  }

  return (
    <Box maxWidth="700px">
      <Header>Set Up Access</Header>
      <HeaderSubtitle>
        Allow access from your Kubernetes user and groups to interact with your
        Kubernetes Clusters.
      </HeaderSubtitle>
      <>
        <Box mb={3}>{$content}</Box>
        <ActionButtons
          onProceed={onProceed}
          disableProceed={
            attempt.status === 'failed' ||
            attempt.status === 'processing' ||
            !hasUsersOrGroups
          }
        />
      </>
    </Box>
  );
}

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 20px;
`;
