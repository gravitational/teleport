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

import React, { useState, useRef, useEffect } from 'react';
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
  Mark,
  ReadOnlyYamlEditor,
} from '../../Shared';
import { loginsAndRuleUsers, logins } from '../../templates';

import { useLoginTrait, State } from './useLoginTrait';

import type { AgentStepProps } from '../../types';

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
  canEditUser,
  isSsoUser,
}: State) {
  const inputRefs = useRef<HTMLInputElement[]>([]);
  const [newLogin, setNewLogin] = useState('');
  const [showInputBox, setShowInputBox] = useState(false);
  const [hasCheckedLogins, setHasCheckedLogins] = useState(false);

  const hasLogins = staticLogins.length > 0 || dynamicLogins.length > 0;
  const canAddLoginTraits = !isSsoUser && canEditUser;

  useEffect(() => {
    setHasCheckedLogins(hasLogins);
  }, [hasLogins]);

  function onAddLogin() {
    addLogin(newLogin);
    setNewLogin('');
    setShowInputBox(false);
    setHasCheckedLogins(true);
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
      if (isSsoUser && !hasLogins) {
        $content = (
          <>
            <Text mt={4} width="100px">
              You don’t have any allowed OS users defined.
              <br />
              Please ask your Teleport administrator to update your role and add
              the required OS users (logins).
            </Text>
            <EditorLogins />
          </>
        );
      } else if (!canAddLoginTraits && !hasLogins) {
        $content = (
          <>
            <Text mt={4} width="100px">
              You don’t have any allowed OS users or permission to add new OS
              users.
              <br />
              Please ask your Teleport administrator to update your role to
              either add the required OS users (logins) or add the{' '}
              <Mark>users</Mark>
              rule:
            </Text>
            <EditorLoginsAndRuleUsers />
          </>
        );
      } else {
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
                    name={login}
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
                <CheckboxWrapper
                  key={index}
                  className={!canAddLoginTraits ? 'disabled' : ''}
                >
                  <CheckboxInput
                    type="checkbox"
                    name={login}
                    id={id}
                    ref={el => (inputRefs.current[index] = el)}
                    defaultChecked
                    onChange={() =>
                      setHasCheckedLogins(
                        staticLogins.length > 0 ||
                          inputRefs.current.some(i => i.checked)
                      )
                    }
                  />
                  <Label htmlFor={id}>{login}</Label>
                </CheckboxWrapper>
              );
            })}
            {canAddLoginTraits && (
              <>
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
            )}
            {!isSsoUser && !canEditUser && (
              <>
                <Text mt={4}>
                  You don't have permission to add new OS users.
                  <br />
                  If you don't see the OS user that you require, please ask your
                  Teleport administrator to update your role to either add the
                  required OS users (logins) or add the <Mark>users</Mark> rule:
                </Text>
                <EditorLoginsAndRuleUsers />
              </>
            )}
            {isSsoUser && (
              <>
                <Text mt={4}>
                  SSO users are not able to add new OS users.
                  <br />
                  If you don't see the OS user that you require, please ask your
                  Teleport administrator to update your role to add the required
                  OS users (logins):
                </Text>
                <EditorLogins />
              </>
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
        Select the OS users you will use to connect to server.
      </HeaderSubtitle>
      <>
        <Box mb={3}>{$content}</Box>
        <ActionButtons
          onProceed={onProceed}
          disableProceed={
            attempt.status === 'failed' ||
            attempt.status === 'processing' ||
            !hasCheckedLogins
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

const EditorLoginsAndRuleUsers = () => (
  <Flex minHeight="185px" mt={3}>
    <ReadOnlyYamlEditor content={loginsAndRuleUsers} />
  </Flex>
);

const EditorLogins = () => (
  <Flex minHeight="115px" mt={3}>
    <ReadOnlyYamlEditor content={logins} />
  </Flex>
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
