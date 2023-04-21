/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { ReactElement, useCallback, useState } from 'react';
import styled from 'styled-components';

import { UserIcon } from 'design/SVGIcon';

import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';

import { SearchIcon } from 'teleport/Assist/Icons/SearchIcon';

import { EditIcon } from '../../../Icons/EditIcon';

import { ActionForm } from './ActionForm';
import { Container, Items, Title } from './common';

interface ActionProps {
  state: ActionState[];
  onStateUpdate: (actionState: ActionState[]) => void;
}

const Item = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  margin-right: 10px;
  font-size: 16px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
  font-weight: bold;
`;

const Buttons = styled.div`
  position: absolute;
  right: 8px;
  top: 8px;
  opacity: 0.6;
`;

const EditButton = styled.div`
  border-radius: 10px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;

  &:hover {
    background: rgba(255, 255, 255, 0.2);
  }
`;

const Node = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  margin-right: 10px;
  font-size: 16px;
  font-weight: bold;
  display: flex;
  align-items: center;
  display: flex;

  svg {
    margin-right: 10px;
  }
`;

const As = styled.div`
  padding: 10px 15px;
`;

const User = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  display: flex;
  align-items: center;
  font-weight: bold;

  svg {
    margin-right: 10px;
  }
`;

function actionStateToItems(formState: ActionState[]) {
  const items = [] as ReactElement[];

  for (const [index, state] of formState.entries()) {
    if (state.type === 'command') {
      items.push(<Item key={0}>{state.value}</Item>);
    }

    if (state.type === 'query') {
      items.push(
        <Node key={`query-${index}`}>
          <SearchIcon size={16} />
          {state.value}
        </Node>
      );
    }

    if (state.type === 'user') {
      items.push(
        <As key="as">as</As>,
        <User key="user">
          <UserIcon size={16} /> {state.value}
        </User>
      );
    }
  }

  return items;
}

export function Action(props: ActionProps) {
  const [editing, setEditing] = useState(false);

  const handleSave = useCallback(
    (state: ActionState[]) => {
      props.onStateUpdate(state);
      setEditing(false);
    },
    [props.onStateUpdate]
  );

  if (editing) {
    return (
      <ActionForm
        initialState={props.state}
        onSave={handleSave}
        onCancel={() => setEditing(false)}
      />
    );
  }

  const items = actionStateToItems(props.state);

  return (
    <Container>
      {!editing && (
        <Buttons>
          <EditButton onClick={() => setEditing(true)}>
            <EditIcon size={18} />
          </EditButton>
        </Buttons>
      )}

      <Items>{items}</Items>
    </Container>
  );
}

interface NodesAndLabelsProps {
  initialQuery: string | undefined;
  login: string | undefined;
  onStateUpdate: (state: ActionState[]) => void;
  disabled: boolean;
}

function propsToState(props: NodesAndLabelsProps): ActionState[] {
  const items: ActionState[] = [];

  // Always include query.
  items.push({ type: 'query', value: props.initialQuery ?? '' });

  if (props.login) {
    items.push({ type: 'user', value: props.login });
  }

  return items;
}

function stateToItems(formState: ActionState[]) {
  const items = [];

  for (const [index, state] of formState.entries()) {
    if (state.type === 'command') {
      items.push(<Item key={0}>{state.value}</Item>);
    }

    if (state.type === 'query') {
      // TODO(jakule || ryan) replace node with query
      items.push(
        <Node key={`query-${index}`}>
          <SearchIcon size={16} />
          {state.value}
        </Node>
      );
    }

    if (state.type === 'user') {
      items.push(
        <As key="as">as</As>,
        <User key="user">
          <UserIcon size={16} /> {state.value}
        </User>
      );
    }
  }

  return items;
}

export function NodesAndLabels(props: NodesAndLabelsProps) {
  const [editing, setEditing] = useState(false);

  const state = propsToState(props);

  console.log('state', state, props);

  const handleSave = useCallback(
    (state: ActionState[]) => {
      props.onStateUpdate(state);
      setEditing(false);
    },
    [props.onStateUpdate]
  );

  if (editing) {
    return (
      <ActionForm
        initialState={state}
        addNodes={true}
        onSave={handleSave}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <Container>
      <Title>Connect using query</Title>

      {!editing && !props.disabled && (
        <Buttons>
          <EditButton onClick={() => setEditing(true)}>
            <EditIcon size={18} />
          </EditButton>
        </Buttons>
      )}

      <Items>{stateToItems(state)}</Items>
    </Container>
  );
}

interface CommandProps {
  command: string;
  onStateUpdate: (command: string) => void;
  disabled: boolean;
}

export function Command(props: CommandProps) {
  const [editing, setEditing] = useState(false);

  const state: ActionState[] = [{ type: 'command', value: props.command }];

  const handleSave = useCallback(
    (state: ActionState[]) => {
      let command = '';

      for (const item of state) {
        if (item.type === 'command') {
          command = item.value;
        }
      }

      props.onStateUpdate(command);
      setEditing(false);
    },
    [props.onStateUpdate]
  );

  if (editing) {
    return (
      <ActionForm
        initialState={state}
        onSave={handleSave}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <Container>
      <Title>Execute</Title>

      {!editing && !props.disabled && (
        <Buttons>
          <EditButton onClick={() => setEditing(true)}>
            <EditIcon size={18} />
          </EditButton>
        </Buttons>
      )}

      <Items>{stateToItems(state)}</Items>
    </Container>
  );
}
