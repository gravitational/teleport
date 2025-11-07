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
  SessionRecordingThumbnail,
} from 'teleport/services/recordings';

import { LEFT_PADDING } from '../constants';
import { TimelineRenderer } from './TimelineRenderer';

const mockMetadata: SessionRecordingMetadata = {
  startTime: new Date('2021-01-01T00:00:00Z').getTime(),
  endTime: new Date('2021-01-01T01:00:00Z').getTime(),
  duration: 20 * 1000, // 20 seconds
  user: 'testuser',
  resourceName: 'test-server',
  clusterName: 'test-cluster',
  events: [
    {
      type: SessionRecordingEventType.Join,
      startTime: 0,
      endTime: 1000,
      user: 'alice',
    },
    {
      type: SessionRecordingEventType.Resize,
      startTime: 2000,
      endTime: 2000,
      cols: 100,
      rows: 30,
    },
  ],
  startCols: 80,
  startRows: 24,
  type: 'ssh',
};

function createThumbnail(
  svg: string,
  cols: number,
  rows: number,
  startOffset: number,
  cursorX = 0,
  cursorY = 0
): SessionRecordingThumbnail {
  return {
    svg,
    startOffset,
    endOffset: startOffset + 1000,
    cols,
    rows,
    cursorX,
    cursorY,
    cursorVisible: true,
  };
}

const mockFrames: SessionRecordingThumbnail[] = [
  createThumbnail('<svg>frame1</svg>', 80, 24, 0),
  createThumbnail('<svg>frame2</svg>', 100, 30, 1000),
  createThumbnail('<svg>frame3</svg>', 120, 40, 2000),
  createThumbnail('<svg>frame4</svg>', 80, 24, 3000),
  createThumbnail('<svg>frame5</svg>', 80, 24, 4000),
  createThumbnail('<svg>frame6</svg>', 80, 24, 5000),
  createThumbnail('<svg>frame7</svg>', 80, 24, 6000),
  createThumbnail('<svg>frame8</svg>', 80, 24, 7000),
  createThumbnail('<svg>frame9</svg>', 80, 24, 8000),
  createThumbnail('<svg>frame10</svg>', 80, 24, 9000),
  createThumbnail('<svg>frame11</svg>', 80, 24, 10000),
  createThumbnail('<svg>frame12</svg>', 80, 24, 11000),
  createThumbnail('<svg>frame13</svg>', 80, 24, 12000),
  createThumbnail('<svg>frame14</svg>', 80, 24, 13000),
  createThumbnail('<svg>frame15</svg>', 80, 24, 14000),
  createThumbnail('<svg>frame16</svg>', 80, 24, 15000),
  createThumbnail('<svg>frame17</svg>', 80, 24, 16000),
  createThumbnail('<svg>frame18</svg>', 80, 24, 17000),
  createThumbnail('<svg>frame19</svg>', 80, 24, 18000),
  createThumbnail('<svg>frame20</svg>', 80, 24, 19000),
  createThumbnail('<svg>frame21</svg>', 80, 24, 20000),
];

function createRenderer(
  metadata: SessionRecordingMetadata = mockMetadata,
  frames: SessionRecordingThumbnail[] = mockFrames,
  containerWidth = 1000,
  containerHeight = 200
) {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  const renderer = new TimelineRenderer(
    ctx,
    metadata,
    frames,
    darkTheme,
    containerWidth,
    containerHeight
  );

  return {
    renderer,
    ctx,
  };
}

let animationFrameCallback: FrameRequestCallback | null = null;

beforeEach(() => {
  jest.clearAllMocks();

  // Mock requestAnimationFrame to capture the callback
  global.requestAnimationFrame = jest.fn(callback => {
    animationFrameCallback = callback;
    return 1;
  });

  global.cancelAnimationFrame = jest.fn();

  // Mock performance.now
  jest.spyOn(performance, 'now').mockReturnValue(0);
});

afterEach(() => {
  jest.restoreAllMocks();
  animationFrameCallback = null;
});

describe('constructor and initialization', () => {
  it('should initialize with correct default values', () => {
    const { renderer } = createRenderer();

    expect(renderer.getZoom()).toBe(0.5);
    expect(renderer.getOffset()).toBe(0);
    expect(renderer.getIsUserControlled()).toBe(false);
  });

  it('should calculate minimum zoom based on container width', () => {
    const { renderer } = createRenderer();

    // With duration 20000ms and 0.1 pixels per ms, base timeline width is 2000px
    // Container width is 1000px - 2*LEFT_PADDING
    // Min zoom should ensure timeline fits in container

    const baseTimelineWidth = 20000 * 0.1; // 2000px
    const availableWidth = 1000 - 2 * LEFT_PADDING;
    const expectedMinZoom = Math.max(
      availableWidth / baseTimelineWidth,
      0.00001
    );

    expect(renderer.getZoom()).toBeGreaterThanOrEqual(expectedMinZoom);
  });

  it('should start the render loop', () => {
    createRenderer();

    expect(global.requestAnimationFrame).toHaveBeenCalled();
  });
});

