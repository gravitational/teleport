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

import type { DefaultTheme } from 'styled-components';

export interface TimelineRenderContext {
  containerHeight: number;
  containerWidth: number;
  eventsHeight: number;
  offset: number;
}

export abstract class TimelineCanvasRenderer {
  protected timelineWidth = 0;

  constructor(
    protected ctx: CanvasRenderingContext2D,
    protected theme: DefaultTheme,
    protected duration: number
  ) {}

  abstract _render(context: TimelineRenderContext): void;

  abstract calculate(): void;

  render(context: TimelineRenderContext) {
    this.ctx.save();

    this._render(context);

    this.ctx.restore();
  }

  setTimelineWidth(timelineWidth: number) {
    this.timelineWidth = timelineWidth;

    this.calculate();
  }
}
