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

import { Author } from 'teleport/Assist/types';

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
