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
} from 'teleport/services/recordings';
import { RiskLevel as RiskLevelValue } from 'teleport/services/recordings/types';
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

export type EventRowWithPositions = TimelineEventPositioned[];

type TimelineEventMeasured = SessionRecordingEvent & {
  title: string;
  textMetrics: TextMetrics;
};

type TimelineEventPositioned = TimelineEventMeasured & {
  endPosition: number;
  startPosition: number;
};

function getEventStyles(theme: DefaultTheme, event: SessionRecordingEvent) {
  switch (event.type) {
    case SessionRecordingEventType.Inactivity:
      return theme.colors.sessionRecordingTimeline.events.inactivity;
    case SessionRecordingEventType.Resize:
      return theme.colors.sessionRecordingTimeline.events.resize;
    case SessionRecordingEventType.Join:
      return theme.colors.sessionRecordingTimeline.events.join;
    case SessionRecordingEventType.Risk:
      switch (event.riskLevel) {
        case RiskLevelValue.Low:
          return {
            background: theme.colors.sessionRecording.riskLevels.low,
            text: 'rgba(0,0,0,0.7)',
          };
        case RiskLevelValue.Medium:
          return {
            background: theme.colors.sessionRecording.riskLevels.medium,
            text: 'rgba(0,0,0,0.7)',
          };
        case RiskLevelValue.High:
          return {
            background: theme.colors.sessionRecording.riskLevels.high,
            text: 'rgba(0,0,0,0.7)',
          };
        case RiskLevelValue.Critical:
          return {
            background: theme.colors.sessionRecording.riskLevels.critical,
            text: 'rgba(0,0,0,0.7)',
          };
        default:
          return theme.colors.sessionRecordingTimeline.events.default;
      }
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
    case SessionRecordingEventType.Risk:
      return event.description;
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
  // measuredEvents contains all events with their text metrics, sorted by start time
  private measuredEvents: TimelineEventMeasured[] = [];
  // rowsWithPositions are computed after calculate() is called, containing the position
  // of each event in the timeline, arranged into rows that account for text width
  private rowsWithPositions: EventRowWithPositions[] = [];

  constructor(
    ctx: CanvasRenderingContext2D,
    theme: DefaultTheme,
    duration: number,
    events: SessionRecordingEvent[]
  ) {
    super(ctx, theme, duration);

    this.measureEvents(events);
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

        // Width is already calculated in calculate() to include minimum text width
        const width = endX - startX;

        const styles = getEventStyles(this.theme, event);

        this.ctx.fillStyle = styles.background;
        this.ctx.beginPath();

        this.ctx.roundRect(x, y, width, EVENT_HEIGHT, EVENT_RADIUS);
        this.ctx.fill();

        this.ctx.fillStyle = styles.text;

        this.ctx.font = `bold 12px ${CANVAS_FONT}`;

        // Calculate text position and constrain it within the event box.
        const textWidth = event.textMetrics.width;
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

  calculate() {
    const rows: EventRowWithPositions[] = [];

    for (const event of this.measuredEvents) {
      const startPosition =
        LEFT_PADDING + (event.startTime / this.duration) * this.timelineWidth;
      const timeBasedEndPosition =
        LEFT_PADDING + (event.endTime / this.duration) * this.timelineWidth;

      // Calculate the minimum width needed to display the text
      const minWidth = event.textMetrics.width + 2 * TEXT_PADDING;
      const endPosition = Math.max(
        timeBasedEndPosition,
        startPosition + minWidth
      );

      const positionedEvent: TimelineEventPositioned = {
        ...event,
        endPosition,
        startPosition,
      };

      // Try to place the event in an existing row
      let placed = false;
      for (const row of rows) {
        const lastEventInRow = row[row.length - 1];

        // Check if there's space after the last event in this row
        if (lastEventInRow.endPosition <= startPosition) {
          row.push(positionedEvent);
          placed = true;
          break;
        }
      }

      // If no existing row has space, create a new row
      if (!placed) {
        rows.push([positionedEvent]);
      }
    }

    this.rowsWithPositions = rows;
    this.height = rows.length * EVENT_ROW_HEIGHT + EVENT_SECTION_PADDING;
  }

  getHeight() {
    return this.height;
  }

  getRowsWithPositions() {
    return this.rowsWithPositions;
  }

  private measureEvents(events: SessionRecordingEvent[]) {
    // Set font before measuring text
    this.ctx.font = `bold 12px ${CANVAS_FONT}`;

    const measuredEvents: TimelineEventMeasured[] = [];

    for (const event of events) {
      if (event.type === SessionRecordingEventType.Resize) {
        continue;
      }

      const title = getEventTitle(event);
      const textMetrics = this.ctx.measureText(title);

      measuredEvents.push({ ...event, textMetrics, title });
    }

    // Sort by start time for row placement algorithm
    this.measuredEvents = measuredEvents.sort(
      (a, b) => a.startTime - b.startTime
    );
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
