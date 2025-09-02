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

  abstract apply(event: TEvent): void;
  abstract handle(event: TEvent): boolean;
  abstract clear(): void;

  fit(): void {}

  onPlay(): void {}
  onPause(): void {}
  // eslint-disable-next-line unused-imports/no-unused-vars
  onSeek(time: number): void {}
  onStop(): void {}
}
