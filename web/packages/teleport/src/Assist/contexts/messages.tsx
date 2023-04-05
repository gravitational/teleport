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

import { useHistory, useParams } from 'react-router';

import { getAccessToken, getHostName } from 'teleport/services/api';

import { Author, Message, Type } from '../services/messages';

interface MessageContextValue {
  send: (message: string) => Promise<void>;
  messages: Message[];
}

const MessagesContext = createContext<MessageContextValue>({
  messages: [],
  send: () => Promise.resolve(void 0),
});

export function MessagesContextProvider(props: PropsWithChildren<unknown>) {
  const { conversationId } = useParams<{ conversationId: string }>();

  const history = useHistory();

  const [messages, setMessages] = useState<Message[]>([
    {
      author: Author.Teleport,
      content: [
        {
          type: Type.Connect,
          value: ['lb001-production', 'lb002-production', 'lb003-production'],
        },
        {
          type: Type.Exec,
          value: 'free -m | awk \'NR==2{printf "RAM in use: %sMB", $3}\'',
        },
        {
          type: Type.Message,
          value:
            "Oh, you must be psychic to think I can tell you the RAM usage on multiple servers at once. I'm connecting to the production load balancers and you'll have to run my command to get the info. Good luck!",
        },
      ],
    },
  ]);

  let socketUrl = `wss://${getHostName()}/v1/webapi/assistant?access_token=${getAccessToken()}`;
  if (conversationId) {
    socketUrl += `&conversation_id=${conversationId}`;
  }

  const { sendMessage, lastMessage } = useWebSocket(socketUrl);

  useEffect(() => {
    if (lastMessage !== null) {
      const value = JSON.parse(lastMessage.data);

      if (value.conversation_id && !conversationId && !value.type) {
        history.replace(`/web/assist/${value.conversation_id}`);

        return;
      }

      if (value.type === 'CHAT_RESPONSE') {
        setMessages(prev =>
          prev.concat({
            author: Author.User,
            content: [
              {
                type: Type.Message,
                value: atob(value.payload),
              },
            ],
          })
        );
      }
    }
  }, [lastMessage, setMessages, conversationId]);

  const send = useCallback(
    async (message: string) => {
      const newMessages = [
        ...messages,
        {
          author: Author.User,
          content: [{ type: Type.Message, value: message }],
        },
      ];

      setMessages(newMessages);

      sendMessage(message);
    },
    [messages]
  );

  return (
    <MessagesContext.Provider value={{ messages, send }}>
      {props.children}
    </MessagesContext.Provider>
  );
}

export function useMessages() {
  return useContext(MessagesContext);
}
