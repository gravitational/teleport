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

import React, { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';

import { RunIcon } from '../../../Icons/RunIcon';
import { ExecuteRemoteCommandContent, Type } from '../../../services/messages';
import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';
import {
  Command,
  NodesAndLabels,
} from 'teleport/Assist/Chat/ChatItem/Action/Action';
import { RunCommand } from 'teleport/Assist/Chat/ChatItem/Action/RunAction';

interface ActionsProps {
  actions: ExecuteRemoteCommandContent;
  scrollTextarea: () => void;
}

const Container = styled.div`
  width: 100%;
  margin-top: 30px;
`;

const Title = styled.div`
  font-size: 15px;
  margin-bottom: 10px;
`;

const Buttons = styled.div`
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
`;

const Button = styled.div`
  display: flex;
  padding: 10px 20px 10px 15px;
  border-radius: 5px;
  font-weight: bold;
  font-size: 18px;
  align-items: center;
  margin-left: 20px;
  cursor: pointer;

  svg {
    margin-right: 5px;
  }
`;

const ButtonRun = styled(Button)<{ disabled: boolean }>`
  border: 2px solid ${p => (p.disabled ? '#cccccc' : '#20b141')};
  opacity: ${p => (p.disabled ? 0.8 : 1)};
  cursor: ${p => (p.disabled ? 'not-allowed' : 'pointer')};

  &:hover {
    background: ${p => (p.disabled ? 'none' : '#20b141')};
  }
`;

const ButtonCancel = styled(Button)`
  color: #e85654;
`;

const Spacer = styled.div`
  text-align: center;
  padding: 10px 0;
  font-size: 14px;
`;

const LoadingContainer = styled.div`
  display: flex;
  justify-content: center;
  margin: 30px 0;
`;

function serverMessageToState(
  message: ExecuteRemoteCommandContent
): ActionState[] {
  const state: ActionState[] = [];

  if (message.nodes) {
    for (const node of message.nodes) {
      state.push({ type: 'node', value: node });
    }
  }

  // if (message.labels) {
  //   for (const label of message.labels) {
  //     state.push({ type: 'label', value: label });
  //   }
  // }

  return state;
}

export function Actions(props: ActionsProps) {
  const [running, setRunning] = useState(false);
  const [actions, setActions] = useState({ ...props.actions });

  console.log(actions);

  const [result] = useState(false);

  useEffect(() => {
    props.scrollTextarea();
  }, [running, props.scrollTextarea]);

  const run = useCallback(async () => {
    if (running) {
      return;
    }

    setRunning(true);
  }, [running]);

  const handleSave = useCallback(
    (newActionState: ActionState[]) => {
      const newActions: ExecuteRemoteCommandContent = {
        type: Type.ExecuteRemoteCommand,
        labels: [],
        nodes: [],
        command: actions.command,
      };

      for (const item of newActionState) {
        if (item.type === 'node') {
          newActions.nodes.push(item.value);
        }

        if (item.type === 'user') {
          newActions.login = item.value;
        }
      }

      setActions(newActions);
    },
    [actions]
  );

  const handleCommandUpdate = useCallback(
    (newCommand: string) => {
      const newActions: ExecuteRemoteCommandContent = {
        ...actions,
        command: newCommand,
      };

      setActions(newActions);
    },
    [actions]
  );

  console.log(actions.command);

  return (
    <Container>
      {!result && <Title>Teleport would like to</Title>}

      <NodesAndLabels
        initialLabels={actions.labels}
        initialNodes={actions.nodes}
        login={actions.login}
        onStateUpdate={handleSave}
        disabled={running}
      />

      <Spacer>and</Spacer>

      <Command command={actions.command} onStateUpdate={handleCommandUpdate} disabled={running} />

      {!result && !running && (
        <Buttons>
          {!running && <ButtonCancel>Cancel</ButtonCancel>}
          <ButtonRun onClick={() => run()}>
            <RunIcon size={30} />
            Run
          </ButtonRun>
        </Buttons>
      )}

      {running && <RunCommand actions={actions} />}
    </Container>
  );
}
