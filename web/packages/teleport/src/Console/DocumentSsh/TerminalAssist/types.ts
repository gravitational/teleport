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

export enum Author {
  Teleport,
  User,
}

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

export interface ServerMessage {
  type: ServerMessageType;
  conversation_id: string;
  payload: string;
  created_time: string;
}

export enum MessageType {
  User,
  SuggestedCommand,
  Explanation,
}

export interface UserMessage {
  author: Author.User;
  type: MessageType.User;
  value: string;
}

export interface SuggestedCommandMessage {
  author: Author.Teleport;
  type: MessageType.SuggestedCommand;
  command: string;
  reasoning: string;
}

export interface ExplanationMessage {
  author: Author.Teleport;
  type: MessageType.Explanation;
  value: string;
}

export type Message =
  | UserMessage
  | SuggestedCommandMessage
  | ExplanationMessage;
