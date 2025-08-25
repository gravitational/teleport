import type { DefaultTheme } from 'styled-components';

import type { SessionRecordingThumbnail } from 'teleport/services/recordings';
import {
  generateTerminalSVGStyleTag,
  injectSVGStyles,
  svgToDataURIBase64,
} from 'teleport/SessionRecordings/svg';
import { calculateThumbnailViewport } from 'teleport/SessionRecordings/view/Timeline/utils';

import {
  BASE_FRAME_WIDTH,
  DEFAULT_FRAME_HEIGHT,
  DEFAULT_MAX_FRAME_WIDTH,
  EVENT_SECTION_PADDING,
  LEFT_PADDING,
  RULER_HEIGHT,
} from '../constants';
import {
  TimelineCanvasRenderer,
  type TimelineRenderContext,
} from './TimelineCanvasRenderer';

const FRAME_RATIO = 138 / 240;

interface ThumbnailWithId extends SessionRecordingThumbnail {
  id: string;
}

interface ThumbnailWithPosition extends ThumbnailWithId {
  index: number;
  position: number;
  width: number;
  isEnd: boolean;
}

export class FramesRenderer extends TimelineCanvasRenderer {
  private readonly frames: ThumbnailWithId[] = [];
  private framesAtCurrentZoom: ThumbnailWithPosition[] = [];

  private frameHeight = FRAME_RATIO * BASE_FRAME_WIDTH;
  private maxFrameWidth = 0;

  private loadedImageElements = new Map<string, HTMLImageElement>();
  private loadedImages = new Map<string, OffscreenCanvas>();

  constructor(
    ctx: CanvasRenderingContext2D,
    theme: DefaultTheme,
    duration: number,
    frames: SessionRecordingThumbnail[],
    initialHeight: number,
    eventsHeight: number
  ) {
    super(ctx, theme, duration);

    const svgTheme = generateTerminalSVGStyleTag(theme);

    this.frameHeight =
      initialHeight - eventsHeight - RULER_HEIGHT - EVENT_SECTION_PADDING * 2;

    this.maxFrameWidth =
      (DEFAULT_MAX_FRAME_WIDTH / DEFAULT_FRAME_HEIGHT) * initialHeight;

    this.frames = frames.map((frame, index) => ({
      ...frame,
      id: `frame-${index}`,
      svg: svgToDataURIBase64(injectSVGStyles(frame.svg, svgTheme)),
    }));

    this.setHeight(initialHeight, eventsHeight);
  }

  _render({ containerWidth, eventsHeight, offset }: TimelineRenderContext) {
    const framesToRender = this.getVisibleFrames(offset, containerWidth);

    for (let i = 0; i < framesToRender.length; i++) {
      const frame = framesToRender[i];
      const img = this.loadedImages.get(frame.id);

      if (img) {
        this.ctx.drawImage(
          img,
          frame.position,
          eventsHeight + EVENT_SECTION_PADDING + RULER_HEIGHT,
          frame.width,
          this.frameHeight
        );
      }
    }
  }

  calculate() {
    const framesWithPositions: ThumbnailWithPosition[] = [];

    for (let i = 0; i < this.frames.length; i++) {
      const frame = this.frames[i];
      const frameAspectRatio = frame.cols / frame.rows;

      const calculatedWidth = Math.ceil(this.frameHeight * frameAspectRatio);
      const frameWidth = Math.min(calculatedWidth, this.maxFrameWidth);

      const position =
        (frame.startOffset / this.duration) * this.timelineWidth + LEFT_PADDING;

      framesWithPositions.push({
        ...frame,
        index: i,
        isEnd: i === this.frames.length - 1,
        position,
        width: frameWidth,
      });
    }

    const framesAtZoom: ThumbnailWithPosition[] = [];

    for (const frame of framesWithPositions) {
      if (framesAtZoom.length === 0) {
        framesAtZoom.push(frame);

        continue;
      }

      const lastFrame = framesAtZoom[framesAtZoom.length - 1];
      const lastFrameEnd = lastFrame.position + lastFrame.width;

      if (frame.isEnd || frame.position >= lastFrameEnd - 1) {
        framesAtZoom.push(frame);
      }
    }

    this.framesAtCurrentZoom = framesAtZoom;
  }

  destroy() {
    this.framesAtCurrentZoom = [];
    this.loadedImageElements.clear();
  }

  loadNonVisibleFrames(): Promise<OffscreenCanvas[]> {
    const nonVisibleFrames = this.frames.filter(
      frame => !this.loadedImages.has(frame.id)
    );

    return Promise.all(nonVisibleFrames.map(frame => this.loadImage(frame)));
  }

  loadVisibleFrames(
    offset: number,
    containerWidth: number
  ): Promise<OffscreenCanvas[]> {
    const visibleFrames = this.getVisibleFrames(offset, containerWidth);

    return Promise.all(visibleFrames.map(frame => this.loadImage(frame)));
  }

  recreateImages(render: () => void) {
    for (const frame of this.frames) {
      const img = this.loadedImageElements.get(frame.id);
      const existingCanvas = this.loadedImages.get(frame.id);

      if (img && existingCanvas) {
        const newCanvas = new OffscreenCanvas(img.width, img.height);

        this.drawFrame(frame, newCanvas, img);

        render();
      }
    }
  }

