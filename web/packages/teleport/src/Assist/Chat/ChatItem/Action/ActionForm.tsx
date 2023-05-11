/**
 * Copyright 2023 Gravitational, Inc.
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

import React, { useCallback, useState } from 'react';
import styled from 'styled-components';

import { SearchIcon, UserIcon } from 'design/SVGIcon';
import { Cross } from 'design/Icon';

import Select from 'shared/components/Select';

import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';

import { Container, Items } from './common';

interface ActionFormProps {
  initialState: ActionState[];
  onSave: (state: ActionState[]) => void;
  onCancel: () => void;
}

const CommandInput = styled.input`
  padding: 10px 15px;
  background: ${p => p.theme.colors.spotBackground[0]};
  border-radius: 5px;
  font-size: 16px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
  font-weight: bold;
  border: none;
  color: ${p => p.theme.colors.text.main};
  width: 100%;
  box-sizing: border-box;

  &:focus {
    outline: none;
  }
`;

const CancelButton = styled.div`
  font-weight: bold;
  border-radius: 5px;
  padding: 5px 15px;
  display: inline-flex;
  align-self: flex-end;
  cursor: pointer;
  margin-right: 10px;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

const SaveButton = styled.div`
  margin-top: 10px;
  background: ${p => p.theme.colors.buttons.primary.default};
  font-weight: bold;
  color: ${p => p.theme.colors.buttons.primary.text};
  border-radius: 5px;
  padding: 5px 15px;
  display: inline-flex;
  align-self: flex-end;
  cursor: pointer;

  &:hover {
    background: ${p => p.theme.colors.buttons.primary.hover};
  }
`;

const LabelForm = styled.div`
  display: flex;
  background: ${p => p.theme.colors.spotBackground[0]};
  align-items: center;
  padding: 1px 15px;
  border-radius: 5px;
  margin-right: 10px;
`;

const LabelFormContent = styled.div`
  display: flex;
  align-items: center;
`;

const Input = styled.input`
  background: transparent;
  padding: 10px 15px;
  border-radius: 5px;
  margin-right: 10px;
  font-size: 16px;
  font-weight: bold;
  border: none;
  width: 340px;
  box-sizing: border-box;
  color: ${p => p.theme.colors.text.main};

  &:focus {
    outline: none;
  }
`;

const DeleteButton = styled.div`
  padding: 1px 4px;
  border-radius: 5px;
  cursor: pointer;
  justify-self: flex-end;

  &:hover {
    background: rgba(255, 255, 255, 0.2);
  }
`;

const FormFooter = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
`;

const FooterButtons = styled.div``;

const As = styled.div`
  padding: 10px 15px;
  margin-left: -10px;
`;

export function ActionForm(props: ActionFormProps) {
  const currentSelectedUser = props.initialState.find(e => e.type === 'user');
  const [formState, setFormState] = useState<ActionState[]>(props.initialState);
  const [currentUser, setCurrentUser] = useState<string>(
    currentSelectedUser ? (currentSelectedUser.value as string) : ''
  );

  const handleChange = useCallback((index: number, value: any) => {
    setFormState(existing =>
      existing.map((item, i) => {
        if (index === i) {
          return {
            ...item,
            value,
          };
        }

        return item;
      })
    );
  }, []);

  const handleUserChange = useCallback((index: number, value: string) => {
    setCurrentUser(value);
    setFormState(existing =>
      existing.map(item => {
        if (item.type === 'user') {
          return {
            ...item,
            value: value,
          };
        }

        return item;
      }, [])
    );
  }, []);

  const handleDelete = useCallback(index => {
    setFormState(existing => existing.filter((item, i) => i !== index));
  }, []);

  const items = [];

  for (const [index, stateItem] of formState.entries()) {
    if (stateItem.type === 'command') {
      items.push(
        <CommandInput
          autoFocus
          key="command"
          value={stateItem.value}
          onChange={event => handleChange(index, event.target.value)}
        />
      );
    }

    if (stateItem.type === 'query') {
      items.push(
        <LabelForm key={`query-${index}`}>
          <LabelFormContent>
            <SearchIcon size={16} />

            <Input
              key="query"
              value={stateItem.value}
              onChange={event => handleChange(index, event.target.value)}
            />
          </LabelFormContent>

          <DeleteButton onClick={() => handleDelete(index)}>
            <Cross />
          </DeleteButton>
        </LabelForm>
      );
    }

    if (stateItem.type === 'availableUsers') {
      items.push(
        <As key={`as-${index}`}>as</As>,
        <LabelForm key={`user-${index}`}>
          <LabelFormContent>
            <UserIcon size={16} />
            <Select
              onChange={event => handleUserChange(index, event['value'])}
              value={{ value: currentUser, label: currentUser }}
              options={stateItem.value.map(option => {
                return { label: option, value: option };
              })}
              css={'width: 20vh; padding: 5px; margin-left: 10px;'}
            />
          </LabelFormContent>

          <DeleteButton onClick={() => handleDelete(index)}>
            <Cross />
          </DeleteButton>
        </LabelForm>
      );
    }
  }

  return (
    <Container>
      <Items>{items}</Items>

      <FormFooter>
        <FooterButtons>
          <CancelButton onClick={() => props.onCancel()}>Cancel</CancelButton>
          <SaveButton onClick={() => props.onSave(formState)}>Save</SaveButton>
        </FooterButtons>
      </FormFooter>
    </Container>
  );
}
