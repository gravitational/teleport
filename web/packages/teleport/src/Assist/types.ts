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

import { EventType } from 'teleport/lib/term/enums';

export enum ServerMessageType {
  Assist = 'CHAT_MESSAGE_ASSISTANT',
  User = 'CHAT_MESSAGE_USER',
  Error = 'CHAT_MESSAGE_ERROR',
  Command = 'COMMAND',
  CommandResult = 'COMMAND_RESULT',
  CommandResultSummary = 'COMMAND_RESULT_SUMMARY',
  CommandResultStream = 'COMMAND_RESULT_STREAM',
  AssistPartialMessage = 'CHAT_PARTIAL_MESSAGE_ASSISTANT',
  AssistPartialMessageEnd = 'CHAT_PARTIAL_MESSAGE_ASSISTANT_FINALIZE',
  AssistThought = 'CHAT_MESSAGE_PROGRESS_UPDATE',
  AccessRequests = 'ACCESS_REQUESTS',
  AccessRequest = 'ACCESS_REQUEST',
  AccessRequestCreated = 'ACCESS_REQUEST_CREATED',
}

// ExecutionEnvelopeType is the type of message that is returned when
// the command summary is returned.
export const ExecutionEnvelopeType = 'summary';

// ExecutionTeleportErrorType is the type of error that is returned when
// Teleport returns an error (failed to execute command, failed to connect, etc.)
export const ExecutionTeleportErrorType = 'teleport-error';

export interface Conversation {
  id: string;
  title?: string;
  created: Date;
}

export interface GetConversationsResponse {
  conversations: [
    {
      id: string;
      title?: string;
      created_time: string;
    },
  ];
}

export interface ServerMessage {
  type: ServerMessageType;
  conversation_id: string;
  payload: string;
  created_time: string;
}

export interface ResolvedCommandServerMessage {
  type: ServerMessageType.Command;
  created: Date;
  query: string;
  command: string;
}

export interface ResolvedCommandResultServerMessage {
  type: ServerMessageType.CommandResult;
  nodeId: string;
  nodeName: string;
  executionId: string;
  sessionId: string;
  output?: string;
  errorMessage?: string;
  created: Date;
}

export interface ResolvedCommandResultSummaryServerMessage {
  type: ServerMessageType.CommandResultSummary;
  executionId: string;
  summary: string;
  command: string;
  created: Date;
}

export interface ResolvedAssistThoughtServerMessage {
  type: ServerMessageType.AssistThought;
  message: string;
  created: Date;
}

export interface ResolvedAssistServerMessage {
  type: ServerMessageType.Assist;
  message: string;
  created: Date;
}

export interface ResolvedUserServerMessage {
  type: ServerMessageType.User;
  message: string;
  created: Date;
}

export interface ResolvedErrorServerMessage {
  type: ServerMessageType.Error;
  message: string;
  created: Date;
}

export interface ResolvedCommandResultStreamServerMessage {
  type: ServerMessageType.CommandResultStream;
  id: number;
  nodeId: string;
  nodeName: string;
  output: string;
  finished: boolean;
  created: Date;
}

export interface ResolvedAccessRequestCreatedMessage {
  type: ServerMessageType.AccessRequestCreated;
  accessRequestId: string;
  created: Date;
}

export interface AccessRequestClientMessage {
  type: ServerMessageType.AccessRequestCreated;
  payload: string;
}

interface AccessRequestResourceBase {
  id: string;
  type: 'app' | 'node' | 'kubernetes' | 'desktop' | 'database';
}

interface AccessRequestGenericResource extends AccessRequestResourceBase {
  type: 'app' | 'kubernetes' | 'desktop' | 'database';
}

interface AccessRequestNodeResource extends AccessRequestResourceBase {
  type: 'node';
  friendlyName: string;
}

export type AccessRequestResource =
  | AccessRequestGenericResource
  | AccessRequestNodeResource;

export interface ResolvedAccessRequestServerMessage {
  type: ServerMessageType.AccessRequest;
  reason: string;
  suggestedReviewers: string[];
  resources: AccessRequestResource[];
  roles: string[];
  created: Date;
}

export interface Resource {
  type: string;
  id: string;
  name: string;
  cluster: string;
}

export enum AccessRequestStatus {
  Pending,
  Approved,
  Declined,
}

export type ResolvedServerMessage =
  | ResolvedCommandServerMessage
  | ResolvedAssistServerMessage
  | ResolvedUserServerMessage
  | ResolvedErrorServerMessage
  | ResolvedCommandResultServerMessage
  | ResolvedCommandResultSummaryServerMessage
  | ResolvedAssistThoughtServerMessage
  | ResolvedCommandResultStreamServerMessage
  | ResolvedAccessRequestServerMessage
  | ResolvedAccessRequestCreatedMessage;

export interface GetConversationMessagesResponse {
  messages: ServerMessage[];
}

export enum Author {
  Teleport,
  User,
}

export interface CreateConversationResponse {
  id: string;
}

export interface GenerateTitleResponse {
  title: string;
}

export interface ConversationMessage {
  streaming: boolean;
  entries: ResolvedServerMessage[];
  author: Author;
  timestamp: Date;
}

export interface CommandResultPayload {
  node_id: string;
  node_name: string;
  session_id: string;
  execution_id: string;
}

export interface CommandResultSummaryPayload {
  execution_id: string;
  command: string;
  summary: string;
}

export interface ThoughtMessagePayload {
  action: string;
}

export interface ExecEvent {
  event: EventType.EXEC;
  exitError?: string;
}

export type SessionEvent = ExecEvent | { event: string };

export enum RunActionStatus {
  Connecting,
  Finished,
}

export interface NodeState {
  nodeId: string;
  status: RunActionStatus;
  stdout?: string;
}

export interface RawPayload {
  node_id: string;
  type: string;
  payload: string;
}

export interface SessionData {
  session: { server_id: string };
}

export interface SessionEndData {
  node_id: string;
}

export interface AccessRequestPayload {
  roles: string[];
  resources: AccessRequestResource[];
  reason: string;
  suggested_reviewers: string[];
  created_time: Date;
}

export interface ExecuteRemoteCommandPayload {
  command: string;
  login?: string;
  labels?: { key: string; value: string }[];
  nodes?: string[];
}
