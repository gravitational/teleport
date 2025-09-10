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

import type {
  SessionRecordingMetadata,
  SessionRecordingThumbnail,
} from 'teleport/services/recordings';

import { LEFT_PADDING } from '../constants';
import { EventsRenderer } from './EventsRenderer';
import { FramesRenderer } from './FramesRenderer';
import { ProgressLineRenderer } from './ProgressLineRenderer';
import { ResizeEventsRenderer } from './ResizeEventsRenderer';
import type {
  TimelineCanvasRenderer,
  TimelineRenderContext,
} from './TimelineCanvasRenderer';
import { TimeMarkersRenderer } from './TimeMarksRenderer';

interface WheelAccumulation {
  deltaX: number;
  zoomDelta: number;
  zoomX: number;
  isActive: boolean;
  lastEventTime: number;
}

/**
 * TimelineRenderer is responsible for rendering the entire timeline,
 * including frames, events, time markers, and the progress line.
 *
 * It manages the state of the timeline, including zoom level and offset,
 * and handles user interactions such as panning and zooming.
 *
 * It uses a render loop to efficiently update the canvas only when needed
 * (instead of continuously), and accumulates wheel events for smooth panning
 * and zooming (allowing for 60fps+ interactions).
 */
export class TimelineRenderer {
  private animationFrameId: null | number = null;
  private needsRender = false;

  // whether or not the user is controlling the timeline (panning/zooming), so we can avoid
  // moving the timeline automatically as the session is playing
  private isUserControlled = false;
  private offset = 0;

  private minZoom = 0.1;
  private maxZoom = 2.5;
  private zoom = 0.5;

  private readonly backgroundColor: string;

  private readonly eventsRenderer: EventsRenderer;
  private readonly framesRenderer: FramesRenderer;
  private readonly progressLineRenderer: ProgressLineRenderer;
  private readonly resizeEventsRenderer: ResizeEventsRenderer;
  private readonly timeMarkersRenderer: TimeMarkersRenderer;
  private readonly renderers: TimelineCanvasRenderer[];

  // Enhanced wheel accumulation matching GraphRenderer
  private wheelAccumulation: WheelAccumulation = {
    deltaX: 0,
    zoomDelta: 0,
    zoomX: 0,
    isActive: false,
    lastEventTime: 0,
  };

  constructor(
    private ctx: CanvasRenderingContext2D,
    private metadata: SessionRecordingMetadata,
    private frames: SessionRecordingThumbnail[],
    private theme: DefaultTheme,
    private containerWidth: number,
    private containerHeight: number
  ) {
    this.backgroundColor =
      this.theme.colors.sessionRecordingTimeline.background;

    this.progressLineRenderer = new ProgressLineRenderer(
      this.ctx,
      this.theme,
      this.metadata.duration
    );

    this.eventsRenderer = new EventsRenderer(
      this.ctx,
      this.theme,
      this.metadata
    );

    this.framesRenderer = new FramesRenderer(
      this.ctx,
      this.theme,
      this.metadata.duration,
      this.frames,
      this.containerHeight,
      this.eventsRenderer.getHeight()
    );

    this.timeMarkersRenderer = new TimeMarkersRenderer(
      this.ctx,
      this.theme,
      this.metadata
    );

    this.resizeEventsRenderer = new ResizeEventsRenderer(
      this.ctx,
      this.theme,
      this.metadata
    );

    this.renderers = [
      this.framesRenderer,
      this.timeMarkersRenderer,
      this.eventsRenderer,
      this.resizeEventsRenderer,
      this.progressLineRenderer,
    ];

    this.calculateMinZoom();

    if (this.zoom < this.minZoom) {
      this.zoom = this.minZoom;
    }

    this.setRenderersTimelineWidth();

    // load the visible frames first, then render, then load the rest of the frames
    // when the browser is idle
    this.framesRenderer
      .loadVisibleFrames(this.offset, this.containerWidth)
      .then(() => {
        this.render();

        requestIdleCallback(() => {
          void this.framesRenderer.loadNonVisibleFrames();
        });
      })
      .catch((error: unknown) => {
        if (error instanceof Error) {
          // eslint-disable-next-line no-console
          console.error('Failed to load frames:', error.message);

          return;
        }

        // eslint-disable-next-line no-console
        console.error('Failed to load frames:', error);
      });

    this.startRenderLoop();
  }

