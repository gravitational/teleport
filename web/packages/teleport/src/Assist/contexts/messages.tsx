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
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react';
import useWebSocket from 'react-use-websocket';

import { useParams } from 'react-router';

import api, { getAccessToken, getHostName } from 'teleport/services/api';

import { Author, ExecuteRemoteCommandContent, Message, MessageContent, TextMessageContent, Type } from '../services/messages';

interface MessageContextValue {
  send: (message: string) => Promise<void>;
  messages: Message[];
  loading: boolean;
  responding: boolean;
}

interface ServerMessage {
  conversation_id: string;
  type: string;
  payload: string;
  created_time: string;
}

interface ConversationHistoryResponse {
  Messages: ServerMessage[];
}

const MessagesContext = createContext<MessageContextValue>({
  messages: [],
  send: () => Promise.resolve(void 0),
  loading: true,
  responding: false,
});

interface MessagesContextProviderProps {
  conversationId: string;
}

function convertServerMessage(message: ServerMessage): Message {
  if (message.type === 'CHAT_MESSAGE_ASSISTANT') {
    return {
      author: Author.Teleport,
      content: {
        type: Type.Message,
        value: message.payload,
      },
    };
  }

  if (message.type === 'CHAT_MESSAGE_USER') {
    return {
      author: Author.User,
      content: {
        type: Type.Message,
        value: message.payload,
      },
    };
  }

  if (message.type === 'COMMAND') {
    const execCmd: ExecuteRemoteCommandContent = JSON.parse(message.payload);

    return {
      author: Author.Teleport,
      isNew: true,
      content: {
        ...execCmd,
        type: Type.ExecuteRemoteCommand,
        login: 'root',
      },
    };
  }
}

export function MessagesContextProvider(
  props: PropsWithChildren<MessagesContextProviderProps>
) {
  const { conversationId } = useParams<{ conversationId: string }>();

  const [loading, setLoading] = useState(true);
  const [responding, setResponding] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);

  const socketUrl = `wss://${getHostName()}/v1/webapi/assistant?access_token=${getAccessToken()}&conversation_id=${
    props.conversationId
  }`;

  const { sendMessage, lastMessage } = useWebSocket(socketUrl);

  const load = useCallback(async () => {
    setMessages([]);

    const res = (await api.get(
      `/v1/webapi/assistant/conversations/${props.conversationId}`
    )) as ConversationHistoryResponse;

    if (!res.Messages) {
      return;
    }

    setMessages(res.Messages.map(convertServerMessage));
  }, [props.conversationId]);

  useEffect(() => {
    setLoading(true);

    load().then(() => setLoading(false));
  }, [props.conversationId]);

  useEffect(() => {
    if (lastMessage !== null) {
      const value = JSON.parse(lastMessage.data) as ServerMessage;

      setMessages(prev => prev.concat(convertServerMessage(value)));
      setResponding(false);
    }
  }, [lastMessage, setMessages, conversationId]);

  const send = useCallback(
    async (message: string) => {
      setResponding(true);

      const newMessages = [
        ...messages,
        {
          author: Author.User,
          isNew: true,
          content: { type: Type.Message, value: message } as const,
        },
      ];

      setMessages(newMessages);

      const data = JSON.stringify({ payload: message });
      console.log('data', data);
      sendMessage(data);
    },
    [messages]
  );

  return (
    <MessagesContext.Provider value={{ messages, send, loading, responding }}>
      {props.children}
    </MessagesContext.Provider>
  );
}

export function useMessages() {
  return useContext(MessagesContext);
}
