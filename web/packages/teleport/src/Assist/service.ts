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

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import {
  convertPayloadToQuery,
  findIntersection,
  sortLoginsWithRootLoginsLast,
} from 'teleport/Assist/context/utils';

import { EventType } from 'teleport/lib/term/enums';

import NodeService from 'teleport/services/nodes';

import {
  ResolvedAssistThoughtServerMessage,
  ServerMessageType,
  ThoughtMessagePayload,
} from './types';

import type {
  CommandResultPayload,
  CommandResultSummaryPayload,
  Conversation,
  CreateConversationResponse,
  ExecEvent,
  ExecuteRemoteCommandPayload,
  GenerateTitleResponse,
  GetConversationMessagesResponse,
  GetConversationsResponse,
  ResolvedCommandResultServerMessage,
  ResolvedCommandResultSummaryServerMessage,
  ResolvedCommandServerMessage,
  ResolvedServerMessage,
  ServerMessage,
  SessionEvent,
} from './types';

export async function loadConversations(): Promise<Conversation[]> {
  const res: GetConversationsResponse = await api.get(
    cfg.api.assistConversationsPath
  );

  return res.conversations.reverse().map(({ id, title, created_time }) => ({
    id,
    title: title || 'New Conversation',
    created: new Date(created_time),
  }));
}

export async function resolveServerMessage(
  message: ServerMessage,
  clusterId: string
): Promise<ResolvedServerMessage> {
  switch (message.type) {
    case ServerMessageType.Command:
      return resolveServerCommandMessage(message);

    case ServerMessageType.CommandResult:
      return resolveServerCommandResultMessage(message, clusterId);

    case ServerMessageType.CommandResultSummary:
      return resolveServerCommandResultSummaryMessage(message);
    case ServerMessageType.AssistThought:
      return resolveServerAssistThoughtMessage(message);
    case ServerMessageType.Assist:
    case ServerMessageType.User:
      return {
        type: message.type,
        message: message.payload,
        created: new Date(message.created_time),
      };
  }
}

export async function getSessionEvents(sessionUrl: string): Promise<{
  events: SessionEvent[] | null;
}> {
  const response = await api.fetch(sessionUrl + '/events');

  if (response.status !== 200) {
    throw new Error('No session recording. The command execution failed.');
  }

  return response.json();
}

export async function getSessionStream(sessionUrl: string) {
  const stream = await api.fetch(sessionUrl + '/stream?offset=0&bytes=4096', {
    Accept: 'text/plain',
    'Content-Type': 'text/plain; charset=utf-8',
  });

  if (stream.status === 200) {
    return stream.text();
  }
}

function isExecEvent(e: SessionEvent): e is ExecEvent {
  return e.event == EventType.EXEC;
}

export function getNodesFromQuery(query: string, clusterId: string) {
  const ns = new NodeService();

  return ns.fetchNodes(clusterId, {
    query,
    limit: 100, // TODO: What if there is more nodes?
  });
}

export async function getLoginsForQuery(query: string, clusterId: string) {
  const nodes = await getNodesFromQuery(query, clusterId);

  if (!nodes.agents.length) {
    throw new Error('No nodes match the query');
  }

  const availableLogins = findIntersection(nodes.agents.map(e => e.sshLogins));

  if (!availableLogins.length) {
    throw new Error('No available logins found');
  }

  return sortLoginsWithRootLoginsLast(availableLogins);
}

export async function resolveServerCommandResultMessage(
  message: ServerMessage,
  clusterId: string
): Promise<ResolvedCommandResultServerMessage> {
  const payload = JSON.parse(message.payload) as CommandResultPayload;

  try {
    const sessionUrl = cfg.getSshPlaybackPrefixUrl({
      clusterId,
      sid: payload.session_id,
    });

    let output = await getSessionStream(sessionUrl);

    if (!output) {
      const events = await getSessionEvents(sessionUrl);
      const execEvent = events.events?.find(isExecEvent);

      output = execEvent?.exitError || 'Empty output';
    }

    return {
      type: ServerMessageType.CommandResult,
      nodeId: payload.node_id,
      nodeName: payload.node_name,
      executionId: payload.execution_id,
      sessionId: payload.session_id,
      created: new Date(message.created_time),
      output,
    };
  } catch (err) {
    return {
      type: ServerMessageType.CommandResult,
      nodeId: payload.node_id,
      nodeName: payload.node_name,
      executionId: payload.execution_id,
      sessionId: payload.session_id,
      created: new Date(message.created_time),
      errorMessage: err.message,
    };
  }
}

export function resolveServerCommandResultSummaryMessage(
  message: ServerMessage
): ResolvedCommandResultSummaryServerMessage {
  const payload = JSON.parse(message.payload) as CommandResultSummaryPayload;

  return {
    type: ServerMessageType.CommandResultSummary,
    executionId: payload.execution_id,
    command: payload.command,
    summary: payload.summary,
    created: new Date(message.created_time),
  };
}

export function resolveServerAssistThoughtMessage(
  message: ServerMessage
): ResolvedAssistThoughtServerMessage {
  const payload = JSON.parse(message.payload) as ThoughtMessagePayload;

  return {
    type: ServerMessageType.AssistThought,
    message: payload.reasoning || payload.action,
    created: new Date(message.created_time),
  };
}

export function resolveServerCommandMessage(
  message: ServerMessage
): ResolvedCommandServerMessage {
  const payload: ExecuteRemoteCommandPayload = JSON.parse(message.payload);
  const query = convertPayloadToQuery(payload);

  return {
    type: ServerMessageType.Command,
    created: new Date(message.created_time),
    query,
    command: payload.command,
  };
}

export async function loadConversationMessages(conversationId: string) {
  const res: GetConversationMessagesResponse = await api.get(
    cfg.getAssistConversationHistoryUrl(conversationId)
  );

  return res.messages;
}

export async function createConversation() {
  const res: CreateConversationResponse = await api.post(
    cfg.api.assistConversationsPath
  );

  return res.id;
}

export function deleteConversation(conversationId: string) {
  return api.delete(cfg.getAssistConversationHistoryUrl(conversationId));
}

export async function generateTitle(messageContent: string): Promise<string> {
  const res: GenerateTitleResponse = await api.post(
    cfg.api.assistGenerateSummaryPath,
    {
      message: messageContent,
    }
  );

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
