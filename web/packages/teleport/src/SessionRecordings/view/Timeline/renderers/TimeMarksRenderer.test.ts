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

import { SessionRecordingMetadata } from 'teleport/services/recordings';
import type { TimelineRenderContext } from 'teleport/SessionRecordings/view/Timeline/renderers/TimelineCanvasRenderer';

import { TimeMarkersRenderer } from './TimeMarksRenderer';

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

function createRenderer(metadata?: Partial<SessionRecordingMetadata>) {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  const renderer = new TimeMarkersRenderer(ctx, darkTheme, {
    ...mockMetadata,
    ...metadata,
  });

  return { ctx, renderer };
}

describe('time marker intervals', () => {
  it('should render 10 second intervals at low zoom', () => {
    const { ctx, renderer } = createRenderer({
      duration: 60 * 1000, // 1 minute
    });

    renderer.setTimelineWidth(500); // low zoom level

    const renderContext: TimelineRenderContext = {
      containerWidth: 600,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // At low zoom, should show 10 second intervals: 0:00, 0:10, 0:20, 0:30, 0:40, 0:50, 1:00
    expect(fillTextCalls).toHaveLength(7);
    expect(fillTextCalls[0].props.text).toBe('0:00');
    expect(fillTextCalls[1].props.text).toBe('0:10');
    expect(fillTextCalls[2].props.text).toBe('0:20');
    expect(fillTextCalls[3].props.text).toBe('0:30');
    expect(fillTextCalls[4].props.text).toBe('0:40');
    expect(fillTextCalls[5].props.text).toBe('0:50');
    expect(fillTextCalls[6].props.text).toBe('1:00');
  });

  it('should render 5 second intervals at medium zoom', () => {
    const { ctx, renderer } = createRenderer({
      duration: 60 * 1000, // 1 minute
    });

    renderer.setTimelineWidth(2000); // medium zoom level

    const renderContext: TimelineRenderContext = {
      containerWidth: 2100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // At medium zoom, should show 5 second intervals
    expect(fillTextCalls).toHaveLength(13);
    expect(fillTextCalls[0].props.text).toBe('0:00');
    expect(fillTextCalls[1].props.text).toBe('0:05');
    expect(fillTextCalls[2].props.text).toBe('0:10');
    expect(fillTextCalls[6].props.text).toBe('0:30');
    expect(fillTextCalls[12].props.text).toBe('1:00');
  });

  it('should render 1 second intervals at high zoom', () => {
    const { ctx, renderer } = createRenderer({
      duration: 10000, // 10 seconds
    });

    renderer.setTimelineWidth(2500); // high zoom level

    const renderContext: TimelineRenderContext = {
      containerWidth: 2600,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // At high zoom, should show 1-second intervals
    expect(fillTextCalls).toHaveLength(11);

    expect(fillTextCalls[0].props.text).toBe('0:00');
    expect(fillTextCalls[1].props.text).toBe('0:01');
    expect(fillTextCalls[2].props.text).toBe('0:02');
    expect(fillTextCalls[5].props.text).toBe('0:05');
    expect(fillTextCalls[10].props.text).toBe('0:10');
  });
});

describe('absolute time display', () => {
  it('should show absolute time for minute markers when enabled', () => {
    const { ctx, renderer } = createRenderer({
      startTime: 1609459200000, // Jan 1, 2021 12:00:00 AM UTC
      duration: 3 * 60 * 1000, // 3 minutes
    });

    renderer.setTimelineWidth(2000);
    renderer.setShowAbsoluteTime(true);

    const renderContext: TimelineRenderContext = {
      containerWidth: 2100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // Should show absolute time for minute markers (0:00, 1:00, 2:00, 3:00)
    // and relative time for 5 second intervals between them
    const minuteMarkers = fillTextCalls.filter(
      (call, index) => index % 12 === 0 // Every 12th marker is a minute marker (60s / 5s interval)
    );

    // The absolute time should be shown in 12-hour format
    // Starting at midnight UTC
    expect(minuteMarkers[0].props.text).toBe('12:00am');
    expect(minuteMarkers[1].props.text).toBe('12:01am');
    expect(minuteMarkers[2].props.text).toBe('12:02am');
    expect(minuteMarkers[3].props.text).toBe('12:03am');

    const nonMinuteMarkers = fillTextCalls.filter(
      (call, index) => index % 12 !== 0
    );

    // Non-minute markers should show relative time
    expect(nonMinuteMarkers[0].props.text).toBe('0:05');
    expect(nonMinuteMarkers[1].props.text).toBe('0:10');
    expect(nonMinuteMarkers[2].props.text).toBe('0:15');
    expect(nonMinuteMarkers[3].props.text).toBe('0:20');
  });

  it('should show relative time when absolute time is disabled', () => {
    const { ctx, renderer } = createRenderer({
      startTime: new Date('2021-01-01T00:00:00Z').getTime(),
      duration: 3 * 60 * 1000, // 3 minutes
    });

    renderer.setTimelineWidth(2000);
    renderer.setShowAbsoluteTime(false);

    const renderContext: TimelineRenderContext = {
      containerWidth: 2100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // All markers should show relative time
    expect(fillTextCalls[0].props.text).toBe('0:00');
    expect(fillTextCalls[12].props.text).toBe('1:00');
    expect(fillTextCalls[24].props.text).toBe('2:00');
    expect(fillTextCalls[36].props.text).toBe('3:00');
  });

  it('should handle PM times correctly', () => {
    const { ctx, renderer } = createRenderer({
      startTime: new Date('2021-01-01T12:00:00Z').getTime(),
      duration: 2 * 60 * 1000, // 2 minutes
    });

    renderer.setTimelineWidth(2000);
    renderer.setShowAbsoluteTime(true);

    const renderContext: TimelineRenderContext = {
      containerWidth: 2100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    const minuteMarkers = fillTextCalls.filter(
      (call, index) => index % 12 === 0
    );

    expect(minuteMarkers[0].props.text).toBe('12:00pm');
    expect(minuteMarkers[1].props.text).toBe('12:01pm');
    expect(minuteMarkers[2].props.text).toBe('12:02pm');
  });
});

describe('visibility', () => {
  it('should only render visible time markers', () => {
    const { ctx, renderer } = createRenderer({
      duration: 60 * 1000, // 1 minute
    });

    renderer.setTimelineWidth(6000); // very high zoom

    const renderContext: TimelineRenderContext = {
      containerWidth: 500,
      offset: -1000, // scrolled to show markers around 10 seconds
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();
    const fillTextCalls = drawCalls.filter(call => call.type === 'fillText');

    // Should only render markers visible in the viewport (with 100px buffer)
    // At this zoom and offset, should see markers starting from 9 seconds
    expect(fillTextCalls.length).toBe(8); // 9s, 10s, 11s, 12s, 13s, 14s, 15s, 16s

    // Check that we're seeing markers in the expected range
    const visibleTexts = fillTextCalls.map(call => call.props.text);

    expect(visibleTexts[0]).toBe('0:09');
    expect(visibleTexts[visibleTexts.length - 1]).toBe('0:16');
  });
});

describe('sub-tick rendering', () => {
  it('should render sub-ticks between main markers at high zoom', () => {
    const { ctx, renderer } = createRenderer({
      duration: 10 * 1000, // 10 seconds
    });

    renderer.setTimelineWidth(3000); // high zoom to get many sub-ticks

    const renderContext: TimelineRenderContext = {
      containerWidth: 3100,
      offset: 0,
      containerHeight: 200,
      eventsHeight: 100,
    };

    renderer._render(renderContext);

    const drawCalls = ctx.__getDrawCalls();

    // Count fillRect calls that are sub-ticks (height 4 or 6)
    const subTickCalls = drawCalls.filter(
      call =>
        call.type === 'fillRect' &&
        (call.props.height === 4 || call.props.height === 6)
    );

    // At high zoom, should have multiple sub-ticks between main markers
    expect(subTickCalls.length).toBeGreaterThan(0);
  });
});
