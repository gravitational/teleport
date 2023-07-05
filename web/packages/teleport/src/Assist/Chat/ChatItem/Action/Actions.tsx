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

import { RunIcon } from 'design/SVGIcon';

import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';
import {
  Command,
  NodesAndLabels,
} from 'teleport/Assist/Chat/ChatItem/Action/Action';
import { RunCommand } from 'teleport/Assist/Chat/ChatItem/Action/RunAction';
import useStickyClusterId from 'teleport/useStickyClusterId';

import ErrorMessage from 'teleport/components/AgentErrorMessage';

import { remoteCommandToMessage } from 'teleport/Assist/contexts/messages';

import { ExecuteRemoteCommandContent, Type } from '../../../services/messages';

interface ActionsProps {
  actions: ExecuteRemoteCommandContent;
  scrollTextarea: () => void;
  showRunButton: boolean;
}

const Container = styled.div`
  width: 100%;
  min-width: 500px;
  padding-bottom: 15px;
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
  padding: 5px 15px 5px 10px;
  border-radius: 5px;
  font-weight: bold;
  font-size: 15px;
  align-items: center;
  margin-left: 10px;
  cursor: pointer;

  svg {
    margin-right: 10px;
  }
`;

const ButtonRun = styled(Button)<{ disabled: boolean }>`
  border: 2px solid ${p => (p.disabled ? '#cccccc' : '#20b141')};
  opacity: ${p => (p.disabled ? 0.8 : 1)};
  cursor: ${p => (p.disabled ? 'not-allowed' : 'pointer')};
  color: white;

  &:hover {
    background: ${p => (p.disabled ? 'none' : '#20b141')};
    color: white;

    svg,
    path {
      fill: white;
    }
  }
`;

const Spacer = styled.div`
  text-align: center;
  padding: 10px 0;
  font-size: 14px;
`;

export function Actions(props: ActionsProps) {
  const [running, setRunning] = useState(false);
  const [actions, setActions] = useState({ ...props.actions });
  const { clusterId } = useStickyClusterId();

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
        selectedLogin: '',
        availableLogins: [],
        query: '',
        command: actions.command,
        errorMsg: '',
      };

      for (const item of newActionState) {
        if (item.type == 'query') {
          newActions.query = item.value;
        }

        if (item.type === 'user') {
          newActions.selectedLogin = item.value;
        }

        if (item.type === 'availableUsers') {
          newActions.availableLogins = item.value;
        }
      }

      remoteCommandToMessage(newActions, clusterId).then(e => setActions(e));
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

  return (
    <Container>
      {actions.errorMsg && <ErrorMessage message={actions.errorMsg} />}
      {!result && <Title>Teleport would like to</Title>}

      <NodesAndLabels
        initialQuery={actions.query}
        selectedLogin={actions.selectedLogin}
        availableLogins={actions.availableLogins}
        onStateUpdate={handleSave}
        disabled={running || !props.showRunButton}
      />

      <Spacer>and</Spacer>

      <Command
        command={actions.command}
        onStateUpdate={handleCommandUpdate}
        disabled={running || !props.showRunButton}
      />

      {!result && !running && props.showRunButton && (
        <Buttons>
          <ButtonRun onClick={() => run()}>
            <RunIcon size={30} fill="white" />
            Run
          </ButtonRun>
        </Buttons>
      )}

      {running && <RunCommand actions={actions} />}
    </Container>
  );
}
