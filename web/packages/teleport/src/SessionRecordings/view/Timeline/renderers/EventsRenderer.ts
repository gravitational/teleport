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

import {
  SessionRecordingEventType,
  type SessionRecordingEvent,
  type SessionRecordingMetadata,
} from 'teleport/services/recordings';
import { formatSessionRecordingDuration } from 'teleport/SessionRecordings/list/RecordingItem';

import {
  CANVAS_FONT,
  EVENT_ROW_HEIGHT,
  EVENT_SECTION_PADDING,
  LEFT_PADDING,
  RULER_HEIGHT,
} from '../constants';
import {
  TimelineCanvasRenderer,
  type TimelineRenderContext,
} from './TimelineCanvasRenderer';

export type EventRow = TimelineEventMeasured[];

export type EventRowWithPositions = TimelineEventPositioned[];

type TimelineEventMeasured = SessionRecordingEvent & {
  title: string;
  textMetrics: TextMetrics;
};

type TimelineEventPositioned = TimelineEventMeasured & {
  endPosition: number;
  startPosition: number;
};

function getEventStyles(theme: DefaultTheme, type: SessionRecordingEventType) {
  switch (type) {
    case SessionRecordingEventType.Inactivity:
      return theme.colors.sessionRecordingTimeline.events.inactivity;
    case SessionRecordingEventType.Resize:
      return theme.colors.sessionRecordingTimeline.events.resize;
    case SessionRecordingEventType.Join:
      return theme.colors.sessionRecordingTimeline.events.join;
    default:
      return theme.colors.sessionRecordingTimeline.events.default;
  }
}

function getEventTitle(event: SessionRecordingEvent) {
  switch (event.type) {
    case SessionRecordingEventType.Inactivity:
      return `Inactivity for ${formatSessionRecordingDuration(event.endTime - event.startTime)}`;
    case SessionRecordingEventType.Join:
      return `${event.user} joined`;
    default:
      return 'Event';
  }
}

const EVENT_RADIUS = 8;
const TEXT_PADDING = 8;
const EVENT_HEIGHT = 24;

/**
 * EventsRenderer renders each "event" that happens in a session recording, i.e. inactivity,
 * or if a user has joined. It does not render resize events, that is handled by a different renderer.
 *
 * Events are shown between the start time and end time of the event. The text in the event
 * scrolls along with the timeline, but is constrained to the event box.
 */
export class EventsRenderer extends TimelineCanvasRenderer {
  private height = 0;
  // rows are computed on creation, containing information such as text width to avoid
  // measuring text during render.
  private rows: EventRow[] = [];
  // rowsWithPositions are computed after calculate() is called, containing the position
  // of each event in the timeline, along with text metrics to avoid measuring text during render
  private rowsWithPositions: EventRowWithPositions[] = [];

  constructor(
    ctx: CanvasRenderingContext2D,
    theme: DefaultTheme,
    metadata: SessionRecordingMetadata
  ) {
    super(ctx, theme, metadata.duration);

    this.createEventRows(metadata.events);
  }

  _render({ containerWidth, offset }: TimelineRenderContext) {
    const eventRows = this.getVisibleEvents(offset, containerWidth);

    for (const [index, row] of eventRows.entries()) {
      for (const event of row) {
        const startX = event.startPosition;
        const endX = event.endPosition;
        this.ctx.save();

        const x = startX;

        const y =
          EVENT_SECTION_PADDING + index * EVENT_ROW_HEIGHT + RULER_HEIGHT;

        // calculate the minimum width of the event box from the text width + padding
        const textWidth = event.textMetrics.width;
        const minWidth = textWidth + 2 * TEXT_PADDING;

        const width = Math.max(endX - startX, minWidth);

        const styles = getEventStyles(this.theme, event.type);

        this.ctx.fillStyle = styles.background;
        this.ctx.beginPath();

        this.ctx.roundRect(x, y, width, EVENT_HEIGHT, EVENT_RADIUS);
        this.ctx.fill();

        this.ctx.fillStyle = styles.text;

        this.ctx.font = `bold 12px ${CANVAS_FONT}`;

        // Calculate text position and constrain it within the event box.
        const textHeight =
          event.textMetrics.actualBoundingBoxAscent +
          event.textMetrics.actualBoundingBoxDescent;

        const textPadding = 8;
        const visibleStart = Math.max(startX, -offset);
        const defaultTextOffset = Math.max(
          visibleStart - startX + textPadding,
          textPadding
        );

        let textOffset = defaultTextOffset;

        if (textWidth > 0) {
          // Ensure the text does not overflow the event box.
          const maxTextOffset = Math.max(
            textPadding,
            width - textWidth - textPadding
          );
          textOffset = Math.min(defaultTextOffset, maxTextOffset);
        }

        const textY =
          y +
          (EVENT_HEIGHT - textHeight) / 2 +
          event.textMetrics.actualBoundingBoxAscent;

        this.ctx.fillText(event.title, x + textOffset, textY + 1);

        this.ctx.restore();
      }
    }
  }

  // calculate calculates the positions of each event in the timeline, along with the
  // text metrics for each event title (to avoid measuring text during render).
  calculate() {
    const eventRowsWithPositions: EventRowWithPositions[] = [];

    for (let i = 0; i < this.rows.length; i++) {
      const row = this.rows[i];
      const positionedRow: EventRowWithPositions = [];

      for (const event of row) {
        const startPosition =
          LEFT_PADDING + (event.startTime / this.duration) * this.timelineWidth;
        const endPosition =
          LEFT_PADDING + (event.endTime / this.duration) * this.timelineWidth;

        positionedRow.push({
          ...event,
          endPosition,
          startPosition,
        });
      }

      eventRowsWithPositions.push(positionedRow);
    }

    this.rowsWithPositions = eventRowsWithPositions;
  }

  getHeight() {
    return this.height;
  }

  getRowsWithPositions() {
    return this.rowsWithPositions;
  }

  private createEventRows(events: SessionRecordingEvent[]) {
    const sortedEvents = [...events].sort((a, b) => a.startTime - b.startTime);
    const rows: EventRow[] = [];

    for (const event of sortedEvents) {
      if (event.type === SessionRecordingEventType.Resize) {
        continue;
      }

      let placed = false;

      const title = getEventTitle(event);

      for (const row of rows) {
        const lastEventInRow = row[row.length - 1];

        if (lastEventInRow.endTime <= event.startTime) {
          const textMetrics = this.ctx.measureText(title);

          row.push({ ...event, textMetrics, title });

          placed = true;
          break;
        }
      }

      if (!placed) {
        const textMetrics = this.ctx.measureText(title);

        rows.push([{ ...event, textMetrics, title }]);
      }
    }

    this.rows = rows;
    this.height = this.rows.length * EVENT_ROW_HEIGHT + EVENT_SECTION_PADDING;
  }

  private getVisibleEvents(
    offset: number,
    containerWidth: number
  ): EventRowWithPositions[] {
    const visibleStart = -offset - 100;
    const visibleEnd = -offset + containerWidth + 100;

    const visibleRows: EventRowWithPositions[] = [];

    for (const row of this.rowsWithPositions) {
      const visibleRow: EventRowWithPositions = [];

      for (const event of row) {
        if (
          event.startPosition <= visibleEnd &&
          event.endPosition >= visibleStart
        ) {
          visibleRow.push(event);
        }
      }

      if (visibleRow.length > 0) {
        visibleRows.push(visibleRow);
      }
    }

    return visibleRows;
  }
}
