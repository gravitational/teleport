import React, { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import useStickyClusterId from 'teleport/useStickyClusterId';
import { getAccessToken, getHostName } from 'teleport/services/api';

import { ExecuteRemoteCommandContent } from 'teleport/Assist/services/messages';
import { MessageTypeEnum, Protobuf } from 'teleport/lib/term/protobuf';
import { Dots } from 'teleport/Assist/Dots';

interface RunCommandProps {
  actions: ExecuteRemoteCommandContent;
}

function convertContentToCommand(message: ExecuteRemoteCommandContent) {
  const command = {
    command: '',
    login: '',
    query: '',
  };

  if (message.login) {
    command.login = message.login;
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

  const [state, setState] = useState(() => []);

  const params = convertContentToCommand(props.actions);

  const search = new URLSearchParams();

  search.set('access_token', getAccessToken());
  search.set('params', JSON.stringify(params));

  const url = `wss://${getHostName()}/v1/webapi/command/${clusterId}/execute?${search.toString()}`;

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
          case MessageTypeEnum.SESSION_DATA:
            break;

          case MessageTypeEnum.RAW:
            const data = JSON.parse(msg.payload) as RawPayload;
            const payload = atob(data.payload);

            console.log('hello');
            console.log(payload);
            setState(state => {
              console.log('state!!!', state);

              const results = state.find(node => node.nodeId == data.node_id);
              if (!results) {
                state.push({
                  nodeId: data.node_id,
                  status: RunActionStatus.Connecting,
                });
              }

              const s = state.map(item => {
                console.log(
                  'item.nodeId',
                  item.nodeId,
                  'data.node_id',
                  data.node_id
                );
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

              console.log(s);

              return s;
            });

            break;

          default:
            console.log(msg);

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
  background: #0a0e31;
  border-radius: 5px;
  padding: 10px 15px;
`;

const NodeTitle = styled.div`
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 10px;
`;

const NodeContent = styled.div`
  background: #020308;
  margin-bottom: 20px;
  border-radius: 5px;
  padding: 1px 20px;
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
