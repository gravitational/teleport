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
  const { chatId } = useParams<{ chatId: string }>();

  const history = useHistory();

  const [messages, setMessages] = useState<Message[]>([]);

  let socketUrl = `wss://${getHostName()}/v1/webapi/assistant?access_token=${getAccessToken()}`;
  if (chatId) {
    socketUrl += `&conversation_id=${chatId}`;
  }

  const { sendMessage, lastMessage } = useWebSocket(socketUrl);

  useEffect(() => {
    if (lastMessage !== null) {
      const value = JSON.parse(lastMessage.data);

      if (value.conversation_id) {
        history.replace(`/web/assist/${value.conversation_id}`);

        return;
      }

      setMessages(prev =>
        prev.concat({
          author: Author.Teleport,
          content: [
            {
              type: Type.Message,
              value: value.content,
            },
          ],
        })
      );
    }
  }, [lastMessage, setMessages]);

  const send = useCallback(
    async (message: string) => {
      const newMessages = [...messages, {
        author: Author.User,
        content: [{ type: Type.Message, value: message }],
      }];

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
