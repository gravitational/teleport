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
import { WebauthnAssertionResponse } from 'teleport/services/auth';
import useWebAuthn from 'teleport/lib/useWebAuthn';
import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';
import { Message, MfaJson } from 'teleport/lib/tdp/codec';
import AuthnDialog from 'teleport/components/AuthnDialog';
import { TermEvent } from 'teleport/lib/term/enums';

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

class assistClient extends EventEmitterWebAuthnSender {
  private ws: WebSocket;
  private proto: Protobuf;
  encoder = new window.TextEncoder();

  constructor(
    url: string,
    setState: React.Dispatch<React.SetStateAction<any[]>>
  ) {
    super();

    this.proto = new Protobuf();

    const refWS = useRef<WebSocket>();

    React.useEffect(() => {
      if (refWS.current) {
        this.ws = refWS.current;
        return;
      }

      this.ws = new WebSocket(url);
      this.ws.binaryType = 'arraybuffer';
      refWS.current = this.ws;

      const proto = new Protobuf();

      this.ws.onmessage = event => {
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
          case MessageTypeEnum.ERROR:
            console.error(msg.payload);
            break;
          case MessageTypeEnum.WEBAUTHN_CHALLENGE:
            //TODO: handle webauthn challenge
            console.log(msg.payload);

            this.emit(TermEvent.WEBAUTHN_CHALLENGE, msg.payload);

            break;
        }
      };
    }, [refWS.current]);
  }

  sendWebAuthn(data: WebauthnAssertionResponse) {
    console.log('sendWebAuthn', data);
    const msg = this.encodeMfaJson({
      mfaType: 'n',
      jsonString: JSON.stringify(data),
    });
    this.send(msg);
  }

  send(data) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN || !data) {
      console.log('websocket unavailable', this.ws, data);
      return;
    }

    console.log('send', data);
    const msg = this.proto.encodeRawMessage(data);
    const bytearray = new Uint8Array(msg);
    this.ws.send(bytearray.buffer);
  }

  // | message type (10) | mfa_type byte | message_length uint32 | json []byte
  encodeMfaJson(mfaJson: MfaJson): Message {
    const dataUtf8array = this.encoder.encode(mfaJson.jsonString);
    return dataUtf8array;
  }
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

  const assistClt = new assistClient(url, setState);
  const webauthn = useWebAuthn(assistClt);

  const nodes = state.map((item, index) => (
    <NodeOutput key={index} state={item} />
  ));

  return (
    <>
      {webauthn.requested && (
        <AuthnDialog
          onContinue={webauthn.authenticate}
          onCancel={() => {}}
          errorText={webauthn.errorText}
        />
      )}
      <div style={{ marginTop: '40px' }}>{nodes}</div>
    </>
  );
}

interface NodeOutputProps {
  state: NodeState;
}

const NodeContainer = styled.div`
  margin-bottom: 20px;
  background: ${p => p.theme.colors.spotBackground[0]};
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
