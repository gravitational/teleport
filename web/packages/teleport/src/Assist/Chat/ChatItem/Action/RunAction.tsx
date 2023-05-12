/*
 *
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

import React, { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { useParams } from 'react-router';

import useStickyClusterId from 'teleport/useStickyClusterId';
import { getAccessToken, getHostName } from 'teleport/services/api';

import { ExecuteRemoteCommandContent } from 'teleport/Assist/services/messages';
import { MessageTypeEnum, Protobuf } from 'teleport/lib/term/protobuf';
import { Dots } from 'teleport/Assist/Dots';
import cfg from 'teleport/config';

interface RunCommandProps {
  actions: ExecuteRemoteCommandContent;
}

function convertContentToCommand(message: ExecuteRemoteCommandContent) {
  const command = {
    command: '',
    login: '',
    query: '',
  };

  if (message.selectedLogin) {
    command.login = message.selectedLogin;
  }

  if (message.command) {
    command.command = message.command;
  }

  if (message.query) {
    command.query = message.query;
  }

  return command;
}

enum RunActionStatus {
  Connecting,
  Finished,
}

interface NodeState {
  nodeId: string;
  status: RunActionStatus;
  stdout?: string;
}

interface RawPayload {
  node_id: string;
  payload: string;
}

export function RunCommand(props: RunCommandProps) {
  const { clusterId } = useStickyClusterId();
  const urlParams = useParams<{ conversationId: string }>();

  const [state, setState] = useState(() => []);

  const params = convertContentToCommand(props.actions);

  const execParams = {
    ...params,
    conversation_id: urlParams.conversationId,
    execution_id: crypto.randomUUID(),
  };

  const url = cfg.getAssistExecuteCommandUrl(
    getHostName(),
    clusterId,
    getAccessToken(),
    execParams
  );

  const websocket = useRef<WebSocket>(null);
  const protoRef = useRef<any>(null);

  useEffect(() => {
    if (!websocket.current) {
      const proto = new Protobuf();
      const ws = new WebSocket(url);
      ws.binaryType = 'arraybuffer';

      ws.onmessage = event => {
        const uintArray = new Uint8Array(event.data);
        const msg = proto.decode(uintArray);

        switch (msg.type) {
          case MessageTypeEnum.RAW:
            const data = JSON.parse(msg.payload) as RawPayload;
            const payload = atob(data.payload);

            setState(state => {
              const results = state.find(node => node.nodeId == data.node_id);
              if (!results) {
                state.push({
                  nodeId: data.node_id,
                  status: RunActionStatus.Connecting,
                });
              }

              const s = state.map(item => {
                if (item.nodeId === data.node_id) {
                  if (!item.stdout) {
                    item.stdout = '';
                  }
                  return {
                    ...item,
                    status: RunActionStatus.Finished,
                    stdout: item.stdout + payload,
                  };
                }

                return item;
              });

              return s;
            });

            break;
        }
      };

      protoRef.current = proto;
      websocket.current = ws;
    }
  }, []);

  const nodes = state.map((item, index) => (
    <NodeOutput key={index} state={item} />
  ));

  return <div style={{ marginTop: '40px' }}>{nodes}</div>;
}

interface NodeOutputProps {
  state: NodeState;
}

const NodeContainer = styled.div`
  margin-bottom: 20px;
  background: rgba(255, 255, 255, 0.07);
  border-radius: 5px;
  padding: 10px 15px 10px;
`;

const NodeTitle = styled.div`
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 10px;
`;

const NodeContent = styled.div`
  background: #020308;
  margin-bottom: 10px;
  min-width: 500px;
  border-radius: 5px;
  padding: 1px 20px;
  color: white;
`;

const LoadingContainer = styled.div`
  display: flex;
  justify-content: center;
  margin: 30px 0 20px;
`;

function NodeOutput(props: NodeOutputProps) {
  return (
    <NodeContainer>
      <NodeTitle>{props.state.nodeId}</NodeTitle>

      {props.state.status === RunActionStatus.Connecting && (
        <LoadingContainer>
          <Dots />
        </LoadingContainer>
      )}

      {props.state.stdout && (
        <NodeContent>
          <pre>{props.state.stdout}</pre>
        </NodeContent>
      )}
    </NodeContainer>
  );
}
