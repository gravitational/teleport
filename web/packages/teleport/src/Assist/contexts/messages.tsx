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

import { getAccessToken, getHostName } from 'teleport/services/api';

import { Message } from '../services/messages';

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

  const [messages, setMessages] = useState<Message[]>([]);

  const socketUrl = `wss://${getHostName()}/v1/webapi/assistant?auth_token=${getAccessToken()}&conversation_id=${chatId}`;
  const { sendMessage, lastMessage, readyState } = useWebSocket(socketUrl);

  useEffect(() => {
    if (lastMessage !== null) {
      setMessages((prev) => prev.concat(lastMessage.data));
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
