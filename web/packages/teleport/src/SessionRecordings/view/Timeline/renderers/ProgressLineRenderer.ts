import { LEFT_PADDING } from '../constants';
import {
  TimelineCanvasRenderer,
  type TimelineRenderContext,
} from './TimelineCanvasRenderer';

export class ProgressLineRenderer extends TimelineCanvasRenderer {
  private currentTime = 0;
  private position = 0;

  _render({ containerHeight }: TimelineRenderContext) {
    this.ctx.strokeStyle =
      this.theme.colors.sessionRecordingTimeline.progressLine;
    this.ctx.lineWidth = 2;

    this.ctx.beginPath();
    this.ctx.moveTo(this.position, 0);
    this.ctx.lineTo(this.position, containerHeight);
    this.ctx.stroke();
  }

  calculate() {
    this.position =
      (this.currentTime / this.duration) * this.timelineWidth + LEFT_PADDING;
  }

  setCurrentTime(currentTime: number) {
    this.currentTime = currentTime;
    this.calculate();
  }
}
