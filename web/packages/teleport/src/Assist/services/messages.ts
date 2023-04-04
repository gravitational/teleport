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
  Exec = 'exec',
  Message = 'message',
  Connect = 'connect',
}

export interface MessageContent {
  type: Type;
  value: string | string[];
}

export interface Message {
  hidden?: boolean;
  content: MessageContent[];
  author: Author;
}

interface CommandOutput {
  serverName: string;
  commandOutput: string;
}

export interface ExecOutput {
  commandOutputs: CommandOutput[];
  humanInterpretation: string;
}

export async function sendMessage(
  messages: Message[]
): Promise<MessageContent[]> {
  const res = await fetch('/api/request', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(messages),
  });

  return res.json();
}

export async function exec(contents: MessageContent[]): Promise<ExecOutput[]> {
  const res = await fetch('/api/exec', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(contents),
  });

  return res.json();
}