describe('pan accumulation', () => {
  it('should set user controlled flag when panning', () => {
    const { renderer } = createRenderer();

    expect(renderer.getIsUserControlled()).toBe(false);

    renderer.accumulatePan(10);

    expect(renderer.getIsUserControlled()).toBe(true);
  });

  it('should apply pan smoothly in render loop', () => {
    const { renderer } = createRenderer();

    renderer.accumulatePan(100);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    // Pan should be applied with easing
    const newOffset = renderer.getOffset();
    expect(newOffset).not.toBe(0);

    expect(Math.abs(newOffset)).toBeLessThan(100); // Due to easing factor
  });

  it('should respect pan bounds', () => {
    const { renderer } = createRenderer();

    // Try to pan beyond the right boundary
    renderer.accumulatePan(-20000);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    expect(renderer.getOffset()).toBe(0);

    // Try to pan beyond the left boundary
    renderer.accumulatePan(20000);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    const timelineWidth = 20000 * 0.1 * renderer.getZoom();
    const minOffset = Math.min(0, 1000 - timelineWidth - LEFT_PADDING * 2);

    expect(renderer.getOffset()).toBe(minOffset);
  });
});

describe('zoom accumulation', () => {
  it('should accumulate zoom delta', () => {
    const { renderer } = createRenderer();
    const initialZoom = renderer.getZoom();

    renderer.accumulateZoom(500, 0.1);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    expect(renderer.getZoom()).not.toBe(initialZoom);
  });

  it('should respect zoom bounds', () => {
    const { renderer } = createRenderer();

    // Try to zoom out beyond minimum
    renderer.accumulateZoom(500, -10);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    expect(renderer.getZoom()).toBeGreaterThanOrEqual(0.00001);

    // Try to zoom in beyond maximum
    renderer.accumulateZoom(500, 10);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    expect(renderer.getZoom()).toBeLessThanOrEqual(2.5);
  });

  it('should keep point under mouse fixed when zooming', () => {
    const { renderer } = createRenderer();
    const mouseX = 500;

    // Get initial time under mouse
    const initialTime = renderer.getTimeAtX(mouseX);

    // Zoom in
    renderer.accumulateZoom(mouseX, 0.5);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    // Time under mouse should remain approximately the same
    const newTime = renderer.getTimeAtX(mouseX);
    expect(Math.abs(newTime - initialTime)).toBeLessThan(1);
  });
});

describe('getTimeAtX', () => {
  it('should convert x coordinate to time correctly', () => {
    const { renderer } = createRenderer();

    // At left padding with no offset
    const timeAtLeftPadding = renderer.getTimeAtX(LEFT_PADDING);

    expect(timeAtLeftPadding).toBe(0);

    // At middle of timeline (assuming zoom 0.5, timeline width is 1000px)
    const timelineWidth = 20000 * 0.1 * 0.5; // 1000px
    const middleX = LEFT_PADDING + timelineWidth / 2;
    const timeAtMiddle = renderer.getTimeAtX(middleX);

    expect(timeAtMiddle).toBe(10000);

    // At end of timeline
    const endX = LEFT_PADDING + timelineWidth;
    const timeAtEnd = renderer.getTimeAtX(endX);

    expect(timeAtEnd).toBe(20000);
  });

  it('should handle offset correctly', () => {
    const { renderer } = createRenderer();

    renderer.setOffset(-100);

    const timeAtLeftPadding = renderer.getTimeAtX(LEFT_PADDING);

    // With offset -100, the timeline is shifted left by 100px
    // So LEFT_PADDING now points to 2 seconds
    expect(timeAtLeftPadding).toBe(2000);
  });

  it('should clamp time to 0 minimum', () => {
    const { renderer } = createRenderer();

    renderer.setOffset(100);

    const timeBeforeStart = renderer.getTimeAtX(0);

    expect(timeBeforeStart).toBe(0);
  });
});

describe('setWidth', () => {
  it('should recalculate minimum zoom when width changes', () => {
    const { renderer } = createRenderer();

    const initialZoom = renderer.getZoom();

    renderer.setWidth(2000);

    // With wider container, minimum zoom could be different
    expect(renderer.getZoom()).not.toBe(initialZoom);
  });

  it('should adjust zoom if below new minimum', () => {
    const { renderer } = createRenderer();

    // Set a very small zoom
    renderer.accumulateZoom(500, -10);

    if (animationFrameCallback) {
      animationFrameCallback(0);
    }

    const smallZoom = renderer.getZoom();

    renderer.setWidth(200); // change to be narrow

    // Zoom should be adjusted to at least fit the container
    expect(renderer.getZoom()).toBeGreaterThanOrEqual(smallZoom);
  });
});

describe('wheel accumulation cleanup', () => {
  it('should clean up small pan values', () => {
    const { renderer } = createRenderer();

    renderer.accumulatePan(0.001);

    // Run multiple render frames to apply easing
    for (let i = 0; i < 10; i++) {
      if (animationFrameCallback) {
        animationFrameCallback(0);
      }
    }

    // Small values should be cleaned up
    expect(renderer.getOffset()).toBeCloseTo(0, 2);
  });

  it('should clean up small zoom values', () => {
    const { renderer } = createRenderer();
    const initialZoom = renderer.getZoom();

    renderer.accumulateZoom(500, 0.0001);

    // Run multiple render frames to apply easing
    for (let i = 0; i < 10; i++) {
      if (animationFrameCallback) {
        animationFrameCallback(0);
      }
    }

    // Zoom should remain essentially unchanged
    expect(Math.abs(renderer.getZoom() - initialZoom)).toBeLessThan(0.01);
  });

  it('should handle large accumulated pan values', () => {
    const { renderer } = createRenderer();

    const initialOffset = renderer.getOffset();

    renderer.accumulatePan(50);

    // Simulate time passing
    jest.spyOn(performance, 'now').mockReturnValue(50);

    for (let i = 0; i < 50; i++) {
      if (animationFrameCallback) {
        animationFrameCallback(0);
      }
    }

    // Pan should be applied
    expect(initialOffset - renderer.getOffset()).toBeGreaterThan(0);
  });
});
