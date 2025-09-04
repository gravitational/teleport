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

export enum RequestType {
  Fetch = 1,
}

export enum ResponseType {
  Start = 1,
  Stop = 2,
  Error = 3,
  Batch = 9,
}

export interface FetchRequest {
  endTime: number;
  // requestCurrentScreen is used to request the stream to send the full state of the recording at the requested start time.
  // This is so the correct state of the recording can be rendered without having to return all the events up until
  // the requested start time.
  requestCurrentScreen?: boolean;
  requestId: number;
  startTime: number;
}

export interface BaseEvent<TType extends number = number> {
  requestId: number;
  timestamp: number;
  type: TType;
}

export interface BatchEvent<T> extends BaseEvent<ResponseType.Batch> {
  events: T[];
}

export interface ErrorEvent extends BaseEvent<ResponseType.Error> {
  error: string;
}

export interface StartEvent extends BaseEvent<ResponseType.Start> {}

export interface StopEvent extends BaseEvent<ResponseType.Stop> {
  endTime: number;
  startTime: number;
}

export type StreamEvent<T> =
  | StartEvent
  | StopEvent
  | ErrorEvent
  | BatchEvent<T>;

export function isBatchEvent<
  TEvent extends BaseEvent<TType>,
  TType extends number = number,
>(event: TEvent | StreamEvent<TEvent>): event is BatchEvent<TEvent> {
  return event.type === ResponseType.Batch;
}

export function isErrorEvent<
  TEvent extends BaseEvent<TType>,
  TType extends number = number,
>(event: TEvent | StreamEvent<TEvent>): event is ErrorEvent {
  return event.type === ResponseType.Error;
}

export function isStartEvent<
  TEvent extends BaseEvent<TType>,
  TType extends number = number,
>(event: TEvent | StreamEvent<TEvent>): event is StartEvent {
  return event.type === ResponseType.Start;
}

export function isStopEvent<
  TEvent extends BaseEvent<TType>,
  TType extends number = number,
>(event: TEvent | StreamEvent<TEvent>): event is StopEvent {
  return event.type === ResponseType.Stop;
}
