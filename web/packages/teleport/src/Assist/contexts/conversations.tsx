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

import Logger from 'shared/libs/logger';

import api from 'teleport/services/api';
import cfg from 'teleport/config';

interface Conversation {
  id: string;
  title: string;
}

interface MessageContextValue {
  create: () => Promise<string>;
  conversations: Conversation[];
  setConversations: React.Dispatch<React.SetStateAction<Conversation[]>>;
  error: string | null;
}

interface CreateConversationResponse {
  id: string;
}

interface ListConversationsResponse {
  conversations: [
    {
      id: string;
      title?: string;
    }
  ];
}

const logger = Logger.create('assist');

const ConversationsContext = createContext<MessageContextValue>({
  conversations: [],
  create: () => Promise.resolve(void 0),
  setConversations: () => void 0,
  error: null,
});

export function ConversationsContextProvider(
  props: PropsWithChildren<unknown>
) {
  const [error, setError] = useState<string>(null);
  const [conversations, setConversations] = useState<Conversation[]>([]);

  const load = useCallback(async () => {
    setConversations([]);

    const res: ListConversationsResponse = await api.get(
      cfg.api.assistConversationsPath
    );

    setConversations(
      res.conversations?.reverse().map(conversation => ({
        id: conversation.id,
        title: conversation.title ?? 'New Chat',
      }))
    );
  }, []);

  const create = useCallback(async () => {
    try {
      const res: CreateConversationResponse = await api.post(
        cfg.api.assistConversationsPath
      );

      setConversations(conversations => [
        {
          id: res.id,
          title: 'New Chat',
        },
        ...conversations,
      ]);

      return res.id;
    } catch (err) {
      setError('An error occurred whilst creating a new conversation');

      logger.error(err);
    }
  }, []);

  useEffect(() => {
    (async () => {
      try {
        await load();
      } catch (err) {
        setError('An error occurred whilst loading the conversation history');

        logger.error(err);
      }
    })();
  }, []);

  return (
    <ConversationsContext.Provider
      value={{ conversations, create, setConversations, error }}
    >
      {props.children}
    </ConversationsContext.Provider>
  );
}

export function useConversations() {
  return useContext(ConversationsContext);
}
