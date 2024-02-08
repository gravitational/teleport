/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, {
  createContext,
  PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
} from 'react';

import { AssistStateActionType, reducer } from 'teleport/Assist/context/state';

import { convertServerMessages } from 'teleport/Assist/context/utils';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';
import { getHostName } from 'teleport/services/api';

import {
  AccessRequestClientMessage,
  ExecutionEnvelopeType,
  ExecutionTeleportErrorType,
  RawPayload,
  ServerMessageType,
  SessionEndData,
} from 'teleport/Assist/types';

import { MessageTypeEnum, Protobuf } from 'teleport/lib/term/protobuf';

import {
  makeMfaAuthenticateChallenge,
  WebauthnAssertionResponse,
} from 'teleport/services/auth';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';

import * as service from '../service';
import {
  resolveAccessRequestMessage,
  resolveServerAssistThoughtMessage,
  resolveServerCommandMessage,
  resolveServerMessage,
} from '../service';

import type {
  ConversationMessage,
  ResolvedServerMessage,
  ServerMessage,
} from 'teleport/Assist/types';
import type { AssistState } from 'teleport/Assist/context/state';

interface AssistContextValue {
  cancelMfaChallenge: () => void;
  createConversation: () => Promise<string>;
  deleteConversation: (conversationId: string) => void;
  executeCommand: (login: string, command: string, query: string) => void;
  sendAccessRequestCreatedMessage: (accessRequestId: string) => void;
  sendMessage: (message: string) => void;
  sendMfaChallenge: (data: WebauthnAssertionResponse) => void;
  selectedConversationMessages: ConversationMessage[];
  setSelectedConversationId: (conversationId: string) => Promise<void>;
  toggleSidebar: (visible: boolean) => void;
}

const AssistContext = createContext<AssistState & AssistContextValue>(null);

let lastCommandExecutionResultId = 0;

const TEN_MINUTES = 10 * 60 * 1000;

