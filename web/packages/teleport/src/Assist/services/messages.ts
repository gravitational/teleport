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
  ExecuteCommandOutput = 'execution_output',
}

export interface Label {
  key: string;
  value: string;
}

export interface ExecuteRemoteCommandContent {
  type: Type.ExecuteRemoteCommand;
  command: string;
  selectedLogin: string;
  availableLogins: string[];
  query: string;
  errorMsg: string;
}

export interface ExecuteRemoteCommandPayload {
  type: Type.ExecuteRemoteCommand;
  command: string;
  login?: string;
  labels?: { key: string; value: string }[];
  nodes?: string[];
}

export interface TextMessageContent {
  type: Type.Message;
  value: string;
}

export interface CommandExecutionOutput {
  type: Type.ExecuteCommandOutput;
  nodeId: string;
  executionId: string;
  payload: string;
  errorMsg?: string;
}

export type MessageContent =
  | TextMessageContent
  | ExecuteRemoteCommandContent
  | CommandExecutionOutput;

export interface Message {
  isNew?: boolean;
  content: MessageContent;
  author: Author;
  idx?: number;
  timestamp: string;
}

export interface ExecOutput {
  humanInterpretation: string;
  commandOutputs: { serverName: string; commandOutput: string }[];
}
