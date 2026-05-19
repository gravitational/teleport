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

import type { BaseEvent } from '../stream/types';

export abstract class Player<
  TEvent extends BaseEvent<TEventType>,
  TEventType extends number = number,
> {
  abstract init(element: HTMLElement): void;
  abstract destroy(): void;

  /**
   * clear is called to clear the current state of the player. This is called when seeking to a time before the
   * current time, to reset the state of the player before receiving the state of the screen.
   */
  abstract clear(): void;

  /**
   * applyEvent is called to apply a single event to the player, at the correct moment in time.
   * This is typically used for events that change the state of the player, such as print or resize.
   *
   * @param event The event to apply.
   *
   * @returns void
   */
  abstract applyEvent(event: TEvent): void;

  /**
   * handleEvent is called to handle an event that needs to be handled as soon as it is received from
   * the stream, regardless of the current playback time. Typically this would be for receiving the
   * full state of the screen.
   *
   * @param event The event to handle.
   *
   * @returns true if the event was handled (and therefore should not be added to the stream buffer), false otherwise.
   */
  abstract handleEvent(event: TEvent): boolean;

  // fit is called to resize the player to fit its container. This is typically player specific.
  fit(): void {}

  // Event hooks for subclasses to optionally implement.
  onPlay(): void {}
  onPause(): void {}
  // eslint-disable-next-line unused-imports/no-unused-vars
  onSeek(time: number): void {}
  onStop(): void {}
}
