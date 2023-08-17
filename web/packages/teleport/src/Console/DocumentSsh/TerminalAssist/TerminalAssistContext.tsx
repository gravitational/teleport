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

import React, {
  createContext,
  PropsWithChildren,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';

import { Author, ServerMessage } from 'teleport/Assist/types';
import { getAccessToken, getHostName } from 'teleport/services/api';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';
import {
  ExplanationMessage,
  Message,
  MessageType,
  SuggestedCommandMessage,
  UserMessage,
} from 'teleport/Console/DocumentSsh/TerminalAssist/types';

interface TerminalAssistContextValue {
  close: () => void;
  explainSelection: (selection: string) => void;
  getLastSuggestedCommand: () => string;
  loading: boolean;
  messages: Message[];
  open: () => void;
  send: (message: string) => void;
  visible: boolean;
}

const TerminalAssistContext = createContext<TerminalAssistContextValue>(null);

export function TerminalAssistContextProvider(
  props: PropsWithChildren<unknown>
) {
  const { clusterId } = useStickyClusterId();

  const [visible, setVisible] = useState(false);

  const socketRef = useRef<WebSocket | null>(null);
  const socketUrl = cfg.getAssistActionWebSocketUrl(
    getHostName(),
    clusterId,
    getAccessToken(),
    'ssh-cmdgen'
  );

  const [loading, setLoading] = useState(false);

  // note: we store messages in reverse order so that we can use flex-direction: column-reverse
  // to automatically scroll to the bottom of the list
  const [messages, setMessages] = useState<Message[]>([]);

  useEffect(() => {
    socketRef.current = new WebSocket(socketUrl);

    socketRef.current.onmessage = e => {
      const data = JSON.parse(e.data) as ServerMessage;
      const payload = JSON.parse(data.payload) as {
        action: string;
        input: string;
        reasoning: string;
      };
      const input = JSON.parse(payload.input) as {
        command: string;
      };

      const message: SuggestedCommandMessage = {
        author: Author.Teleport,
        type: MessageType.SuggestedCommand,
        command: input.command,
        reasoning: payload.reasoning,
      };

      setLoading(false);
      setMessages(m => [message, ...m]);
    };
  }, []);

  function close() {
    setVisible(false);
  }

  function open() {
    setVisible(true);
  }

  function explainSelection(selection: string) {
    if (!visible) {
      setVisible(true);
    }

    setLoading(true);

    const encodedOutput = btoa(selection);

    const socketUrl = cfg.getAssistActionWebSocketUrl(
      getHostName(),
      clusterId,
      getAccessToken(),
      'ssh-explain'
    );

    const ws = new WebSocket(socketUrl);

    ws.onopen = () => {
      ws.send(encodedOutput);
    };

    ws.onmessage = event => {
      const message = event.data;
      const msg = JSON.parse(message) as ServerMessage;

      const explanation: ExplanationMessage = {
        author: Author.Teleport,
        type: MessageType.Explanation,
        value: msg.payload,
      };

      setMessages(m => [explanation, ...m]);
      setLoading(false);

      ws.close();
    };
  }

  function send(message: string) {
    setLoading(true);

    const userMessage: UserMessage = {
      author: Author.User,
      type: MessageType.User,
      value: message,
    };

    socketRef.current.send(message);

    setMessages(m => [userMessage, ...m]);
  }

  function getLastSuggestedCommand() {
    if (messages[0] && messages[0].type === MessageType.SuggestedCommand) {
      return (messages[0] as SuggestedCommandMessage).command;
    }
  }

  return (
    <TerminalAssistContext.Provider
      value={{
        close,
        explainSelection,
        getLastSuggestedCommand,
        loading,
        messages,
        open,
        send,
        visible,
      }}
    >
      {props.children}
    </TerminalAssistContext.Provider>
  );
}

export function useTerminalAssist() {
  return useContext(TerminalAssistContext);
}