export function AssistContextProvider(props: PropsWithChildren<unknown>) {
  const activeWebSocket = useRef<AuthenticatedWebSocket>(null);
  // TODO(ryan): this should be removed once https://github.com/gravitational/teleport.e/pull/1609 is implemented
  const executeCommandWebSocket = useRef<AuthenticatedWebSocket>(null);
  const refreshWebSocketTimeout = useRef<number | null>(null);

  const { clusterId } = useStickyClusterId();

  const [state, dispatch] = useReducer(reducer, {
    sidebarVisible: false,
    conversations: {
      loading: false,
      data: [],
      selectedId: null,
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
  });

  async function loadConversations() {
    dispatch({
      type: AssistStateActionType.SetConversationsLoading,
      loading: true,
    });

    const conversations = await service.loadConversations();

    dispatch({
      type: AssistStateActionType.ReplaceConversations,
      conversations,
    });
  }

  function setupWebSocket(conversationId: string, initialMessage?: string) {
    activeWebSocket.current = new AuthenticatedWebSocket(
      cfg.getAssistConversationWebSocketUrl(
        getHostName(),
        clusterId,
        conversationId
      )
    );

    window.clearTimeout(refreshWebSocketTimeout.current);

    // refresh the websocket connection just before the ten-minute timeout of the session
    refreshWebSocketTimeout.current = window.setTimeout(
      () => setupWebSocket(conversationId),
      TEN_MINUTES * 0.8
    );

    activeWebSocket.current.onopen = () => {
      if (initialMessage) {
        activeWebSocket.current.send(initialMessage);
      }
    };

    activeWebSocket.current.onclose = () => {
      dispatch({
        type: AssistStateActionType.SetStreaming,
        streaming: false,
      });
    };

    activeWebSocket.current.onmessage = async event => {
      const data = JSON.parse(event.data) as ServerMessage;

      switch (data.type) {
        case ServerMessageType.Assist:
          dispatch({
            type: AssistStateActionType.AddMessage,
            messageType: ServerMessageType.Assist,
            message: data.payload,
            conversationId,
          });

          dispatch({
            type: AssistStateActionType.SetStreaming,
            streaming: false,
          });

          break;

        case ServerMessageType.AssistPartialMessageEnd:
          dispatch({
            type: AssistStateActionType.SetStreaming,
            streaming: false,
          });

          break;

        case ServerMessageType.AssistPartialMessage: {
          dispatch({
            type: AssistStateActionType.AddPartialMessage,
            message: data.payload,
            conversationId,
          });

          break;
        }

        case ServerMessageType.AssistThought:
          const message = resolveServerAssistThoughtMessage(data);
          dispatch({
            type: AssistStateActionType.AddThought,
            message: message.message,
            conversationId,
          });

          break;
        case ServerMessageType.Command: {
          const message = resolveServerCommandMessage(data);

          dispatch({
            type: AssistStateActionType.AddExecuteRemoteCommand,
            message,
            conversationId,
          });

          dispatch({
            type: AssistStateActionType.SetStreaming,
            streaming: false,
          });

          break;
        }

        case ServerMessageType.AccessRequest: {
          const message = resolveAccessRequestMessage(data);

          dispatch({
            type: AssistStateActionType.AddAccessRequest,
            message,
            conversationId,
          });

          dispatch({
            type: AssistStateActionType.SetStreaming,
            streaming: false,
          });

          break;
        }

        case ServerMessageType.Error:
          dispatch({
            type: AssistStateActionType.AddMessage,
            messageType: ServerMessageType.Error,
            message: data.payload,
            conversationId,
          });

          dispatch({
            type: AssistStateActionType.SetStreaming,
            streaming: false,
          });

          break;
      }
    };
  }

  async function createConversation() {
    if (state.messages.streaming) {
      dispatch({
        type: AssistStateActionType.SetStreaming,
        streaming: false,
      });
    }

    dispatch({
      type: AssistStateActionType.SetConversationsLoading,
      loading: true,
    });

    const conversationId = await service.createConversation();

    dispatch({
      type: AssistStateActionType.AddConversation,
      conversationId,
    });

    setupWebSocket(conversationId);

    const serverMessages =
      await service.loadConversationMessages(conversationId);
    const messages: ResolvedServerMessage[] = [];

    for (const message of serverMessages) {
      messages.push(await resolveServerMessage(message, clusterId));
    }

    dispatch({
      type: AssistStateActionType.SetConversationMessages,
      conversationId,
      messages,
    });

    return conversationId;
  }

  async function setSelectedConversationId(conversationId: string) {
    if (activeWebSocket.current) {
      activeWebSocket.current.close();
    }

    if (state.messages.streaming) {
      dispatch({
        type: AssistStateActionType.SetStreaming,
        streaming: false,
      });
    }

    dispatch({
      type: AssistStateActionType.SetSelectedConversationId,
      conversationId,
    });

    if (!state.messages.data.has(conversationId)) {
      dispatch({
        type: AssistStateActionType.SetConversationMessagesLoading,
        loading: true,
      });

      const serverMessages =
        await service.loadConversationMessages(conversationId);
      const messages: ResolvedServerMessage[] = [];

      for (const message of serverMessages) {
        messages.push(await resolveServerMessage(message, clusterId));
      }

      dispatch({
        type: AssistStateActionType.SetConversationMessages,
        conversationId,
        messages,
      });
    }

    setupWebSocket(conversationId);
  }

  async function sendMessage(message: string) {
    if (!activeWebSocket.current) {
      return;
    }

    const messages = state.messages.data.get(state.conversations.selectedId);

    dispatch({
      type: AssistStateActionType.SetStreaming,
      streaming: true,
    });

    const data = JSON.stringify({ payload: message });

    if (
      !activeWebSocket.current ||
      activeWebSocket.current.readyState === AuthenticatedWebSocket.CLOSED
    ) {
      setupWebSocket(state.conversations.selectedId, data);
    } else {
      activeWebSocket.current.send(data);
    }

    dispatch({
      type: AssistStateActionType.AddMessage,
      messageType: ServerMessageType.User,
      conversationId: state.conversations.selectedId,
      message,
    });

    if (messages.length === 1) {
      const title = await service.generateTitle(message);

      await service.setConversationTitle(state.conversations.selectedId, title);

      dispatch({
        type: AssistStateActionType.UpdateConversationTitle,
        conversationId: state.conversations.selectedId,
        title,
      });
    }
  }

  function sendMfaChallenge(data: WebauthnAssertionResponse) {
    if (
      !executeCommandWebSocket.current ||
      executeCommandWebSocket.current.readyState !==
        AuthenticatedWebSocket.OPEN ||
      !data
    ) {
      console.warn(
        'websocket unavailable',
        executeCommandWebSocket.current,
        data
      );

      return;
    }

    dispatch({
      type: AssistStateActionType.PromptMfa,
      promptMfa: false,
      publicKey: null,
    });

    const encoder = new window.TextEncoder();
    const proto = new Protobuf();

    const encodedMessage = encoder.encode(JSON.stringify(data));
    const message = proto.encodeChallengeResponse(encodedMessage);
    const bytearray = new Uint8Array(message);

    executeCommandWebSocket.current.send(bytearray.buffer);
  }

  async function executeCommand(login: string, command: string, query: string) {
    if (executeCommandWebSocket.current) {
      executeCommandWebSocket.current.close();
    }

    dispatch({
      type: AssistStateActionType.AddThought,
      conversationId: state.conversations.selectedId,
      message: 'Connecting to nodes',
    });

    const nodes = await service.getNodesFromQuery(query, clusterId);

    const nodeIdToResultId = new Map<string, number>();

    for (const node of nodes.agents) {
      const id = lastCommandExecutionResultId++;

      nodeIdToResultId.set(node.id, id);

      dispatch({
        type: AssistStateActionType.AddCommandResult,
        conversationId: state.conversations.selectedId,
        id,
        nodeName: node.hostname,
        nodeId: node.id,
      });
    }

    const execParams = {
      login,
      command,
      query,
      conversation_id: state.conversations.selectedId,
      execution_id: crypto.randomUUID(),
    };

    const url = cfg.getAssistExecuteCommandUrl(
      getHostName(),
      clusterId,
      execParams
    );

    const proto = new Protobuf();
    executeCommandWebSocket.current = new AuthenticatedWebSocket(url);
    executeCommandWebSocket.current.binaryType = 'arraybuffer';

    executeCommandWebSocket.current.onmessage = event => {
      const uintArray = new Uint8Array(event.data);

      const msg = proto.decode(uintArray);

      switch (msg.type) {
        case MessageTypeEnum.RAW: {
          const data = JSON.parse(msg.payload) as RawPayload;
          const payload = atob(data.payload);

          switch (data.type) {
            case ExecutionTeleportErrorType:
              dispatch({
                type: AssistStateActionType.FinishCommandResult,
                conversationId: state.conversations.selectedId,
                commandResultId: nodeIdToResultId.get(data.node_id),
              });

              nodeIdToResultId.delete(data.node_id);
              break;

            case ExecutionEnvelopeType:
              dispatch({
                type: AssistStateActionType.AddCommandResultSummary,
                conversationId: state.conversations.selectedId,
                summary: payload,
                executionId: execParams.execution_id,
                command: execParams.command,
              });
              break;

            default:
              dispatch({
                type: AssistStateActionType.UpdateCommandResult,
                conversationId: state.conversations.selectedId,
                commandResultId: nodeIdToResultId.get(data.node_id),
                output: payload,
              });
          }

          break;
        }

        case MessageTypeEnum.WEBAUTHN_CHALLENGE:
          const challenge = JSON.parse(msg.payload);
          const publicKey =
            makeMfaAuthenticateChallenge(challenge).webauthnPublicKey;

          dispatch({
            type: AssistStateActionType.PromptMfa,
            promptMfa: true,
            publicKey,
          });

          break;

        case MessageTypeEnum.ERROR:
          console.error(msg.payload);

          break;

        case MessageTypeEnum.SESSION_END: {
          const data = JSON.parse(msg.payload) as SessionEndData;

          dispatch({
            type: AssistStateActionType.FinishCommandResult,
            conversationId: state.conversations.selectedId,
            commandResultId: nodeIdToResultId.get(data.node_id),
          });

          nodeIdToResultId.delete(data.node_id);

          break;
        }
      }
    };

    executeCommandWebSocket.current.onclose = () => {
      executeCommandWebSocket.current = null;

      // If the execution failed, we won't get a SESSION_END message, so we
      // need to mark all the results as finished here.
      for (const nodeId of nodeIdToResultId.keys()) {
        dispatch({
          type: AssistStateActionType.FinishCommandResult,
          conversationId: state.conversations.selectedId,
          commandResultId: nodeIdToResultId.get(nodeId),
        });
      }
      nodeIdToResultId.clear();
    };
  }

  async function deleteConversation(conversationId: string) {
    await service.deleteConversation(conversationId);

    dispatch({
      type: AssistStateActionType.DeleteConversation,
      conversationId,
    });
  }

  function cancelMfaChallenge() {
    dispatch({
      type: AssistStateActionType.PromptMfa,
      promptMfa: false,
      publicKey: null,
    });
  }

  function toggleSidebar(visible: boolean) {
    dispatch({
      type: AssistStateActionType.ToggleSidebar,
      visible,
    });
  }

  function sendAccessRequestCreatedMessage(accessRequestId: string) {
    if (!activeWebSocket.current) {
      return;
    }

    const message: AccessRequestClientMessage = {
      type: ServerMessageType.AccessRequestCreated,
      payload: accessRequestId,
    };

    const data = JSON.stringify(message);
    activeWebSocket.current.send(data);

    dispatch({
      type: AssistStateActionType.AddAccessRequestCreated,
      conversationId: state.conversations.selectedId,
      accessRequestId,
    });
  }

  useEffect(() => {
    loadConversations();

    return () => {
      window.clearTimeout(refreshWebSocketTimeout.current);
    };
  }, []);

  const selectedConversationMessages = useMemo(
    () =>
      state.messages.data.has(state.conversations.selectedId)
        ? convertServerMessages(
            state.messages.data.get(state.conversations.selectedId)
          )
        : null,
    [state.conversations.selectedId, state.messages.data]
  );

  return (
    <AssistContext.Provider
      value={{
        ...state,
        cancelMfaChallenge,
        createConversation,
        deleteConversation,
        executeCommand,
        selectedConversationMessages,
        sendAccessRequestCreatedMessage,
        sendMessage,
        sendMfaChallenge,
        setSelectedConversationId,
        toggleSidebar,
      }}
    >
      {props.children}
    </AssistContext.Provider>
  );
}

export function useAssist() {
  return useContext(AssistContext);
}
