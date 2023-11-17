import React, {
  createContext,
  PropsWithChildren,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';

import { ServerMessage, ServerMessageType } from 'teleport/Assist/types';
import { getAccessToken, getHostName } from 'teleport/services/api';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';

interface QueryAssistContextValue {
  close: () => void;
  loading: boolean;
  latestMessage: string;
  send: (message: string) => void;
  open: () => void;
  visible: boolean;
  errorMessage: string | null;
}

const QueryAssistContext = createContext<QueryAssistContextValue>(null);

export function QueryAssistContextProvider(props: PropsWithChildren<unknown>) {
  const { clusterId } = useStickyClusterId();

  const [visible, setVisible] = useState(false);

  const socketRef = useRef<WebSocket | null>(null);
  const socketUrl = cfg.getAssistActionWebSocketUrl(
    getHostName(),
    clusterId,
    getAccessToken(),
    'audit-query'
  );

  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [latestMessage, setLatestMessage] = useState('');

  useEffect(() => {
    socketRef.current = new WebSocket(socketUrl);

    socketRef.current.onerror = () => {
      setErrorMessage('Could not connect to the Assist backend');
    };

    socketRef.current.onmessage = e => {
      const data = JSON.parse(e.data) as ServerMessage;

      if (data.type === ServerMessageType.AssistPartialMessage) {
        setLatestMessage(message => message + data.payload);
      }

      if (data.type === ServerMessageType.AssistPartialMessageEnd) {
        setLoading(false);
      }

      if (data.type === ServerMessageType.Assist) {
        // if we have a message from Assist then it was unable to generate the query, so
        // we will display that as an error

        setErrorMessage(data.payload);
        setLoading(false);
      }
    };
  }, []);

  function close() {
    setVisible(false);
  }

  function open() {
    setVisible(true);
    setErrorMessage(null);
  }

  function send(message: string) {
    setErrorMessage(null);
    setLoading(true);
    setVisible(false);
    setLatestMessage('');

    socketRef.current.send(message);
  }

  return (
    <QueryAssistContext.Provider
      value={{
        close,
        loading,
        latestMessage,
        send,
        open,
        visible,
        errorMessage,
      }}
    >
      {props.children}
    </QueryAssistContext.Provider>
  );
}

export function useQueryAssist() {
  return useContext(QueryAssistContext);
}
