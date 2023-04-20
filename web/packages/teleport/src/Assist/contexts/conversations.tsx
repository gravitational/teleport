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

import api from 'teleport/services/api';

interface Conversation {
  id: string;
  title: string;
}

interface MessageContextValue {
  create: () => Promise<string>;
  conversations: Conversation[];
}

interface CreateConversationResponse {
  id: string;
}

interface ListConversationsResponse {
  conversation_id: string[]; // TODO: this should be an array of objects with `title` and `conversation_id` properties
}

const ConversationsContext = createContext<MessageContextValue>({
  conversations: [],
  create: () => Promise.resolve(void 0),
});

export function ConversationsContextProvider(
  props: PropsWithChildren<unknown>
) {
  const [conversations, setConversations] = useState<Conversation[]>([]);

  const load = useCallback(async () => {
    setConversations([]);

    const res: ListConversationsResponse = await api.get(
      '/v1/webapi/assistant/conversations'
    );

    setConversations(
      res.conversation_id.reverse().map(conversationId => ({
        id: conversationId,
        title: 'New Chat',
      }))
    );
  }, []);

  const create = useCallback(async () => {
    const res: CreateConversationResponse = await api.post(
      '/v1/webapi/assistant/conversations'
    );

    setConversations(conversations => [
      {
        id: res.id,
        title: 'New Chat',
      },
      ...conversations,
    ]);

    return res.id;
  }, []);

  useEffect(() => {
    load();
  }, []);

  return (
    <ConversationsContext.Provider value={{ conversations, create }}>
      {props.children}
    </ConversationsContext.Provider>
  );
}

export function useConversations() {
  return useContext(ConversationsContext);
}
