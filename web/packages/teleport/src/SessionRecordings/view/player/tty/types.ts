/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import type { BaseEvent } from '../../stream/types';

export enum EventType {
  SessionStart = 4,
  SessionPrint = 5,
  SessionEnd = 6,
  Resize = 7,
  Screen = 8,
}

export interface ResizeEvent extends BaseEvent<EventType.Resize> {
  terminalSize: TerminalSize;
}

export interface ScreenEvent extends BaseEvent<EventType.Screen> {
  screen: SerializedTerminal;
}

export interface SerializedTerminal {
  cols: number;
  cursorX: number;
  cursorY: number;
  data: Uint8Array;
  rows: number;
}

export interface SessionEndEvent extends BaseEvent<EventType.SessionEnd> {}

export interface SessionPrintEvent extends BaseEvent<EventType.SessionPrint> {
  data: Uint8Array;
}

export interface SessionStartEvent extends BaseEvent<EventType.SessionStart> {
  terminalSize: TerminalSize;
}

export interface TerminalSize {
  cols: number;
  rows: number;
}

export type TtyEvent =
  | ResizeEvent
  | ScreenEvent
  | SessionEndEvent
  | SessionPrintEvent
  | SessionStartEvent;
