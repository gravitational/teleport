/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { LEFT_PADDING } from '../constants';
import {
  TimelineCanvasRenderer,
  type TimelineRenderContext,
} from './TimelineCanvasRenderer';

export class ProgressLineRenderer extends TimelineCanvasRenderer {
  private currentTime = 0;
  private position = 0;

  _render({ containerHeight }: TimelineRenderContext) {
    this.ctx.strokeStyle =
      this.theme.colors.sessionRecordingTimeline.progressLine;
    this.ctx.lineWidth = 2;

    this.ctx.beginPath();
    this.ctx.moveTo(this.position, 0);
    this.ctx.lineTo(this.position, containerHeight);
    this.ctx.stroke();
  }

  calculate() {
    this.position =
      (this.currentTime / this.duration) * this.timelineWidth + LEFT_PADDING;
  }

  setCurrentTime(currentTime: number) {
    this.currentTime = currentTime;
    this.calculate();
  }
}
