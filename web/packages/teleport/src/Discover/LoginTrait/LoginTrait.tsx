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
import {
  Flex,
  ButtonPrimary,
  ButtonText,
  Text,
  Box,
  Indicator,
  Input,
} from 'design';
import { Danger } from 'design/Alert';
import * as Icons from 'design/Icon';

import { Header, ActionButtons } from '../Shared';
import { useDiscoverContext } from '../discoverContextProvider';

import { useLoginTrait, State } from './useLoginTrait';

import type { AgentStepProps } from '../types';

export default function Container(props: AgentStepProps) {
  const ctx = useDiscoverContext();
  const state = useLoginTrait({ ctx, props });

  return <LoginTrait {...state} />;
}

export function LoginTrait({ attempt, nextStep, logins, addLogin }: State) {
  const inputRefs = useRef<HTMLInputElement[]>([]);
  const [newLogin, setNewLogin] = useState('');
  const [showInputBox, setShowInputBox] = useState(false);

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
      $content = <Danger>{attempt.statusText}</Danger>;
      break;

    case 'processing':
      $content = (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
      break;

    case 'success':
      $content = (
        <>
          <Text bold mb={2}>
            Select or Add Linux Principles
          </Text>
          <Box mb={6}>
            {logins.map((login, index) => {
              return (
                <Flex
                  key={index}
                  p={2}
                  mb={1}
                  borderRadius={2}
                  width="300px"
                  alignItems="center"
                  borderColor="colors.primary.light"
                  css={`
                    border: 1px solid
                      ${props => props.theme.colors.primary.light};
                  `}
                >
                  <input
                    type="checkbox"
                    name={login}
                    css={`
                      margin-right: 10px;
                      accent-color: ${props =>
                        props.theme.colors.secondary.main};
                      &:hover {
                        cursor: pointer;
                      }
                    `}
                    ref={el => (inputRefs.current[index] = el)}
                    defaultChecked
                  />
                  <label
                    htmlFor={login}
                    css={`
                      width: 250px;
                      overflow: hidden;
                      text-overflow: ellipsis;
                    `}
                  >
                    {login}
                  </label>
                </Flex>
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
          </Box>
          <ActionButtons onProceed={onProceed} />
        </>
      );
      break;
  }

  return (
    <Box>
      <Header>Set Up Access</Header>
      {$content}
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
    Add New Principle
  </ButtonText>
);