  recreateVisibleImages(
    offset: number,
    containerWidth: number,
    render: () => void
  ) {
    const visibleFrames = this.getVisibleFrames(offset, containerWidth);

    for (const frame of visibleFrames) {
      const img = this.loadedImageElements.get(frame.id);
      const canvas = this.loadedImages.get(frame.id);

      if (img && canvas) {
        this.drawFrame(frame, canvas, img);

        render();
      }
    }
  }

  setHeight(height: number, eventsHeight: number) {
    this.frameHeight =
      height - eventsHeight - RULER_HEIGHT - EVENT_SECTION_PADDING * 2;

    this.maxFrameWidth =
      (DEFAULT_MAX_FRAME_WIDTH / DEFAULT_FRAME_HEIGHT) * height;
  }

  private binarySearchFrameIndex(position: number): number {
    let left = 0;
    let right = this.framesAtCurrentZoom.length - 1;
    let result = 0;

    while (left <= right) {
      const mid = Math.floor((left + right) / 2);
      const frame = this.framesAtCurrentZoom[mid];

      if (frame.position <= position) {
        result = mid;
        left = mid + 1;
      } else {
        right = mid - 1;
      }
    }

    return Math.max(0, result - 1);
  }

  private getVisibleFrames(
    offset: number,
    containerWidth: number
  ): ThumbnailWithPosition[] {
    const visibleStart = -offset - 100;
    const visibleEnd = -offset + containerWidth + 100;

    const frames: ThumbnailWithPosition[] = [];
    const startIndex = this.binarySearchFrameIndex(visibleStart);

    for (let i = startIndex; i < this.framesAtCurrentZoom.length; i++) {
      const frame = this.framesAtCurrentZoom[i];

      if (frame.isEnd) {
        if (frame.position < visibleEnd + frame.width) {
          const frameBefore = frames.find(
            f => f.position + f.width > visibleEnd - 100 - frame.width
          );

          if (!frameBefore) {
            frames.push({
              ...frame,
              position: visibleEnd - 100 - frame.width,
            });
          }

          continue;
        }

        continue;
      }

      if (frame.position > visibleEnd) {
        break;
      }

      const frameEnd = frame.position + frame.width;

      if (frameEnd >= visibleStart) {
        frames.push(frame);
      }
    }

    return frames;
  }

  private drawFrame(
    frame: ThumbnailWithId,
    canvas: OffscreenCanvas,
    image: HTMLImageElement
  ) {
    const frameAspectRatio = frame.cols / frame.rows;
    const calculatedWidth = Math.ceil(this.frameHeight * frameAspectRatio);
    const width = Math.min(calculatedWidth, this.maxFrameWidth);
    const height = this.frameHeight;

    const dpr = window.devicePixelRatio || 1;

    canvas.width = width * dpr;
    canvas.height = height * dpr;

    const ctx = canvas.getContext('2d');

    if (!ctx) {
      throw new Error('Failed to get offscreen canvas context');
    }

    ctx.scale(dpr, dpr);

    ctx.save();

    // Calculate viewport position
    const viewport = calculateThumbnailViewport(
      frame,
      image.width,
      image.height
    );

    // Calculate source dimensions maintaining aspect ratio
    const imageAspect = image.width / image.height;
    const canvasAspect = width / height;

    let adjustedSourceWidth = viewport.sourceWidth;
    let adjustedSourceHeight = viewport.sourceHeight;

    // Adjust source dimensions to match canvas aspect ratio
    if (imageAspect > canvasAspect) {
      // Image is wider - adjust width
      const adjustedFullWidth = image.height * canvasAspect;
      adjustedSourceWidth = adjustedFullWidth / viewport.zoomLevel;
    } else {
      // Image is taller - adjust height
      const adjustedFullHeight = image.width / canvasAspect;
      adjustedSourceHeight = adjustedFullHeight / viewport.zoomLevel;
    }

    const borderRadius = 12;

    // Create clipping path for rounded corners
    ctx.beginPath();
    ctx.roundRect(0, 0, width, height, borderRadius);
    ctx.clip();

    // Draw the image
    ctx.drawImage(
      image,
      viewport.sourceX,
      viewport.sourceY,
      adjustedSourceWidth,
      adjustedSourceHeight,
      0,
      0,
      width,
      height
    );

    ctx.restore();

    // Draw border AFTER restore to avoid clipping
    ctx.save();
    ctx.strokeStyle = this.theme.colors.sessionRecordingTimeline.frameBorder;
    ctx.lineWidth = 1;

    ctx.beginPath();
    // Adjust border position by 0.5px to ensure it's fully visible
    ctx.roundRect(0.5, 0.5, width - 1, height - 1, borderRadius);
    ctx.stroke();

    ctx.restore();

    this.loadedImages.set(frame.id, canvas);
  }

  private loadImage(frame: ThumbnailWithId): Promise<OffscreenCanvas> {
    return new Promise((resolve, reject) => {
      const img = new Image();

      img.onload = () => {
        try {
          const offscreen = new OffscreenCanvas(img.width, img.height);

          this.drawFrame(frame, offscreen, img);
          this.loadedImageElements.set(frame.id, img);

          resolve(offscreen);
        } catch {
          reject(new Error(`Failed to process image for frame ${frame.id}`));
        }
      };

      img.onerror = () => {
        reject(new Error(`Failed to load image for frame ${frame.id}`));
      };

      img.src = frame.svg;
    });
  }
}
