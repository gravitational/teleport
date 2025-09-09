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

import 'jest-canvas-mock';

import { darkTheme } from 'design/theme';

import {
  SessionRecordingEventType,
  SessionRecordingMetadata,
  SessionRecordingResizeEvent,
} from 'teleport/services/recordings';
import type { TimelineRenderContext } from 'teleport/SessionRecordings/view/Timeline/renderers/TimelineCanvasRenderer';

import { ResizeEventsRenderer } from './ResizeEventsRenderer';

// TODO(ryan): share some common mocks between all tests once everything is merged
const mockMetadata: SessionRecordingMetadata = {
  startTime: 1609459200000, // Jan 1, 2021 12:00:00 AM UTC
  endTime: 1609462800000, // Jan 1, 2021 1:00:00 AM UTC
  duration: 3600000, // 1 hour in milliseconds
  user: 'testuser',
  resourceName: 'test-server',
  clusterName: 'test-cluster',
  events: [],
  startCols: 80,
  startRows: 24,
  type: 'ssh',
};

function createRenderer(
  metadata?: Partial<SessionRecordingMetadata>,
  events?: SessionRecordingResizeEvent[]
) {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  const renderer = new ResizeEventsRenderer(ctx, darkTheme, {
    ...mockMetadata,
    events: events || [],
    ...metadata,
  });

  return { ctx, renderer };
}

function createResizeEvent(
  startTime: number,
  cols: number,
  rows: number,
  endTime?: number
): SessionRecordingResizeEvent {
  return {
    type: SessionRecordingEventType.Resize,
    startTime,
    endTime: endTime || 0,
    cols,
    rows,
  };
}

describe('resize event calculation', () => {
  it('should calculate end times for resize events', () => {
    const events = [
      createResizeEvent(0, 80, 24),
      createResizeEvent(10000, 100, 30),
      createResizeEvent(20000, 120, 40),
    ];

    const { renderer } = createRenderer(
      {
        duration: 30 * 1000, // 30 seconds
      },
      events
    );

    renderer.calculate();

    // Check that events have proper end times
    // First event ends at second event start - 1
    // Second event ends at third event start - 1
    // Third event ends at duration

    const allEvents = renderer.getAllEvents();
    expect(allEvents[0].endTime).toBe(9999);
    expect(allEvents[1].endTime).toBe(19999);
    expect(allEvents[2].endTime).toBe(30000);
  });

  it('should calculate text metrics and positions', () => {
    const events = [createResizeEvent(5000, 120, 40)];

    const { ctx, renderer } = createRenderer(
      {
        duration: 10 * 1000, // 10 seconds
      },
      events
    );

    renderer.setTimelineWidth(1000);
    renderer.calculate();

    const allEvents = renderer.getAllEvents();

    const expectedWidth = ctx.measureText('120x40').width;

    expect(allEvents[0].title).toBe('120x40');
    expect(allEvents[0].startPosition).toBeDefined();
    expect(allEvents[0].endPosition).toBeDefined();
    expect(allEvents[0].textMetrics).toBeDefined();
    expect(allEvents[0].textMetrics.width).toBe(expectedWidth);
  });

  it('should assign events to rows to avoid overlap', () => {
    const events = [
      createResizeEvent(0, 80, 24),
      createResizeEvent(5000, 100, 30),
      createResizeEvent(5010, 120, 40),
      createResizeEvent(15000, 140, 50),
    ];

    const { renderer } = createRenderer(
      {
        duration: 20 * 1000, // 20 seconds
      },
      events
    );

    renderer.setTimelineWidth(100);
    renderer.calculate();

    const allEvents = renderer.getAllEvents();

    expect(allEvents[0].originalRow).toBe(0);
    expect(allEvents[1].originalRow).toBe(1);
    expect(allEvents[2].originalRow).toBe(0);
    expect(allEvents[3].originalRow).toBe(0);

    // Check that overlapping events are not in the same row
    const rows = new Map<number, any[]>();
    for (const event of allEvents) {
      if (!rows.has(event.originalRow)) {
        rows.set(event.originalRow, []);
      }

      rows.get(event.originalRow).push(event);
    }

    // Verify no overlap within each row
    for (const [, eventsInRow] of rows) {
      for (let i = 0; i < eventsInRow.length - 1; i++) {
        for (let j = i + 1; j < eventsInRow.length; j++) {
          const event1 = eventsInRow[i];
          const event2 = eventsInRow[j];

          const hasOverlap =
            event1.startPosition <
              event2.startPosition + event2.textMetrics.width &&
            event2.startPosition <
              event1.startPosition + event1.textMetrics.width;

          expect(hasOverlap).toBe(false);
        }
      }
    }
  });
});

describe('resize event rendering', () => {
  it('should render resize events within visible area', () => {
    const events = [
      createResizeEvent(5000, 80, 24),
      createResizeEvent(15000, 100, 30),
    ];

    const { ctx, renderer } = createRenderer(
      {
        duration: 20 * 1000, // 20 seconds
      },
      events
    );

    renderer.setTimelineWidth(1000);
    renderer.calculate();

    const renderContext: TimelineRenderContext = {
      containerWidth: 1100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();

    // Should render text for both resize events
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    expect(fillTextCalls).toHaveLength(2);
    expect(fillTextCalls[0].props.text).toBe('80x24');
    expect(fillTextCalls[1].props.text).toBe('100x30');

    const ctxEvents = ctx.__getEvents();
    const roundRects = ctxEvents.filter(e => e.type === 'roundRect');

    expect(roundRects).toHaveLength(2 * 2); // Each event has two roundRects (background and border)
  });

  it('should only render events within view buffer', () => {
    const events = [
      createResizeEvent(0, 80, 24),
      createResizeEvent(50000, 100, 30), // Far offscreen
      createResizeEvent(10000, 120, 40),
    ];

    const { ctx, renderer } = createRenderer(
      {
        duration: 60 * 1000, // 60 seconds
      },
      events
    );

    renderer.setTimelineWidth(1000);
    renderer.calculate();

    const renderContext: TimelineRenderContext = {
      containerWidth: 200,
      offset: 0, // Viewing start of timeline
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // Should only render first and third events (second is far offscreen)
    expect(fillTextCalls).toHaveLength(2);
    expect(fillTextCalls[0].props.text).toBe('80x24');
    expect(fillTextCalls[1].props.text).toBe('120x40');
  });

  it('should render vertical lines connecting events to timeline', () => {
    const events = [createResizeEvent(5000, 80, 24)];

    const { ctx, renderer } = createRenderer(
      {
        duration: 10000, // 10 seconds
      },
      events
    );

    renderer.setTimelineWidth(1000);
    renderer.calculate();

    const renderContext: TimelineRenderContext = {
      containerWidth: 1100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const paths = ctx.__getPath();

    expect(paths).toHaveLength(3);

    expect(paths[0].type).toBe('beginPath');
    expect(paths[1].type).toBe('moveTo');

    const moveToX = paths[1].props.x;
    const moveToY = paths[1].props.y;

    expect(paths[2].type).toBe('lineTo');
    expect(paths[2].props.x).toBe(moveToX);
    expect(paths[2].props.y).toBeGreaterThan(moveToY);
  });
});
