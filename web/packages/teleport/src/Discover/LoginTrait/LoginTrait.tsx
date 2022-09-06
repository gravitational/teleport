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

import React, { useState, useRef } from 'react';
import styled from 'styled-components';
import {
  Flex,
  ButtonPrimary,
  Text,
  Box,
  Indicator,
  Input,
  ButtonText,
} from 'design';
import * as Icons from 'design/Icon';

import useTeleport from 'teleport/useTeleport';

import {
  Header,
  HeaderSubtitle,
  ActionButtons,
  ButtonBlueText,
} from '../Shared';

import { useLoginTrait, State } from './useLoginTrait';

import type { AgentStepProps } from '../types';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useLoginTrait({ ctx, props });

  return <LoginTrait {...state} />;
}

export function LoginTrait({
  attempt,
  nextStep,
  dynamicLogins,
  staticLogins,
  addLogin,
  fetchLoginTraits,
}: State) {
  const inputRefs = useRef<HTMLInputElement[]>([]);
  const [newLogin, setNewLogin] = useState('');
  const [showInputBox, setShowInputBox] = useState(false);

  const hasLogins = staticLogins.length > 0 || dynamicLogins.length > 0;

  function onAddLogin() {
    addLogin(newLogin);
    setNewLogin('');
    setShowInputBox(false);
  }

  function onProceed() {
    const names: string[] = [];
    inputRefs.current.forEach(el => {
      if (el.checked) {
        names.push(el.name);
      }
    });

    nextStep(names);
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
            Refetch OS Users
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
      $content = (
        <>
          {!hasLogins && (
            <CheckboxWrapper>
              <Text
                css={`
                  font-style: italic;
                  overflow: visible;
                `}
              >
                No OS users added
              </Text>
            </CheckboxWrapper>
          )}
          {/* static logins cannot be modified */}
          {staticLogins.map((login, index) => {
            const id = `${login}${index}`;
            return (
              <CheckboxWrapper key={index} className="disabled">
                <CheckboxInput
                  type="checkbox"
                  name={id}
                  id={id}
                  defaultChecked
                />
                <Label htmlFor={id}>{login}</Label>
              </CheckboxWrapper>
            );
          })}
          {dynamicLogins.map((login, index) => {
            const id = `${login}${index}`;
            return (
              <CheckboxWrapper key={index}>
                <CheckboxInput
                  type="checkbox"
                  name={id}
                  id={id}
                  ref={el => (inputRefs.current[index] = el)}
                  defaultChecked
                />
                <Label htmlFor={id}>{login}</Label>
              </CheckboxWrapper>
            );
          })}
          {showInputBox ? (
            <AddLoginInput
              newLogin={newLogin}
              addLogin={onAddLogin}
              setNewLogin={setNewLogin}
            />
          ) : (
            <AddLoginButton setShowInputBox={setShowInputBox} />
          )}
        </>
      );
      break;
  }

  return (
    <Box maxWidth="700px">
      <Header>Set Up Access</Header>
      <HeaderSubtitle>
        Select the OS users you will use to connect to server.
      </HeaderSubtitle>
      <>
        <Text bold mb={2}>
          Select OS Users
        </Text>
        <Box mb={3}>{$content}</Box>
        <ActionButtons
          onProceed={onProceed}
          disableProceed={
            attempt.status === 'failed' ||
            attempt.status === 'processing' ||
            !hasLogins
          }
        />
      </>
    </Box>
  );
}

const AddLoginInput = ({
  newLogin,
  addLogin,
  setNewLogin,
}: {
  newLogin: string;
  addLogin(): void;
  setNewLogin(l: React.SetStateAction<string>): void;
}) => {
  return (
    <form
      onSubmit={e => {
        e.preventDefault();
        addLogin();
      }}
    >
      <Flex alignItems="end" mt={3}>
        <Input
          placeholder="name"
          autoFocus
          width="200px"
          value={newLogin}
          type="text"
          onChange={e => setNewLogin(e.target.value.trim())}
          mr={3}
          mb={0}
        />
        <ButtonPrimary
          type="submit"
          size="small"
          mb={2}
          disabled={newLogin.length === 0}
        >
          Add
        </ButtonPrimary>
      </Flex>
    </form>
  );
};

const AddLoginButton = ({
  setShowInputBox,
}: {
  setShowInputBox(s: React.SetStateAction<boolean>): void;
}) => (
  <ButtonText
    mt={2}
    onClick={() => setShowInputBox(true)}
    css={`
      line-height: normal;
      padding-left: 4px;
    `}
    autoFocus
  >
    <Icons.Add
      css={`
        font-weight: bold;
        letter-spacing: 4px;
        &:after {
          content: ' ';
        }
      `}
    />
    Add new OS User
  </ButtonText>
);

const CheckboxWrapper = styled(Flex)`
  padding: 8px;
  margin-bottom: 4px;
  width: 300px;
  align-items: center;
  border: 1px solid ${props => props.theme.colors.primary.light};
  border-radius: 8px;

  &.disabled {
    pointer-events: none;
    opacity: 0.5;
  }
`;

const CheckboxInput = styled.input`
  margin-right: 10px;
  accent-color: ${props => props.theme.colors.secondary.main};

  &:hover {
    cursor: pointer;
  }
`;

const Label = styled.label`
  width: 250px;
  overflow: hidden;
  text-overflow: ellipsis;
`;
