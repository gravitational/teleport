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
    }
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

export interface ResolvedAccessRequestServerMessage {
  type: ServerMessageType.AccessRequest;
  resources: Resource[];
  reason: string;
  suggested_reviewers: string[];
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

export interface AccessRequestEvent {
  created: Date;
  resources: Resource[];
  message: string;
  username: string;
  roles: string[];
}

export interface ResolvedAccessRequestsServerMessage {
  type: ServerMessageType.AccessRequests;
  status: AccessRequestStatus;
  summary: string;
  username: string;
  events: AccessRequestEvent[];
  created: Date;
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
  | ResolvedAccessRequestsServerMessage
  | ResolvedAccessRequestServerMessage;

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

export interface ExecuteRemoteCommandPayload {
  command: string;
  login?: string;
  labels?: { key: string; value: string }[];
  nodes?: string[];
}

export enum ViewMode {
  Docked = 1,
  Popup = 2,
  PopupExpanded = 3,
  PopupExpandedSidebarVisible = 4,
}

export interface AssistUserPreferences {
  preferredLogins: string[];
  viewMode: ViewMode;
}
