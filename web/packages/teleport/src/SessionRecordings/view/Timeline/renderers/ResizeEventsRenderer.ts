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

import {
  SessionRecordingEventType,
  type SessionRecordingMetadata,
  type SessionRecordingResizeEvent,
} from 'teleport/services/recordings';

import { LEFT_PADDING } from '../constants';
import {
  TimelineCanvasRenderer,
  type TimelineRenderContext,
} from './TimelineCanvasRenderer';

interface EventWithCalculatedPosition {
  event: ResizeTimelineEventPositioned;
  row: number;
  y: number;
}

interface LineSegment {
  end: number;
  start: number;
}

interface ResizeTimelineEventPositioned extends SessionRecordingResizeEvent {
  title: string;
  endPosition: number;
  originalRow: number;
  startPosition: number;
  textMetrics: TextMetrics;
}

function calculateEventEndTimes(
  resizeEvents: SessionRecordingResizeEvent[],
  duration: number
) {
  const events: SessionRecordingResizeEvent[] = [];

  for (const event of resizeEvents) {
    const lastResizeEvent = events.findLast(e => e.startTime < event.startTime);

    if (lastResizeEvent) {
      lastResizeEvent.endTime = event.startTime - 1;
    }

    events.push(event);
  }

  const lastResizeEvent = events.findLast(
    e => e.type === SessionRecordingEventType.Resize
  );

  if (lastResizeEvent) {
    lastResizeEvent.endTime = duration;
  }

  return events;
}

const TEXT_PADDING_X = 8;
const TEXT_PADDING_Y = 3.5;
const ROW_HEIGHT = 20;
const BORDER_RADIUS = 6;

/**
 * ResizeEventsRenderer is responsible for rendering terminal resize events on the timeline,
 * in the bottom left.
 *
 * It renders the size in a mono font, `ColsxRows`, inside a rounded rectangle with a semi-transparent background.
 * The resize events will move along with the timeline as the user scrolls, until it encounters another
 * resize event, at which point that resize event will "push" the previous event out of the way.
 *
 * If there are multiple resize events that would overlap, they will be stacked vertically.
 * Each event will have a line connecting it to the bottom of the timeline, unless that line
 * would intersect with another event's box, in which case the line will stop just before the box.
 */
export class ResizeEventsRenderer extends TimelineCanvasRenderer {
  private allEvents: ResizeTimelineEventPositioned[] = [];
  private readonly resizeEvents: SessionRecordingResizeEvent[] = [];

  constructor(
    ctx: CanvasRenderingContext2D,
    theme: DefaultTheme,
    metadata: SessionRecordingMetadata
  ) {
    super(ctx, theme, metadata.duration);

    this.resizeEvents = calculateEventEndTimes(
      metadata.events.filter(
        event => event.type === SessionRecordingEventType.Resize
      ),
      metadata.duration
    );
  }

  _render({ containerHeight, containerWidth, offset }: TimelineRenderContext) {
    const eventsWithPositions = this.getEventPositions(
      offset,
      containerWidth,
      containerHeight
    );

    for (const { event, row, y } of eventsWithPositions) {
      this.renderEvent(
        event,
        containerHeight,
        offset,
        row,
        y,
        eventsWithPositions
      );
    }
  }

  // calculate determines the position of each resize event on the timeline,
  // and how many rows are needed to display them without overlap.
  // It also calculates the text metrics for each event title.
  calculate() {
    this.allEvents = [];

    for (let i = 0; i < this.resizeEvents.length; i++) {
      const event = this.resizeEvents[i];

      const startPosition =
        (event.startTime / this.duration) * this.timelineWidth + LEFT_PADDING;
      const endPosition =
        (event.endTime / this.duration) * this.timelineWidth + LEFT_PADDING;

      this.ctx.save();

      this.ctx.font = `bold 10px ${this.theme.fonts.mono}`;

      const title = `${event.cols}x${event.rows}`;

      const textMetrics = this.ctx.measureText(title);

      this.ctx.restore();

      this.allEvents.push({
        ...event,
        title,
        endPosition,
        originalRow: 0,
        startPosition,
        textMetrics,
      });
    }

    const rows: ResizeTimelineEventPositioned[][] = [];

    // calculate the rows for each event, starting from the last event
    // this ensures that events stack upwards and to the right
    for (let i = this.allEvents.length - 1; i >= 0; i--) {
      const event = this.allEvents[i];
      let placed = false;

      for (let rowIndex = 0; rowIndex < rows.length; rowIndex++) {
        // check for overlap with existing events in this row
        const hasOverlap = rows[rowIndex].some(
          existingEvent =>
            existingEvent.startPosition <
              event.startPosition + event.textMetrics.width &&
            event.startPosition <
              existingEvent.startPosition + existingEvent.textMetrics.width
        );

        if (!hasOverlap) {
          // no overlap, place the event in this row
          event.originalRow = rowIndex;
          rows[rowIndex].push(event);

          placed = true;

          break;
        }
      }

      if (!placed) {
        // no suitable row found, create a new row
        event.originalRow = rows.length;
        rows.push([event]);
      }
    }
  }

