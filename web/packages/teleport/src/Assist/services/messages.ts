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

export enum Author {
  Teleport,
  User,
}

export enum Type {
  Message = 'message',
  ExecuteRemoteCommand = 'connect',
}

export interface Label {
  key: string;
  value: string;
}

export interface ExecuteRemoteCommandContent {
  type: Type.ExecuteRemoteCommand;
  command: string;
  labels?: string[];
  nodes?: string[];
  login?: string;
}

export interface TextMessageContent {
  type: Type.Message;
  value: string;
}

export type MessageContent = TextMessageContent | ExecuteRemoteCommandContent;

export interface Message {
  isNew?: boolean;
  content: MessageContent;
  author: Author;
}
