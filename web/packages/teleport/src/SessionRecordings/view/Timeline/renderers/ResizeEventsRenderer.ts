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

export class ResizeEventsRenderer extends TimelineCanvasRenderer {
  private allEvents: ResizeTimelineEventPositioned[] = [];
  private readonly resizeEvents: SessionRecordingResizeEvent[] = [];

  private borderRadius = 6;
  private rowHeight = 20;
  private textPadding = 3.5;

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

  _render(context: TimelineRenderContext) {
    const eventsWithPositions = this.getEventPositions(
      context.offset,
      context.containerWidth,
      context.containerHeight
    );

    for (const { event, row, y } of eventsWithPositions) {
      this.renderEvent(context, event, row, y, eventsWithPositions);
    }
  }

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

    for (let i = this.allEvents.length - 1; i >= 0; i--) {
      const event = this.allEvents[i];
      let placed = false;

      for (let rowIndex = 0; rowIndex < rows.length; rowIndex++) {
        const hasOverlap = rows[rowIndex].some(
          existingEvent =>
            existingEvent.startPosition <
              event.startPosition + event.textMetrics.width &&
            event.startPosition <
              existingEvent.startPosition + existingEvent.textMetrics.width
        );

        if (!hasOverlap) {
          event.originalRow = rowIndex;
          rows[rowIndex].push(event);

          placed = true;

          break;
        }
      }

      if (!placed) {
        event.originalRow = rows.length;
        rows.push([event]);
      }
    }
  }

  private calculateLineSegments(
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

      if (otherEvent === currentEvent || otherRow >= rowIndex) continue;

      const textPadding = 8;
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

      const otherRectX = otherX - this.textPadding;
      const otherRectWidth = otherTextWidth + this.textPadding * 2;

      const otherTextHeight =
        otherEvent.textMetrics.actualBoundingBoxAscent +
        otherEvent.textMetrics.actualBoundingBoxDescent;
      const otherRectY =
        otherY -
        otherEvent.textMetrics.actualBoundingBoxAscent -
        this.textPadding;
      const otherRectHeight = otherTextHeight + this.textPadding * 2;

      if (
        lineX >= otherRectX &&
        lineX <= otherRectX + otherRectWidth &&
        lineStartY <= otherRectY + otherRectHeight &&
        lineEndY >= otherRectY
      ) {
        const newSegments: LineSegment[] = [];

        for (const segment of segments) {
          if (segment.start < otherRectY && segment.end > otherRectY) {
            newSegments.push({ end: otherRectY, start: segment.start });
          }

          if (
            segment.start < otherRectY + otherRectHeight &&
            segment.end > otherRectY + otherRectHeight
          ) {
            newSegments.push({
              end: segment.end,
              start: otherRectY + otherRectHeight,
            });
          }

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

  private getEventPositions(
    offset: number,
    containerWidth: number,
    containerHeight: number
  ): EventWithCalculatedPosition[] {
    const viewStart = -offset;
    const viewEnd = -offset + containerWidth;
    const buffer = containerWidth / 2;

    const activeEvents = this.allEvents.filter(
      event =>
        event.endPosition > viewStart - buffer &&
        event.startPosition < viewEnd + buffer
    );

    const sortedEvents = [...activeEvents].sort(
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

    return bottom - row * this.rowHeight;
  }

  private renderEvent(
    context: TimelineRenderContext,
    event: ResizeTimelineEventPositioned,
    row: number,
    y: number,
    allEvents: EventWithCalculatedPosition[]
  ): void {
    const textPadding = 8;
    const visibleStart = Math.max(event.startPosition, -context.offset);
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

    this.renderResizeEventBox(context, event, x, y);
    this.renderResizeEventText(context, event, x, y);
    this.renderResizeEventLine(context, event, x, y, row, allEvents);
  }

  private renderResizeEventBox(
    context: TimelineRenderContext,
    event: ResizeTimelineEventPositioned,
    x: number,
    y: number
  ): void {
    const textWidth = event.textMetrics.width;
    const textHeight =
      event.textMetrics.actualBoundingBoxAscent +
      event.textMetrics.actualBoundingBoxDescent;

    const rectX = x - this.textPadding;
    const rectY =
      y - event.textMetrics.actualBoundingBoxAscent - this.textPadding;
    const rectWidth = textWidth + this.textPadding * 2;
    const rectHeight = textHeight + this.textPadding * 2;

    this.ctx.save();

    this.ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
    this.ctx.beginPath();
    this.ctx.roundRect(rectX, rectY, rectWidth, rectHeight, this.borderRadius);
    this.ctx.fill();

    this.ctx.save();
    this.ctx.shadowColor = 'rgba(0, 0, 0, .05)';
    this.ctx.shadowBlur = 4;
    this.ctx.shadowOffsetX = 0;
    this.ctx.shadowOffsetY = 0;

    this.ctx.fillStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.background;
    this.ctx.beginPath();
    this.ctx.roundRect(rectX, rectY, rectWidth, rectHeight, this.borderRadius);
    this.ctx.fill();
    this.ctx.strokeStyle =
      this.theme.colors.sessionRecordingTimeline.events.resize.border;
    this.ctx.lineWidth = 1.5;
    this.ctx.stroke();

    this.ctx.restore();
    this.ctx.restore();
  }

  private renderResizeEventLine(
    context: TimelineRenderContext,
    event: ResizeTimelineEventPositioned,
    x: number,
    y: number,
    rowIndex: number,
    allEvents: EventWithCalculatedPosition[]
  ): void {
    const textWidth = event.textMetrics.width;
    const textHeight =
      event.textMetrics.actualBoundingBoxAscent +
      event.textMetrics.actualBoundingBoxDescent;

    const rectY =
      y - event.textMetrics.actualBoundingBoxAscent - this.textPadding;
    const rectHeight = textHeight + this.textPadding * 2;

    const lineX = x + textWidth / 2;
    const lineStartY = rectY + rectHeight;
    const lineEndY = context.containerHeight;

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
      context.offset,
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
    options: TimelineRenderContext,
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
