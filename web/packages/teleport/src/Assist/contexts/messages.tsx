import React, {
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react';

import { getAccessToken, getHostName } from 'teleport/services/api';

import { Message } from '../services/messages';

interface MessageContextValue {
  send: (message: Message) => Promise<void>;
  messages: Message[];
  streaming: boolean;
}

const MessagesContext = createContext<MessageContextValue>({
  messages: [],
  send: () => Promise.resolve(void 0),
  streaming: false,
});

export function MessagesContextProvider(props: PropsWithChildren<unknown>) {
  const [messages, setMessages] = useState<Message[]>([]);

  const [streaming, setStreaming] = useState(false);

  const [ws, setWS] = useState<WebSocket>(null);

  useEffect(() => {
    setWS(
      new WebSocket(
        'wss://' +
          getHostName() +
          `/v1/webapi/assistant?auth_token=${getAccessToken()}`
      )
    );
  }, []);

  useEffect(() => {
    if (ws) {
      ws.onmessage = data => {
        console.log('data', data);
      };
    }
  }, [ws]);

  const send = useCallback(
    async (message: Message) => {
      const newMessages = [...messages, message];

      setMessages(newMessages);

      ws.send(JSON.stringify(newMessages));
    },
    [messages]
  );

  return (
    <MessagesContext.Provider value={{ messages, send, streaming }}>
      {props.children}
    </MessagesContext.Provider>
  );
}

export function useMessages() {
  return useContext(MessagesContext);
}
