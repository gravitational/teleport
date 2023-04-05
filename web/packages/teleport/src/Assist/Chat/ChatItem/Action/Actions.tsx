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

import React, {
  Children,
  PropsWithChildren,
  ReactNode,
  useCallback,
  useEffect,
  useState,
} from 'react';
import styled from 'styled-components';

import useWebSocket from 'react-use-websocket';

import { RunIcon } from '../../../Icons/RunIcon';
import { ExecOutput, MessageContent } from '../../../services/messages';

import { ExecResult } from './ExecResult';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { getAccessToken, getHostName } from 'teleport/services/api';

interface ActionsProps {
  contents: MessageContent[];
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

export function Actions(props: PropsWithChildren<ActionsProps>) {
  const children: ReactNode[] = [];
  const [running, setRunning] = useState(false);
  const [result] = useState<ExecOutput | null>(null);

  Children.forEach(props.children, (child, index) => {
    children.push(child);
    children.push(<Spacer key={`spacer-${index}`}>and</Spacer>);
  });

  useEffect(() => {
    props.scrollTextarea();
  }, [running, props.scrollTextarea]);

  const run = useCallback(async () => {
    if (running) {
      return;
    }

    setRunning(true);
  }, [props.contents, running]);

  return (
    <Container>
      {!result && <Title>Teleport would like to</Title>}

      {children.slice(0, -1)}

      {!result && (
        <Buttons>
          {!running && <ButtonCancel>Cancel</ButtonCancel>}
          <ButtonRun onClick={() => run()} disabled={running}>
            <RunIcon size={30} />
            {running ? 'Running' : 'Run'}
          </ButtonRun>
        </Buttons>
      )}

      {running && (
        <RunCommand />
      )}
    </Container>
  );
}

interface RunCommandProps {
  command?: string;
  login?: string;
  nodeIds?: string[];
}

function RunCommand(props: RunCommandProps) {
  const { clusterId } = useStickyClusterId();

  const search = new URLSearchParams();

  const params = {
    command: 'ls -al',
    login: 'root',
    node_id: ['fbc3a404-32ac-408c-bc72-174f895a2fe6'],
  };

  search.set('access_token', getAccessToken());
  search.set('params', JSON.stringify(params));

  console.log(search.toString());

  const url = `wss://${getHostName()}/v1/webapi/command/${clusterId}/execute?${search.toString()}`;

  const { lastMessage } = useWebSocket(url);

  console.log(lastMessage);

  return (
    <div>
      hello!
    </div>
  );
}
