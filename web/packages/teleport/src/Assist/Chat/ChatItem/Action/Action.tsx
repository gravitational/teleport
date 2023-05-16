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

import { EditIcon, SearchIcon, UserIcon } from 'design/SVGIcon';

import Select from 'shared/components/Select';

import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';

import { ActionForm } from './ActionForm';
import { Container, Items, Title } from './common';

interface ActionProps {
  state: ActionState[];
  onStateUpdate: (actionState: ActionState[]) => void;
}

const Item = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.07);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 5px;
  margin-right: 10px;
  font-size: 14px;
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

const Query = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.07);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 5px;
  align-items: center;
  display: flex;
  margin-right: 10px;
  font-size: 14px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
  font-weight: bold;

  svg {
    margin-right: 10px;
  }
`;

const As = styled.div`
  padding: 10px 15px;
  margin-left: -10px;
`;

const User = styled.div`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.07);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 5px;
  display: flex;
  align-items: center;
  font-weight: bold;

  .react-select--is-disabled {
    .react-select__single-value,
    .react-select__placeholder,
    .react-select__indicator {
      color: rgba(255, 255, 255, 0.7);
    }
  }

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
        <Query key={`query-${index}`}>
          <SearchIcon size={16} />
          {state.value}
        </Query>
      );
    }

    if (state.type === 'availableUsers') {
      items.push(
        <React.Fragment key="user">
          <As>as</As>
          <User>
            <UserIcon size={16} />
            <Select
              onChange={() => {}}
              value={{ value: state.value[0], label: state.value[0] }}
              options={state.value.map(option => {
                return { label: option, value: option };
              })}
              css={'width: 20vh; padding: 5px'}
            />
          </User>
        </React.Fragment>
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
      <Buttons>
        <EditButton onClick={() => setEditing(true)}>
          <EditIcon size={18} />
        </EditButton>
      </Buttons>

      <Items>{items}</Items>
    </Container>
  );
}

interface NodesAndLabelsProps {
  initialQuery: string | undefined;
  selectedLogin: string | undefined;
  availableLogins: string[] | undefined;
  onStateUpdate: (state: ActionState[]) => void;
  disabled: boolean;
}

function propsToState(props: NodesAndLabelsProps): ActionState[] {
  const items: ActionState[] = [];

  // Always include query.
  items.push({ type: 'query', value: props.initialQuery ?? '' });

  if (props.availableLogins) {
    items.push({ type: 'availableUsers', value: props.availableLogins });
  }

  if (props.selectedLogin) {
    items.push({ type: 'user', value: props.selectedLogin });
  }

  return items;
}

function stateToItems(
  updateUser: (state: ActionState[]) => void,
  formState: ActionState[]
) {
  const items = [];

  for (const [index, state] of formState.entries()) {
    if (state.type === 'command') {
      items.push(<Item key={0}>{state.value}</Item>);
    }

    if (state.type === 'query') {
      items.push(
        <Query key={`query-${index}`}>
          <SearchIcon size={16} />
          {state.value}
        </Query>
      );
    }

    const handleChange = event => {
      updateUser([...formState, { type: 'user', value: event.value }]);
    };

    if (state.type === 'user') {
      items.push(
        <React.Fragment key={'user-key'}>
          <As key="as">as</As>
          <User key="user">
            <UserIcon size={16} />
            <Select
              onChange={handleChange}
              value={{ value: state.value, label: state.value }}
              options={[{ value: state.value, label: state.value }]}
              isDisabled={true}
              css={'width: 20vh'}
            />
          </User>
        </React.Fragment>
      );
    }
  }

  return items;
}

export function NodesAndLabels(props: NodesAndLabelsProps) {
  const [editing, setEditing] = useState(false);

  const state = propsToState(props);

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
        onSave={handleSave}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <Container>
      <Title>Connect using query</Title>

      {!props.disabled && (
        <Buttons>
          <EditButton onClick={() => setEditing(true)}>
            <EditIcon size={18} />
          </EditButton>
        </Buttons>
      )}

      <Items>{stateToItems(handleSave, state)}</Items>
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

      {!props.disabled && (
        <Buttons>
          <EditButton onClick={() => setEditing(true)}>
            <EditIcon size={18} />
          </EditButton>
        </Buttons>
      )}

      <Items>{stateToItems(handleSave, state)}</Items>
    </Container>
  );
}
