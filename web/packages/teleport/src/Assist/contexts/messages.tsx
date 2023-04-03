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
  send: (message: Message) => Promise<void>;
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

  const { sendMessage, lastMessage, readyState } = useWebSocket(socketUrl);

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
    async (message: Message) => {
      const newMessages = [...messages, message];

      setMessages(newMessages);

      sendMessage(JSON.stringify(newMessages));
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
