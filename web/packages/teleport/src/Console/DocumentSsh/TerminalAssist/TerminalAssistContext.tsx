/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, {
  createContext,
  PropsWithChildren,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';

import { getHostName } from 'teleport/services/api';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';
import {
  Author,
  ExplanationMessage,
  Message,
  MessageType,
  ServerMessage,
  SuggestedCommandMessage,
  UserMessage,
} from 'teleport/Console/DocumentSsh/TerminalAssist/types';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';

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

  const socketRef = useRef<AuthenticatedWebSocket | null>(null);
  const socketUrl = cfg.getAssistActionWebSocketUrl(
    getHostName(),
    clusterId,
    'ssh-cmdgen'
  );

  const [loading, setLoading] = useState(false);

  // note: we store messages in reverse order so that we can use flex-direction: column-reverse
  // to automatically scroll to the bottom of the list
  const [messages, setMessages] = useState<Message[]>([]);

  useEffect(() => {
    socketRef.current = new AuthenticatedWebSocket(socketUrl);

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
      'ssh-explain'
    );

    const ws = new AuthenticatedWebSocket(socketUrl);

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