  getAllEvents() {
    return this.allEvents;
  }

  // calculateLineSegments calculates the segments of the vertical line that connects
  // the event box to the bottom of the timeline, avoiding intersections with other event boxes.
  calculateLineSegments(
    lineX: number,
    lineStartY: number,
    lineEndY: number,
    rowIndex: number,
    currentEvent: ResizeTimelineEventPositioned,
    offset: number,
    allEvents: EventWithCalculatedPosition[]
  ): LineSegment[] {
    let segments: LineSegment[] = [{ end: lineEndY, start: lineStartY }];

    for (const eventPos of allEvents) {
      const { event: otherEvent, row: otherRow, y: otherY } = eventPos;

      // Skip the current event and events in the same or lower rows
      if (otherEvent === currentEvent || otherRow >= rowIndex) {
        continue;
      }

      const textPadding = TEXT_PADDING_X;

      // Calculate the bounding box of the other event
      // Adjust for the current offset to determine visibility
      // If the other event is partially off-screen, adjust the text offset accordingly
      // This ensures the bounding box is accurate even when the timeline is scrolled
      // and the event is not fully visible

      const visibleStart = Math.max(otherEvent.startPosition, -offset);
      const defaultTextOffset = Math.max(
        visibleStart - otherEvent.startPosition + textPadding,
        textPadding
      );

      const width = otherEvent.endPosition - otherEvent.startPosition;
      let textOffset = defaultTextOffset;

      if (otherEvent.textMetrics.width > 0) {
        const maxTextOffset = Math.max(
          textPadding,
          width - otherEvent.textMetrics.width - textPadding
        );
        textOffset = Math.min(defaultTextOffset, maxTextOffset);
      }

      const otherX = otherEvent.startPosition + textOffset;
      const otherTextWidth = otherEvent.textMetrics.width;

      const otherRectX = otherX - TEXT_PADDING_X;
      const otherRectWidth = otherTextWidth + TEXT_PADDING_X * 2;

      const otherTextHeight =
        otherEvent.textMetrics.actualBoundingBoxAscent +
        otherEvent.textMetrics.actualBoundingBoxDescent;
      const otherRectY =
        otherY -
        otherEvent.textMetrics.actualBoundingBoxAscent -
        TEXT_PADDING_Y;
      const otherRectHeight = otherTextHeight + TEXT_PADDING_Y * 2;

      // Check if the vertical line intersects with the other event's bounding box
      if (
        lineX >= otherRectX &&
        lineX <= otherRectX + otherRectWidth &&
        lineStartY <= otherRectY + otherRectHeight &&
        lineEndY >= otherRectY
      ) {
        const newSegments: LineSegment[] = [];

        for (const segment of segments) {
          // If the segment intersects with the other event's bounding box, split it
          if (segment.start < otherRectY && segment.end > otherRectY) {
            newSegments.push({ end: otherRectY, start: segment.start });
          }

          // If the segment intersects with the bottom of the other event's bounding box, split it
          if (
            segment.start < otherRectY + otherRectHeight &&
            segment.end > otherRectY + otherRectHeight
          ) {
            newSegments.push({
              end: segment.end,
              start: otherRectY + otherRectHeight,
            });
          }

          // If the segment is completely above or below the other event's bounding box, keep it as is
          if (
            segment.start >= otherRectY + otherRectHeight ||
            segment.end <= otherRectY
          ) {
            newSegments.push(segment);
          }
        }

        segments = newSegments;
      }
    }

    return segments;
  }

  // getEventPositions filters and positions events based on the current view offset and container size.
  // It returns events that are within the visible area and assigns them to rows to avoid overlap.
  private getEventPositions(
    offset: number,
    containerWidth: number,
    containerHeight: number
  ): EventWithCalculatedPosition[] {
    const viewStart = -offset;
    const viewEnd = -offset + containerWidth;

    const activeEvents = this.allEvents.filter(
      event => event.endPosition > viewStart && event.startPosition < viewEnd
    );

    const sortedEvents = activeEvents.toSorted(
      (a, b) => a.startPosition - b.startPosition
    );

    const eventsWithPositions: EventWithCalculatedPosition[] = [];
    const rowEndPositions = new Map<number, number>();

    for (const event of sortedEvents) {
      const baseRow = event.originalRow;

      let targetRow = baseRow;

      const padding = 10;

      for (let i = 0; i < baseRow; i++) {
        const rowEnd = rowEndPositions.get(i) ?? -Infinity;

        // Check if the event can fit in this row without overlapping
        if (event.startPosition >= rowEnd + padding) {
          targetRow = i;

          break;
        }
      }

      const y = this.getYForRow(targetRow, containerHeight);

      eventsWithPositions.push({
        event,
        row: targetRow,
        y,
      });

      rowEndPositions.set(targetRow, event.endPosition);
    }

    return eventsWithPositions;
  }

