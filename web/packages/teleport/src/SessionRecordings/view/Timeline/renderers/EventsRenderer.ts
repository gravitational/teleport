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

type EventRow = TimelineEventMeasured[];

type EventRowWithPositions = TimelineEventPositioned[];

type TimelineEventMeasured = SessionRecordingEvent & {
  title: string;
  textWidth: number;
};

type TimelineEventPositioned = TimelineEventMeasured & {
  endPosition: number;
  startPosition: number;
  textMetrics: TextMetrics;
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

export class EventsRenderer extends TimelineCanvasRenderer {
  private height = 0;
  private rows: EventRow[] = [];
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
        const width = endX - startX;
        const height = 24;
        const radius = 8;

        const styles = getEventStyles(this.theme, event.type);

        this.ctx.fillStyle = styles.background;

        this.ctx.beginPath();
        this.ctx.roundRect(x, y, width, height, radius);

        this.ctx.fill();
        this.ctx.fillStyle = styles.text;

        this.ctx.font = `bold 12px ${CANVAS_FONT}`;

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
          const maxTextOffset = Math.max(
            textPadding,
            width - textWidth - textPadding
          );
          textOffset = Math.min(defaultTextOffset, maxTextOffset);
        }

        const textY =
          y +
          (height - textHeight) / 2 +
          event.textMetrics.actualBoundingBoxAscent;

        this.ctx.fillText(event.title, x + textOffset, textY + 1);

        this.ctx.restore();
      }
    }
  }

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

        this.ctx.save();

        this.ctx.font = `bold 12px ${CANVAS_FONT}`;

        const textMetrics = this.ctx.measureText(event.title);

        this.ctx.restore();

        positionedRow.push({
          ...event,
          endPosition,
          startPosition,
          textMetrics,
        });
      }

      eventRowsWithPositions.push(positionedRow);
    }

    this.rowsWithPositions = eventRowsWithPositions;
  }

  getHeight() {
    return this.height;
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
          const textWidth = this.ctx.measureText(title).width;

          row.push({ ...event, textWidth, title });

          placed = true;
          break;
        }
      }

      if (!placed) {
        const textWidth = this.ctx.measureText(title).width;

        rows.push([{ ...event, textWidth, title }]);
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
