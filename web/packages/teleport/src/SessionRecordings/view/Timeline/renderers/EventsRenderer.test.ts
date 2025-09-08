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
} from 'teleport/services/recordings';
import type { TimelineRenderContext } from 'teleport/SessionRecordings/view/Timeline/renderers/TimelineCanvasRenderer';

import {
  EVENT_ROW_HEIGHT,
  EVENT_SECTION_PADDING,
  LEFT_PADDING,
  RULER_HEIGHT,
} from '../constants';
import { EventsRenderer } from './EventsRenderer';

const mockMetadata: SessionRecordingMetadata = {
  startTime: 1609459200, // Jan 1, 2021
  endTime: 1609462800, // Jan 1, 2021
  duration: 3600000, // 1 hour in milliseconds
  user: 'testuser',
  resourceName: 'test-server',
  clusterName: 'test-cluster',
  events: [],
  startCols: 80,
  startRows: 24,
  type: 'ssh',
};

function createRenderer(metadata?: Partial<SessionRecordingMetadata>) {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  const renderer = new EventsRenderer(ctx, darkTheme, {
    ...mockMetadata,
    ...metadata,
  });

  return { ctx, renderer };
}

describe('EventsRenderer', () => {
  describe('event row creation', () => {
    it('should place non-overlapping events in the same row', () => {
      const { renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 1000,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 1500,
            endTime: 2500,
            user: 'bob',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 3000,
            endTime: 4000,
            user: 'charlie',
          },
        ],
      });

      expect(renderer.getHeight()).toBe(
        EVENT_ROW_HEIGHT + EVENT_SECTION_PADDING
      );
    });

    it('should create multiple rows for overlapping events', () => {
      const { renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 2000,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 1000,
            endTime: 3000,
            user: 'bob',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 1500,
            endTime: 3500,
            user: 'charlie',
          },
        ],
      });

      expect(renderer.getHeight()).toBe(
        3 * EVENT_ROW_HEIGHT + EVENT_SECTION_PADDING
      );
    });

    it('should filter out resize events', () => {
      const { renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Resize,
            startTime: 0,
            endTime: 1000,
            cols: 100,
            rows: 30,
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 500,
            endTime: 1500,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Resize,
            startTime: 2000,
            endTime: 3000,
            cols: 120,
            rows: 40,
          },
        ],
      });

      expect(renderer.getHeight()).toBe(
        EVENT_ROW_HEIGHT + EVENT_SECTION_PADDING
      );
    });
  });

  describe('position calculation', () => {
    it('should calculate correct positions for events', () => {
      const { renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 2500,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 5000,
            endTime: 7500,
            user: 'bob',
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const rowsWithPositions = renderer.getRowsWithPositions();

      expect(rowsWithPositions).toHaveLength(1);
      expect(rowsWithPositions[0]).toHaveLength(2);

      expect(rowsWithPositions[0][0].startPosition).toBe(LEFT_PADDING);
      expect(rowsWithPositions[0][0].endPosition).toBe(LEFT_PADDING + 250);

      expect(rowsWithPositions[0][1].startPosition).toBe(LEFT_PADDING + 500);
      expect(rowsWithPositions[0][1].endPosition).toBe(LEFT_PADDING + 750);
    });

    it('should measure the text of each event', () => {
      const { ctx, renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Inactivity,
            startTime: 0,
            endTime: 3000,
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 4000,
            endTime: 5000,
            user: 'verylongusername',
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const events = ctx.__getEvents();
      const measureTextEvents = events.filter(
        call => call.type === 'measureText'
      );

      expect(measureTextEvents[0].props.text).toBe('Inactivity for 3s');
      expect(measureTextEvents[1].props.text).toBe('verylongusername joined');
    });
  });

  describe('visibility', () => {
    it('should only render visible events', () => {
      const { ctx, renderer } = createRenderer({
        duration: 20000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 1000,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 10000,
            endTime: 11000,
            user: 'bob',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 19000,
            endTime: 20000,
            user: 'charlie',
          },
        ],
      });

      renderer.setTimelineWidth(2000);
      renderer.calculate();

      const renderContext: TimelineRenderContext = {
        containerWidth: 100,
        offset: 0,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const drawCalls = ctx.__getDrawCalls();
      const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

      expect(fillTextCalls).toHaveLength(1);
      expect(fillTextCalls[0].props.text).toBe('alice joined');
    });

    it('should include events partially visible in viewport', () => {
      const { ctx, renderer } = createRenderer({
        duration: 20000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 1000,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 10000,
            endTime: 11000,
            user: 'bob',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 19000,
            endTime: 20000,
            user: 'charlie',
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const renderContext: TimelineRenderContext = {
        containerWidth: 550,
        offset: 0,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const drawCalls = ctx.__getDrawCalls();
      const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

      expect(fillTextCalls).toHaveLength(2);
      expect(fillTextCalls[0].props.text).toBe('alice joined');
      expect(fillTextCalls[1].props.text).toBe('bob joined');
    });
  });

  describe('event rendering', () => {
    it('should render inactivity events with duration text', () => {
      const { ctx, renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Inactivity,
            startTime: 1000,
            endTime: 6000,
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const renderContext: TimelineRenderContext = {
        containerWidth: 1200,
        offset: 0,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const drawCalls = ctx.__getDrawCalls();
      const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

      expect(fillTextCalls).toHaveLength(1);
      expect(fillTextCalls[0].props.text).toBe('Inactivity for 5s');
    });

    it('should constrain text within event bounds', () => {
      const { ctx, renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 1000,
            endTime: 1100,
            user: 'verylongusernamethatoverflows',
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const renderContext: TimelineRenderContext = {
        containerWidth: 1200,
        offset: -100,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const drawCalls = ctx.__getDrawCalls();
      const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

      expect(fillTextCalls).toHaveLength(1);

      // text should be rendered within event bounds
      // event starts at 1000ms which is 10% of 1000px
      // so event starts at 100px + LEFT_PADDING
      // text should not start before that

      const textX = fillTextCalls[0].props.x;
      const eventStartX = LEFT_PADDING + (1000 / 10000) * 1000;

      expect(textX).toBeGreaterThanOrEqual(eventStartX);
    });

    it('should position events at correct Y coordinate for each row', () => {
      const { ctx, renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 2000,
            user: 'alice',
          },
          {
            type: SessionRecordingEventType.Join,
            startTime: 1000,
            endTime: 3000,
            user: 'bob',
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const renderContext: TimelineRenderContext = {
        containerWidth: 1200,
        offset: 0,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const events = ctx.__getEvents();
      const roundRectEvents = events.filter(e => e.type === 'roundRect');

      expect(roundRectEvents).toHaveLength(2);

      // first event should be in first row
      expect(roundRectEvents[0].props.y).toBe(
        EVENT_SECTION_PADDING + RULER_HEIGHT
      );
      // second event should be in second row
      expect(roundRectEvents[1].props.y).toBe(
        EVENT_SECTION_PADDING + EVENT_ROW_HEIGHT + RULER_HEIGHT
      );
    });
  });

  describe('edge cases', () => {
    it('should handle empty events array', () => {
      const { ctx, renderer } = createRenderer({
        duration: 10000,
        events: [],
      });

      expect(renderer.getHeight()).toBe(EVENT_SECTION_PADDING);

      const renderContext: TimelineRenderContext = {
        containerWidth: 1200,
        offset: 0,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const drawCalls = ctx.__getDrawCalls();

      expect(drawCalls).toHaveLength(0);
    });

    it('should handle very long event titles', () => {
      const { ctx, renderer } = createRenderer({
        duration: 10000,
        events: [
          {
            type: SessionRecordingEventType.Join,
            startTime: 0,
            endTime: 100,
            user: 'a'.repeat(100),
          },
        ],
      });

      renderer.setTimelineWidth(1000);
      renderer.calculate();

      const renderContext: TimelineRenderContext = {
        containerWidth: 1200,
        offset: 0,
        containerHeight: 200,
        eventsHeight: renderer.getHeight(),
      };

      renderer._render(renderContext);

      const events = ctx.__getEvents();

      const roundRectCalls = events.filter(call => call.type === 'roundRect');

      expect(roundRectCalls).toHaveLength(1);

      // the event should be the width of the text plus padding
      const textMetrics = ctx.measureText('a'.repeat(100) + ' joined');
      const eventWidth = textMetrics.width + 16; // 8px padding on each side

      expect(roundRectCalls[0].props.width).toBeLessThanOrEqual(eventWidth);
    });
  });
});