  destroy() {
    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }
  }

  // accumulatePan accumulates horizontal panning delta from wheel events.
  // Positive delta pans right, negative delta pans left.
  // The actual panning is smoothed and applied in the render loop.
  accumulatePan(delta: number) {
    this.wheelAccumulation.deltaX += delta;
    this.wheelAccumulation.isActive = true;
    this.wheelAccumulation.lastEventTime = performance.now();
    this.needsRender = true;

    if (!this.isUserControlled) {
      this.isUserControlled = true;
    }
  }

  // accumulateZoom accumulates zoom delta from wheel events.
  // Positive delta zooms in, negative delta zooms out.
  // The actual zooming is smoothed and applied in the render loop.
  accumulateZoom(x: number, delta: number) {
    this.wheelAccumulation.zoomDelta += delta;
    this.wheelAccumulation.zoomX = x;
    this.wheelAccumulation.isActive = true;
    this.wheelAccumulation.lastEventTime = performance.now();
    this.needsRender = true;
  }

  getTimeAtX(x: number) {
    const absoluteX = x - LEFT_PADDING - this.offset;
    const timelineWidth = this.calculateTimelineWidth();
    const timeRatio = absoluteX / timelineWidth;

    // round the zoom time to avoid floating point precision issues
    // causing the timeline to move around the mouse when zooming in
    return Math.max(
      0,
      Math.round(timeRatio * this.metadata.duration * 10) / 10
    );
  }

  getIsUserControlled() {
    return this.isUserControlled;
  }

  getOffset() {
    return this.offset;
  }

  getZoom() {
    return this.zoom;
  }

  // Force recreate all images, useful when the height of the timeline changes
  recreateImages() {
    this.framesRenderer.recreateImages(this.render.bind(this));
  }

  // Recreate only the images that are currently visible in the viewport
  recreateVisibleImages() {
    this.framesRenderer.recreateVisibleImages(
      this.offset,
      this.containerWidth,
      this.render.bind(this)
    );
  }

  render() {
    this.needsRender = true;
  }

  setCurrentTime(currentTime: number) {
    this.progressLineRenderer.setCurrentTime(currentTime);

    this.render();
  }

  setShowAbsoluteTime(showAbsoluteTime: boolean) {
    this.timeMarkersRenderer.setShowAbsoluteTime(showAbsoluteTime);

    this.render();
  }

  setIsUserControlled(isUserControlled: boolean) {
    this.isUserControlled = isUserControlled;
  }

  setHeight(height: number) {
    // When changing the height, we want to keep the center of the timeline
    // at the same position in time, so we need to adjust the offset accordingly
    // This is done by calculating the time ratio of the center position before
    // the height change, then applying that ratio to the new timeline width
    // to get the new center position, and finally adjusting the offset to keep
    // the center position consistent.

    const centerX = this.containerWidth / 2;
    const oldTimelineWidth = this.calculateTimelineWidth();
    const absoluteCenterPosition = centerX - this.offset - LEFT_PADDING;
    const timeRatio = absoluteCenterPosition / oldTimelineWidth;

    this.containerHeight = height;

    this.framesRenderer.setHeight(height, this.eventsRenderer.getHeight());

    this.setRenderersTimelineWidth();

    const newTimelineWidth = this.calculateTimelineWidth();
    const newAbsoluteCenterPosition = timeRatio * newTimelineWidth;
    const newOffset = centerX - newAbsoluteCenterPosition - LEFT_PADDING;

    const maxOffset = 0;
    const minOffset = Math.min(
      0,
      this.containerWidth - newTimelineWidth - LEFT_PADDING * 2
    );

    this.offset = Math.max(minOffset, Math.min(maxOffset, newOffset));

    this.calculateMinZoom();
    if (this.zoom < this.minZoom) {
      this.zoom = this.minZoom;
    }

    this.render();
  }

  setOffset(offset: number) {
    this.offset = offset;

    this.render();
  }

  setWidth(width: number) {
    this.containerWidth = width;

    this.calculateMinZoom();

    if (this.zoom < this.minZoom) {
      this.zoom = this.minZoom;
    }

    this.setRenderersTimelineWidth();

    this.render();
  }

  private calculateMinZoom() {
    const baseTimelineWidth = this.getBaseTimelineWidth();

    const availableWidth = this.containerWidth - 2 * LEFT_PADDING;

    this.minZoom = 1;

    if (baseTimelineWidth > 0) {
      const calculatedMinZoom = availableWidth / baseTimelineWidth;

      this.minZoom = Math.max(calculatedMinZoom, 0.00001);
    }
  }

  private getBaseTimelineWidth() {
    const pixelsPerMs = 0.1;

    return this.metadata.duration * pixelsPerMs;
  }

  private calculateTimelineWidth() {
    return this.getBaseTimelineWidth() * this.zoom;
  }

  private setRenderersTimelineWidth() {
    const timelineWidth = this.calculateTimelineWidth();

    for (const renderer of this.renderers) {
      renderer.setTimelineWidth(timelineWidth);
    }
  }

  private _render() {
    this.ctx.fillStyle = this.backgroundColor;
    this.ctx.fillRect(0, 0, this.containerWidth, this.containerHeight);

    this.ctx.save();
    this.ctx.translate(this.offset, 0);

    const renderContext: TimelineRenderContext = {
      containerWidth: this.containerWidth,
      containerHeight: this.containerHeight,
      eventsHeight: this.eventsRenderer.getHeight(),
      offset: this.offset,
    };

    for (const renderer of this.renderers) {
      renderer.render(renderContext);
    }

    this.ctx.restore();
  }

  private startRenderLoop() {
    const loop = () => {
      if (this.wheelAccumulation.isActive) {
        // Handle wheel event accumulation for smooth panning and zooming
        this.applyWheelAccumulation();
      }

      if (this.needsRender) {
        this._render();
        this.needsRender = false;
      }

      this.animationFrameId = requestAnimationFrame(loop);
    };

    loop();
  }

  // Apply accumulated wheel events for smooth panning and zooming
  private applyWheelAccumulation() {
    const timeSinceLastEvent =
      performance.now() - this.wheelAccumulation.lastEventTime;

    // Apply accumulated pan
    if (this.wheelAccumulation.deltaX !== 0) {
      // Smooth out the pan with an easing factor
      const easingFactor = 0.4;
      const appliedDeltaX = this.wheelAccumulation.deltaX * easingFactor;

      // Calculate new offset with bounds checking
      const filmstripWidth = this.calculateTimelineWidth() + LEFT_PADDING * 2;

      const maxOffset = 0;
      const minOffset = Math.min(0, this.containerWidth - filmstripWidth);
      const newOffset = this.offset - appliedDeltaX;

      this.offset = Math.max(minOffset, Math.min(maxOffset, newOffset));

      // Reduce accumulated value
      this.wheelAccumulation.deltaX -= appliedDeltaX;

      // Clean up small values
      if (Math.abs(this.wheelAccumulation.deltaX) < 0.01) {
        this.wheelAccumulation.deltaX = 0;
      }

      this.needsRender = true;
    }

    // Apply accumulated zoom
    if (this.wheelAccumulation.zoomDelta !== 0) {
      const easingFactor = 0.4;
      const appliedZoomDelta = this.wheelAccumulation.zoomDelta * easingFactor;

      const oldZoom = this.zoom;
      const newZoom = Math.max(
        this.minZoom,
        Math.min(this.maxZoom, this.zoom * Math.exp(appliedZoomDelta))
      );

      if (newZoom !== oldZoom) {
        // Calculate new offset to keep the point under the mouse cursor fixed
        // when zooming in or out
        const newTimelineWidth = this.getBaseTimelineWidth() * newZoom;
        const newFilmstripWidth = newTimelineWidth + LEFT_PADDING * 2;

        if (newFilmstripWidth <= this.containerWidth) {
          // Timeline fits in container, center it
          this.offset = 0;
          this.zoom = newZoom;
        } else {
          // Keep the point under the mouse cursor fixed

          const currentTimelineWidth = this.calculateTimelineWidth();
          const adjustedMouseX = this.wheelAccumulation.zoomX - LEFT_PADDING;
          const absoluteMousePosition = adjustedMouseX - this.offset;
          const timeRatio = absoluteMousePosition / currentTimelineWidth;
          const timeUnderMouse = timeRatio * this.metadata.duration;

          const newAbsolutePosition =
            (timeUnderMouse / this.metadata.duration) * newTimelineWidth;

          const maxOffset = 0;
          const minOffset = this.containerWidth - newFilmstripWidth;
          const newOffset = adjustedMouseX - newAbsolutePosition;

          this.offset = Math.max(minOffset, Math.min(maxOffset, newOffset));
          this.zoom = newZoom;
        }

        this.setRenderersTimelineWidth();
      }

      // Reduce accumulated zoom
      this.wheelAccumulation.zoomDelta -= appliedZoomDelta;

      if (Math.abs(this.wheelAccumulation.zoomDelta) < 0.001) {
        this.wheelAccumulation.zoomDelta = 0;
      }

      this.needsRender = true;
    }

    // Check if we should stop the wheel animation
    if (
      timeSinceLastEvent > 100 &&
      Math.abs(this.wheelAccumulation.deltaX) < 0.01 &&
      Math.abs(this.wheelAccumulation.zoomDelta) < 0.001
    ) {
      this.wheelAccumulation.isActive = false;
      this.wheelAccumulation.deltaX = 0;
      this.wheelAccumulation.zoomDelta = 0;
    }
  }
}
