/*
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
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react';
import useWebSocket from 'react-use-websocket';

import { useParams } from 'react-router';

import Logger from 'shared/libs/logger';

import api, { getAccessToken, getHostName } from 'teleport/services/api';

import NodeService from 'teleport/services/nodes';
import useStickyClusterId from 'teleport/useStickyClusterId';

import cfg from 'teleport/config';

import { ApiError } from 'teleport/services/api/parseError';

import {
  Author,
  ExecuteRemoteCommandContent,
  ExecuteRemoteCommandPayload,
  Message,
  TextMessageContent,
  Type,
} from '../services/messages';

interface MessageContextValue {
  send: (message: string) => Promise<void>;
  messages: Message[];
  loading: boolean;
  responding: boolean;
  error: string | null;
}

interface ServerMessage {
  conversation_id: string;
  type: string;
  payload: string;
  created_time: string;
}

interface ConversationHistoryResponse {
  messages: ServerMessage[];
}

const MessagesContext = createContext<MessageContextValue>({
  messages: [],
  send: () => Promise.resolve(void 0),
  loading: true,
  responding: false,
  error: null,
});

interface MessagesContextProviderProps {
  conversationId: string;
}

type MessagesAction = (messages: Message[]) => void;

interface PartialMessagePayload {
  content: string;
  idx: number;
}

const convertToQuery = (cmd: ExecuteRemoteCommandPayload): string => {
  let query = '';

  if (cmd.nodes) {
    query += cmd.nodes.map(node => `name == "${node}"`).join(' || ');
  }

  if (cmd.labels) {
    if (cmd.nodes) {
      query += ' || ';
    }

    query += cmd.labels
      .map(label => `labels["${label.key}"] == "${label.value}"`)
      .join(' || ');
  }

  return query;
};

export const remoteCommandToMessage = async (
  execCmd: ExecuteRemoteCommandContent,
  clusterId: string
): Promise<ExecuteRemoteCommandContent> => {
  try {
    const ns = new NodeService();
    // fetch available users
    const nodes = await ns.fetchNodes(clusterId, {
      query: execCmd.query,
      limit: 100, // TODO: What if there is more nodes?
    });

    if (nodes.agents.length == 0) {
      return {
        ...execCmd,
        selectedLogin: '',
        availableLogins: [],
        errorMsg: 'no nodes found',
      };
    }

    const availableLogins = findIntersection(
      nodes.agents.map(e => e.sshLogins)
    );

    let errorMsg = '';
    if (availableLogins.length == 0) {
      errorMsg = 'no users found';
    }

    // If the login has been selected, use it.
    let avLogin = execCmd.selectedLogin;
    if (!avLogin) {
      // If the login has not been selected, use the first one.
      avLogin = availableLogins ? availableLogins[0] : '';
    } else {
      // If the login has been selected, check if it is available.
      // Updated query could have changed the available logins.
      if (!availableLogins.includes(avLogin)) {
        avLogin = availableLogins ? availableLogins[0] : '';
      }
    }

    return {
      ...execCmd,
      selectedLogin: avLogin,
      availableLogins: availableLogins,
      errorMsg: errorMsg,
    };
  } catch (e) {
    return {
      ...execCmd,
      errorMsg: (e as ApiError).message,
    };
  }
};

async function convertServerMessage(
  message: ServerMessage,
  clusterId: string
): Promise<MessagesAction> {
  if (
    message.type === 'CHAT_MESSAGE_ASSISTANT' ||
    message.type === 'CHAT_MESSAGE_ERROR'
  ) {
    const newMessage: Message = {
      author: Author.Teleport,
      timestamp: message.created_time,
      content: {
        type: Type.Message,
        value: message.payload,
      },
    };

    return (messages: Message[]) => messages.push(newMessage);
  }

  if (message.type === 'CHAT_PARTIAL_MESSAGE_ASSISTANT') {
    return (messages: Message[]) => {
      const partial: PartialMessagePayload = JSON.parse(message.payload);
      const existing = messages.findIndex(m => m.idx === partial.idx);
      if (existing !== -1) {
        const copy = JSON.parse(JSON.stringify(messages[existing]));
        (copy.content as TextMessageContent).value += partial.content;
        messages[existing] = copy;
      } else {
        const newMessage: Message = {
          author: Author.Teleport,
          timestamp: message.created_time,
          content: {
            type: Type.Message,
            value: partial.content,
          },
          idx: partial.idx,
        };

        messages.push(newMessage);
      }
    };
  }

  if (message.type === 'CHAT_PARTIAL_MESSAGE_ASSISTANT_FINALIZE') {
    return () => {};
  }

  if (message.type == 'COMMAND_RESULT') {
    const payload = JSON.parse(message.payload) as {
      session_id: string;
      execution_id: string;
      node_id: string;
    };

    const sessionUrl = cfg.getSshPlaybackPrefixUrl({
      clusterId: clusterId,
      sid: payload.session_id,
    });

    // The offset here is set base on A/B test that was run between me, myself and I.
    const resp = await api.fetch(sessionUrl + '/stream?offset=0&bytes=4096', {
      Accept: 'text/plain',
      'Content-Type': 'text/plain; charset=utf-8',
    });

    let msg;
    let errorMsg;
    if (resp.status === 200) {
      msg = await resp.text();
    } else {
      errorMsg = 'No session recording. The command execution failed.';
    }

    const newMessage: Message = {
      author: Author.Teleport,
      timestamp: message.created_time,
      content: {
        type: Type.ExecuteCommandOutput,
        nodeId: payload.node_id,
        executionId: payload.execution_id,
        payload: msg,
        errorMsg,
      },
    };

    return (messages: Message[]) => messages.push(newMessage);
  }

  if (message.type === 'CHAT_MESSAGE_USER') {
    const newMessage: Message = {
      author: Author.User,
      timestamp: message.created_time,
      content: {
        type: Type.Message,
        value: message.payload,
      },
    };

    return (messages: Message[]) => messages.push(newMessage);
  }

  if (message.type === 'COMMAND') {
    const execCmd: ExecuteRemoteCommandPayload = JSON.parse(message.payload);
    const searchQuery = convertToQuery(execCmd);
    const executionContent = await remoteCommandToMessage(
      {
        ...execCmd,
        type: Type.ExecuteRemoteCommand,
        query: searchQuery,
        selectedLogin: '',
        availableLogins: [],
        errorMsg: '',
      },
      clusterId
    );
    const newMessage = {
      author: Author.Teleport,
      isNew: true,
      timestamp: message.created_time,
      content: executionContent,
    };

    return (messages: Message[]) => messages.push(newMessage);
  }

  throw new Error('unrecognized message type');
}

function findIntersection<T>(elems: T[][]): T[] {
  if (elems.length == 0) {
    return [];
  }

  if (elems.length == 1) {
    return elems[0];
  }

  const intersectSets = (a: Set<T>, b: Set<T>) => {
    const c = new Set<T>();
    a.forEach(v => b.has(v) && c.add(v));
    return c;
  };

  return [...elems.map(e => new Set(e)).reduce(intersectSets)];
}

export async function generateTitle(messageContent: string): Promise<string> {
  const res: {
    title: string;
  } = await api.post(cfg.api.assistGenerateSummaryPath, {
    message: messageContent,
  });
  return res.title;
}

export async function setConversationTitle(
  conversationId: string,
  title: string
): Promise<void> {
  await api.post(cfg.getAssistSetConversationTitleUrl(conversationId), {
    title: title,
  });
}

const logger = Logger.create('assist');

export function MessagesContextProvider(
  props: PropsWithChildren<MessagesContextProviderProps>
) {
  const { conversationId } = useParams<{ conversationId: string }>();
  const { clusterId } = useStickyClusterId();

  const [error, setError] = useState<string>(null);
  const [loading, setLoading] = useState(true);
  const [responding, setResponding] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);

  const socketUrl = cfg.getAssistConversationWebSocketUrl(
    getHostName(),
    clusterId,
    getAccessToken(),
    props.conversationId
  );

  const { sendMessage, lastMessage } = useWebSocket(socketUrl, {
    share: true, // when share is false the websocket tends to disconnect
    retryOnError: true,
  });

  const load = useCallback(async () => {
    setMessages([]);

    const res: ConversationHistoryResponse = await api.get(
      cfg.getAssistConversationHistoryUrl(props.conversationId)
    );

    if (!res.messages) {
      return;
    }

    let messages: Message[] = [];
    for (const m of res.messages) {
      const action = await convertServerMessage(m, clusterId);
      action(messages);
    }

    setMessages(messages.map(message => ({ ...message, isNew: false })));
  }, [props.conversationId]);

  useEffect(() => {
    (async () => {
      setLoading(true);

      try {
        await load();

        setLoading(false);
      } catch (err) {
        setError('An error occurred whilst loading the conversation history');

        logger.error(err);
      }
    })();
  }, [props.conversationId]);

  useEffect(() => {
    if (lastMessage !== null) {
      const value = JSON.parse(lastMessage.data) as ServerMessage;

      // When a streaming message ends, or a non-streaming message arrives
      if (
        value.type === 'CHAT_PARTIAL_MESSAGE_ASSISTANT_FINALIZE' ||
        value.type === 'COMMAND' ||
        value.type === 'CHAT_MESSAGE_ASSISTANT' ||
        value.type === 'CHAT_MESSAGE_ERROR'
      ) {
        setResponding(false);
      }

      convertServerMessage(value, clusterId).then(res => {
        setMessages(prev => {
          const curr = [...prev];
          res(curr);
          return curr;
        });
      });
    }
  }, [lastMessage, setMessages, conversationId]);

  const send = useCallback(
    async (message: string) => {
      setResponding(true);

      const newMessages = [
        ...messages,
        {
          author: Author.User,
          timestamp: new Date().toISOString(),
          isNew: true,
          content: { type: Type.Message, value: message } as const,
        },
      ];

      setMessages(newMessages);

      const data = JSON.stringify({ payload: message });
      sendMessage(data);
    },
    [messages]
  );

  return (
    <MessagesContext.Provider
      value={{ messages, send, loading, responding, error }}
    >
      {props.children}
    </MessagesContext.Provider>
  );
}

export function useMessages() {
  return useContext(MessagesContext);
}
