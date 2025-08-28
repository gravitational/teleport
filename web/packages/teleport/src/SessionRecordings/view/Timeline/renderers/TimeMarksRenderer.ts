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

import type { DefaultTheme } from 'styled-components';

import type { SessionRecordingMetadata } from 'teleport/services/recordings';

import { CANVAS_FONT, LEFT_PADDING } from '../constants';
import {
  TimelineCanvasRenderer,
  type TimelineRenderContext,
} from './TimelineCanvasRenderer';

interface SubTick {
  height: number;
  position: number;
}

interface TimeMarker {
  absolute: boolean;
  label: string;
  position: number;
  time: number;
}

export class TimeMarkersRenderer extends TimelineCanvasRenderer {
  private subTicks: SubTick[] = [];
  private timeMarkers: TimeMarker[] = [];
  private showAbsoluteTime = false;

  private readonly startTime: number;

  constructor(
    ctx: CanvasRenderingContext2D,
    theme: DefaultTheme,
    metadata: SessionRecordingMetadata
  ) {
    super(ctx, theme, metadata.duration);
    this.startTime = metadata.startTime;
  }

  _render({ containerWidth, offset }: TimelineRenderContext) {
    const { markers, subTicks } = this.getVisibleTimeMarkers(
      offset,
      containerWidth
    );

    this.ctx.font = `10px ${CANVAS_FONT}`;

    for (const marker of markers) {
      const textWidth = this.ctx.measureText(marker.label).width;
      const x = marker.position + LEFT_PADDING;

      this.ctx.fillStyle =
        this.theme.colors.sessionRecordingTimeline.timeMarks.primary;
      this.ctx.fillRect(x, 0, 1, 10);

      if (marker.absolute) {
        this.ctx.font = `bold 10px ${CANVAS_FONT}`;
      }

      this.ctx.fillStyle = marker.absolute
        ? this.theme.colors.sessionRecordingTimeline.timeMarks.absolute
        : this.theme.colors.sessionRecordingTimeline.timeMarks.text;

      this.ctx.fillText(marker.label, x - textWidth / 2, 24);

      if (marker.absolute) {
        this.ctx.font = `10px ${CANVAS_FONT}`;
      }
    }

    this.ctx.fillStyle =
      this.theme.colors.sessionRecordingTimeline.timeMarks.secondary;

    for (const tick of subTicks) {
      this.ctx.fillRect(tick.position + LEFT_PADDING, 0, 1, tick.height);
    }
  }

  calculate() {
    const markers: TimeMarker[] = [];

    let interval = 1000;
    let pixelsPerSecond = (this.timelineWidth / this.duration) * 1000;

    if (pixelsPerSecond < 10) interval = 10000;
    else if (pixelsPerSecond < 50) interval = 5000;

    for (let time = 0; time < this.duration + interval; time += interval) {
      // if show absolute time is enabled, change the 0:00, 1:00, etc, labels to absolute time
      const absolute = this.showAbsoluteTime && (time / 1000) % 60 === 0;
      const label = absolute
        ? formatAbsoluteTime(this.startTime + time)
        : formatRelativeTime(time / 1000);

      markers.push({
        absolute,
        label,
        position: (time / this.duration) * this.timelineWidth,
        time,
      });
    }

    const subTicks: SubTick[] = [];

    const markerSpacing = (interval / this.duration) * this.timelineWidth;

    let numSubticks = 0;
    if (markerSpacing > 300) numSubticks = 9;
    else if (markerSpacing > 200) numSubticks = 7;
    else if (markerSpacing > 150) numSubticks = 4;
    else if (markerSpacing > 100) numSubticks = 3;
    else if (markerSpacing > 50) numSubticks = 1;

    for (let i = 0; i < markers.length - 1; i++) {
      const currentMarker = markers[i];

      for (let j = 1; j <= numSubticks; j++) {
        const fraction = j / (numSubticks + 1);
        const position = currentMarker.position + markerSpacing * fraction;

        let height = 4;
        if (numSubticks % 2 === 1) {
          height = j === Math.ceil(numSubticks / 2) ? 6 : 4;
        } else {
          const mid1 = numSubticks / 2;
          const mid2 = mid1 + 1;
          height = j === mid1 || j === mid2 ? 6 : 4;
        }

        subTicks.push({ height, position });
      }
    }

    this.timeMarkers = markers;
    this.subTicks = subTicks;
  }

  setShowAbsoluteTime(show: boolean) {
    this.showAbsoluteTime = show;

    this.calculate();
  }

  private getVisibleTimeMarkers(offset: number, containerWidth: number) {
    const visibleStart = -offset - 100;
    const visibleEnd = -offset + containerWidth + 100;

    const visibleMarkers: TimeMarker[] = [];
    const visibleSubTicks: SubTick[] = [];

    for (const marker of this.timeMarkers) {
      if (marker.position >= visibleStart && marker.position <= visibleEnd) {
        visibleMarkers.push(marker);
      }
    }

    for (const subTick of this.subTicks) {
      if (subTick.position >= visibleStart && subTick.position <= visibleEnd) {
        visibleSubTicks.push(subTick);
      }
    }

    return { markers: visibleMarkers, subTicks: visibleSubTicks };
  }
}

function formatRelativeTime(seconds: number) {
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);

  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function formatAbsoluteTime(timestamp: number) {
  const date = new Date(timestamp);

  return date
    .toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    .replace(' AM', 'am')
    .replace(' PM', 'pm');
}
