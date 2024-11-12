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

export type PtyProcessOptions = {
  env: Record<string, string>;
  path: string;
  args: string[];
  cwd?: string;
  initMessage?: string;
  /** Whether to use the ConPTY system on Windows. */
  useConpty: boolean;
};

export type IPtyProcess = {
  start(cols: number, rows: number): void;
  write(data: string): void;
  resize(cols: number, rows: number): void;
  dispose(): Promise<void>;
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
