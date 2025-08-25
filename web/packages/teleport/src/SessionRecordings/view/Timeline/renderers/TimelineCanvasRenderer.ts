import type { DefaultTheme } from 'styled-components';

export interface TimelineRenderContext {
  containerHeight: number;
  containerWidth: number;
  eventsHeight: number;
  offset: number;
}

export abstract class TimelineCanvasRenderer {
  protected timelineWidth = 0;

  constructor(
    protected ctx: CanvasRenderingContext2D,
    protected theme: DefaultTheme,
    protected duration: number
  ) {}

  abstract _render(context: TimelineRenderContext): void;

  abstract calculate(): void;

  render(context: TimelineRenderContext) {
    this.ctx.save();

    this._render(context);

    this.ctx.restore();
  }

  setTimelineWidth(timelineWidth: number) {
    this.timelineWidth = timelineWidth;

    this.calculate();
  }
}
