/**
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

import {
  ResolvedUserServerMessage,
  ServerMessageType,
  ViewMode,
} from 'teleport/Assist/types';

import {
  AssistStateActionType,
  replaceConversations,
  setConversationMessages,
  setConversationMessagesLoading,
  setConversationsLoading,
  setSelectedConversationId,
} from './state';

import type {
  AssistState,
  ReplaceConversationsAction,
  SetConversationMessagesAction,
  SetConversationMessagesLoadingAction,
  SetConversationsLoadingAction,
  SetSelectedConversationIdAction,
} from './state';

const defaultState: AssistState = {
  settings: {
    loading: false,
    viewMode: ViewMode.Docked,
    preferredLogins: [],
    sidebarVisible: false,
  },
  conversations: {
    loading: false,
    selectedId: null,
    error: null,
    data: [],
  },
  messages: {
    loading: false,
    streaming: false,
    data: new Map(),
  },
  mfa: {
    prompt: false,
    publicKey: null,
  },
};

describe('assist state', () => {
  it('should set conversations loading', () => {
    const action: SetConversationsLoadingAction = {
      type: AssistStateActionType.SetConversationsLoading,
      loading: true,
    };

    expect(defaultState.conversations.loading).toBe(false);

    const nextState = setConversationsLoading(defaultState, action);

    expect(nextState.conversations.loading).toBe(true);
  });

  it('should replace conversations', () => {
    const state = {
      ...defaultState,
      conversations: {
        ...defaultState.conversations,
        data: [
          {
            id: '3',
            title: 'Conversation 3',
            created: new Date('2020-01-03'),
          },
        ],
      },
    };

    const action: ReplaceConversationsAction = {
      type: AssistStateActionType.ReplaceConversations,
      conversations: [
        {
          id: '1',
          title: 'Conversation 1',
          created: new Date('2020-01-01'),
        },
        {
          id: '2',
          title: 'Conversation 2',
          created: new Date('2020-01-02'),
        },
      ],
    };

    const nextState = replaceConversations(state, action);

    expect(nextState.conversations.data).toHaveLength(2);

    expect(nextState.conversations.data[0].id).toBe('1');
    expect(nextState.conversations.data[0].title).toBe('Conversation 1');

    expect(nextState.conversations.data[1].id).toBe('2');
    expect(nextState.conversations.data[1].title).toBe('Conversation 2');
  });

  it('should set selected conversation id', () => {
    const action: SetSelectedConversationIdAction = {
      type: AssistStateActionType.SetSelectedConversationId,
      conversationId: '1',
    };

    expect(defaultState.conversations.selectedId).toBeNull();

    const nextState = setSelectedConversationId(defaultState, action);

    expect(nextState.conversations.selectedId).toBe('1');
  });

  it('should set conversation messages loading', () => {
    const action: SetConversationMessagesLoadingAction = {
      type: AssistStateActionType.SetConversationMessagesLoading,
      loading: true,
    };

    expect(defaultState.messages.loading).toBe(false);

    const nextState = setConversationMessagesLoading(defaultState, action);

    expect(nextState.messages.loading).toBe(true);
  });

  it('should set conversation messages', () => {
    const state: AssistState = {
      ...defaultState,
      messages: {
        ...defaultState.messages,
        data: new Map([
          [
            '1',
            [
              {
                type: ServerMessageType.User,
                message: 'Message 1',
                created: new Date('2020-01-01'),
              },
            ],
          ],
        ]),
      },
    };

    const action: SetConversationMessagesAction = {
      type: AssistStateActionType.SetConversationMessages,
      conversationId: '2',
      messages: [
        {
          type: ServerMessageType.User,
          message: 'Message 2',
          created: new Date('2020-01-02'),
        },
      ],
    };

    expect(state.messages.data.has('2')).toBe(false);

    const nextState = setConversationMessages(state, action);

    const newMessage = nextState.messages.data.get(
      '2'
    ) as ResolvedUserServerMessage[];

    expect(newMessage).toHaveLength(1);
    expect(newMessage[0].message).toBe('Message 2');
  });
});
