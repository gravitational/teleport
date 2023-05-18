/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export type PtyProcessOptions = {
  env: Record<string, string>;
  path: string;
  args: string[];
  cwd?: string;
  initCommand?: string;
};

export type IPtyProcess = {
  start(cols: number, rows: number): void;
  write(data: string): void;
  resize(cols: number, rows: number): void;
  dispose(): void;
  getCwd(): Promise<string>;
  getPtyId(): string;
  // The listener removal functions are used only on the frontend app side from the renderer process.
  // They're not used in the shared process. However, IPtyProcess is a type shared between both, so
  // both sides need to return them. In the future we should consider defining two separate types
  // for both cases.
  onData(cb: (data: string) => void): RemoveListenerFunction;
  onOpen(cb: () => void): RemoveListenerFunction;
  onStartError(cb: (message: string) => void): RemoveListenerFunction;
  onExit(
    cb: (ev: { exitCode: number; signal?: number }) => void
  ): RemoveListenerFunction;
};

type RemoveListenerFunction = () => void;