  private getYForRow(row: number, containerHeight: number): number {
    const bottom = containerHeight - 10;

    return bottom - row * ROW_HEIGHT;
  }

  private renderEvent(
    event: ResizeTimelineEventPositioned,
    containerHeight: number,
    offset: number,
    row: number,
    y: number,
    allEvents: EventWithCalculatedPosition[]
  ): void {
    const textPadding = 8;
    const visibleStart = Math.max(event.startPosition, -offset);
    const defaultTextOffset = Math.max(
      visibleStart - event.startPosition + textPadding,
      textPadding
    );

    const width = event.endPosition - event.startPosition;
    let textOffset = defaultTextOffset;

    if (event.textMetrics.width > 0) {
      const maxTextOffset = Math.max(
        textPadding,
        width - event.textMetrics.width - textPadding
      );
      textOffset = Math.min(defaultTextOffset, maxTextOffset);
    }

    const x = event.startPosition + textOffset;

    this.renderResizeEventBox(event, x, y);
    this.renderResizeEventText(event, x, y);
    this.renderResizeEventLine(
      event,
      x,
      y,
      containerHeight,
      offset,
      row,
      allEvents
    );
  }

  private renderResizeEventBox(
    event: ResizeTimelineEventPositioned,
    x: number,
    y: number
  ): void {
    const textWidth = event.textMetrics.width;
    const textHeight =
      event.textMetrics.actualBoundingBoxAscent +
      event.textMetrics.actualBoundingBoxDescent;

    const rectX = x - TEXT_PADDING_X;
    const rectY =
      y - event.textMetrics.actualBoundingBoxAscent - TEXT_PADDING_Y;
    const rectWidth = textWidth + TEXT_PADDING_X * 2;
    const rectHeight = textHeight + TEXT_PADDING_Y * 2;

    this.ctx.save();

    // Draw semi-transparent background for better readability
    this.ctx.fillStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.semiBackground;
    this.ctx.beginPath();
    this.ctx.roundRect(rectX, rectY, rectWidth, rectHeight, BORDER_RADIUS);
    this.ctx.fill();

    // Draw main box with shadow
    this.ctx.shadowColor = 'rgba(0, 0, 0, .05)';
    this.ctx.shadowBlur = 4;
    this.ctx.shadowOffsetX = 0;
    this.ctx.shadowOffsetY = 0;

    // Main box background and border
    this.ctx.fillStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.background;
    this.ctx.beginPath();
    this.ctx.roundRect(rectX, rectY, rectWidth, rectHeight, BORDER_RADIUS);
    this.ctx.fill();
    this.ctx.strokeStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.border;
    this.ctx.lineWidth = 1.5;
    this.ctx.stroke();

    this.ctx.restore();
  }

  private renderResizeEventLine(
    event: ResizeTimelineEventPositioned,
    x: number,
    y: number,
    containerHeight: number,
    offset: number,
    rowIndex: number,
    allEvents: EventWithCalculatedPosition[]
  ): void {
    const textWidth = event.textMetrics.width;
    const textHeight =
      event.textMetrics.actualBoundingBoxAscent +
      event.textMetrics.actualBoundingBoxDescent;

    const rectY =
      y - event.textMetrics.actualBoundingBoxAscent - TEXT_PADDING_Y;
    const rectHeight = textHeight + TEXT_PADDING_Y * 2;

    const lineX = x + textWidth / 2;
    const lineStartY = rectY + rectHeight;
    const lineEndY = containerHeight;

    this.ctx.save();

    this.ctx.strokeStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.border;
    this.ctx.lineWidth = 1.5;

    const segments = this.calculateLineSegments(
      lineX,
      lineStartY,
      lineEndY,
      rowIndex,
      event,
      offset,
      allEvents
    );

    for (const segment of segments) {
      this.ctx.beginPath();
      this.ctx.moveTo(lineX, segment.start);
      this.ctx.lineTo(lineX, segment.end);
      this.ctx.stroke();
    }

    this.ctx.restore();
  }

  private renderResizeEventText(
    event: ResizeTimelineEventPositioned,
    x: number,
    y: number
  ): void {
    this.ctx.save();

    this.ctx.fillStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.text;
    this.ctx.font = `bold 10px ${this.theme.fonts.mono}`;

    this.ctx.fillText(event.title, x, y);
    this.ctx.restore();
  }
}
