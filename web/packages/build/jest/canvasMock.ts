/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Minimal in-repo replacement for jest-canvas-mock. jsdom does not implement
// HTMLCanvasElement.getContext('2d'), and a handful of tests inspect the
// recorded canvas events to assert what was drawn (see SessionRecordings
// timeline renderers). This file mocks just enough of CanvasRenderingContext2D
// for those tests, exposing the same __getEvents/__getDrawCalls/__getPath
// helpers that jest-canvas-mock used to provide.

type Transform = [number, number, number, number, number, number];

type CanvasStyle = string | CanvasGradient | CanvasPattern;

type EventProps = { [key: string]: any };

export interface CanvasRenderingContext2DEvent {
  type: string;
  transform: Transform;
  props: EventProps;
}

const IDENTITY: Transform = [1, 0, 0, 1, 0, 0];

class MockTextMetrics implements TextMetrics {
  readonly width: number;
  readonly actualBoundingBoxLeft = 0;
  readonly actualBoundingBoxRight = 0;
  readonly fontBoundingBoxAscent = 0;
  readonly fontBoundingBoxDescent = 0;
  readonly actualBoundingBoxAscent = 0;
  readonly actualBoundingBoxDescent = 0;
  readonly emHeightAscent = 0;
  readonly emHeightDescent = 0;
  readonly hangingBaseline = 0;
  readonly alphabeticBaseline = 0;
  readonly ideographicBaseline = 0;

  constructor(text: string) {
    this.width = text.length;
  }
}

class MockCanvasGradient implements CanvasGradient {
  addColorStop(_offset: number, _color: string) {}
}

class MockCanvasPattern implements CanvasPattern {
  setTransform(_transform?: DOMMatrix2DInit) {}
}

function createEvent(
  type: string,
  props: EventProps
): CanvasRenderingContext2DEvent {
  return { type, transform: IDENTITY.slice() as Transform, props };
}

class MockCanvasRenderingContext2D {
  private _events: CanvasRenderingContext2DEvent[] = [];
  private _drawCalls: CanvasRenderingContext2DEvent[] = [];
  private _path: CanvasRenderingContext2DEvent[] = [
    createEvent('beginPath', {}),
  ];
  private _clip: CanvasRenderingContext2DEvent[] = [];

  fillStyle: CanvasStyle = '#000000';
  strokeStyle: CanvasStyle = '#000000';
  font = '10px sans-serif';
  textAlign: CanvasTextAlign = 'start';
  textBaseline: CanvasTextBaseline = 'alphabetic';
  direction: CanvasDirection = 'inherit';
  lineWidth = 1;
  lineCap: CanvasLineCap = 'butt';
  lineJoin: CanvasLineJoin = 'miter';
  miterLimit = 10;
  lineDashOffset = 0;
  globalAlpha = 1;
  globalCompositeOperation: GlobalCompositeOperation = 'source-over';
  shadowBlur = 0;
  shadowColor = 'rgba(0, 0, 0, 0)';
  shadowOffsetX = 0;
  shadowOffsetY = 0;
  imageSmoothingEnabled = true;
  imageSmoothingQuality: ImageSmoothingQuality = 'low';
  filter = 'none';
  fontKerning: CanvasFontKerning = 'auto';
  fontStretch: CanvasFontStretch = 'normal';
  fontVariantCaps: CanvasFontVariantCaps = 'normal';
  letterSpacing = '0px';
  wordSpacing = '0px';
  textRendering: CanvasTextRendering = 'auto';

  readonly canvas: HTMLCanvasElement;

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
  }

  private push(
    type: string,
    props: EventProps = {}
  ): CanvasRenderingContext2DEvent {
    const event = createEvent(type, props);
    this._events.push(event);
    return event;
  }

  private pushDraw(
    type: string,
    props: EventProps = {}
  ): CanvasRenderingContext2DEvent {
    const event = this.push(type, props);
    this._drawCalls.push(event);
    return event;
  }

  private pushPath(
    type: string,
    props: EventProps = {}
  ): CanvasRenderingContext2DEvent {
    const event = this.push(type, props);
    this._path.push(event);
    return event;
  }

  __getEvents(): CanvasRenderingContext2DEvent[] {
    return this._events.slice();
  }

  __clearEvents() {
    this._events = [];
  }

  __getDrawCalls(): CanvasRenderingContext2DEvent[] {
    return this._drawCalls.slice();
  }

  __clearDrawCalls() {
    this._drawCalls = [];
  }

  __getPath(): CanvasRenderingContext2DEvent[] {
    return this._path.slice();
  }

  __clearPath() {
    this._path = [createEvent('beginPath', {})];
  }

  __getClippingRegion(): CanvasRenderingContext2DEvent[] {
    return this._clip.slice();
  }

  save() {
    this.push('save');
  }

  restore() {
    this.push('restore');
  }

  scale(x: number, y: number) {
    this.push('scale', { x, y });
  }

  rotate(angle: number) {
    this.push('rotate', { angle });
  }

  translate(x: number, y: number) {
    this.push('translate', { x, y });
  }

  transform(a: number, b: number, c: number, d: number, e: number, f: number) {
    this.push('transform', { a, b, c, d, e, f });
  }

  setTransform(
    a?: number | DOMMatrix2DInit,
    b?: number,
    c?: number,
    d?: number,
    e?: number,
    f?: number
  ) {
    this.push('setTransform', { a, b, c, d, e, f });
  }

  resetTransform() {
    this.push('resetTransform');
  }

  getTransform(): DOMMatrix {
    return { a: 1, b: 0, c: 0, d: 1, e: 0, f: 0 } as DOMMatrix;
  }

  beginPath() {
    this.__clearPath();
    this._events.push(this._path[0]);
  }

  closePath() {
    this.pushPath('closePath');
  }

  moveTo(x: number, y: number) {
    this.pushPath('moveTo', { x, y });
  }

  lineTo(x: number, y: number) {
    this.pushPath('lineTo', { x, y });
  }

  bezierCurveTo(
    cp1x: number,
    cp1y: number,
    cp2x: number,
    cp2y: number,
    x: number,
    y: number
  ) {
    this.pushPath('bezierCurveTo', { cp1x, cp1y, cp2x, cp2y, x, y });
  }

  quadraticCurveTo(cpx: number, cpy: number, x: number, y: number) {
    this.pushPath('quadraticCurveTo', { cpx, cpy, x, y });
  }

  arc(
    x: number,
    y: number,
    radius: number,
    startAngle: number,
    endAngle: number,
    anticlockwise = false
  ) {
    this.pushPath('arc', {
      x,
      y,
      radius,
      startAngle,
      endAngle,
      anticlockwise,
    });
  }

  arcTo(x1: number, y1: number, x2: number, y2: number, radius: number) {
    this.pushPath('arcTo', { x1, y1, x2, y2, radius });
  }

  ellipse(
    x: number,
    y: number,
    radiusX: number,
    radiusY: number,
    rotation: number,
    startAngle: number,
    endAngle: number,
    anticlockwise = false
  ) {
    this.pushPath('ellipse', {
      x,
      y,
      radiusX,
      radiusY,
      rotation,
      startAngle,
      endAngle,
      anticlockwise,
    });
  }

  rect(x: number, y: number, width: number, height: number) {
    this.pushPath('rect', { x, y, width, height });
  }

  roundRect(
    x: number,
    y: number,
    width: number,
    height: number,
    radii?: number | DOMPointInit | (number | DOMPointInit)[]
  ) {
    this.pushPath('roundRect', { x, y, width, height, radii });
  }

  fill() {
    this.pushDraw('fill');
  }

  stroke() {
    this.pushDraw('stroke');
  }

  clip() {
    const event = this.push('clip');
    this._clip.push(event);
  }

  fillRect(x: number, y: number, width: number, height: number) {
    this.pushDraw('fillRect', { x, y, width, height });
  }

  strokeRect(x: number, y: number, width: number, height: number) {
    this.pushDraw('strokeRect', { x, y, width, height });
  }

  clearRect(x: number, y: number, width: number, height: number) {
    this.pushDraw('clearRect', { x, y, width, height });
  }

  fillText(text: string, x: number, y: number, maxWidth?: number) {
    this.pushDraw('fillText', { text, x, y, maxWidth });
  }

  strokeText(text: string, x: number, y: number, maxWidth?: number) {
    this.pushDraw('strokeText', { text, x, y, maxWidth });
  }

  drawImage(
    image: CanvasImageSource,
    sxOrDx: number,
    syOrDy: number,
    sWidthOrDWidth?: number,
    sHeightOrDHeight?: number,
    dx?: number,
    dy?: number,
    dWidth?: number,
    dHeight?: number
  ) {
    if (dx === undefined) {
      this.pushDraw('drawImage', {
        img: image,
        dx: sxOrDx,
        dy: syOrDy,
        dWidth: sWidthOrDWidth,
        dHeight: sHeightOrDHeight,
      });
      return;
    }
    this.pushDraw('drawImage', {
      img: image,
      sx: sxOrDx,
      sy: syOrDy,
      sWidth: sWidthOrDWidth,
      sHeight: sHeightOrDHeight,
      dx,
      dy,
      dWidth,
      dHeight,
    });
  }

  measureText(text: string): TextMetrics {
    const normalized = text == null ? '' : String(text);
    this.push('measureText', { text: normalized });
    return new MockTextMetrics(normalized);
  }

  createLinearGradient(
    x0: number,
    y0: number,
    x1: number,
    y1: number
  ): CanvasGradient {
    this.push('createLinearGradient', { x0, y0, x1, y1 });
    return new MockCanvasGradient();
  }

  createRadialGradient(
    x0: number,
    y0: number,
    r0: number,
    x1: number,
    y1: number,
    r1: number
  ): CanvasGradient {
    this.push('createRadialGradient', { x0, y0, r0, x1, y1, r1 });
    return new MockCanvasGradient();
  }

  createConicGradient(
    startAngle: number,
    x: number,
    y: number
  ): CanvasGradient {
    this.push('createConicGradient', { startAngle, x, y });
    return new MockCanvasGradient();
  }

  createPattern(
    _image: CanvasImageSource,
    _repetition: string | null
  ): CanvasPattern | null {
    return new MockCanvasPattern();
  }

  createImageData(
    widthOrImageData: number | ImageData,
    height?: number
  ): ImageData {
    const isImageData = typeof widthOrImageData !== 'number';
    const w = isImageData ? widthOrImageData.width : widthOrImageData;
    const h = isImageData ? widthOrImageData.height : (height ?? 0);
    return {
      data: new Uint8ClampedArray(w * h * 4),
      width: w,
      height: h,
      colorSpace: 'srgb',
    } as ImageData;
  }

  getImageData(_sx: number, _sy: number, sw: number, sh: number): ImageData {
    return {
      data: new Uint8ClampedArray(sw * sh * 4),
      width: sw,
      height: sh,
      colorSpace: 'srgb',
    } as ImageData;
  }

  putImageData() {}

  setLineDash(_segments: number[]) {}

  getLineDash(): number[] {
    return [];
  }

  isPointInPath(): boolean {
    return false;
  }

  isPointInStroke(): boolean {
    return false;
  }

  drawFocusIfNeeded() {}

  scrollPathIntoView() {}

  getContextAttributes(): CanvasRenderingContext2DSettings {
    return {};
  }

  isContextLost(): boolean {
    return false;
  }

  reset() {
    this.__clearEvents();
    this.__clearDrawCalls();
    this.__clearPath();
    this._clip = [];
  }
}

const contexts = new WeakMap<HTMLCanvasElement, MockCanvasRenderingContext2D>();

function installCanvasMock() {
  if (typeof window === 'undefined') return;
  const proto = window.HTMLCanvasElement?.prototype;
  if (!proto) return;

  // The DOM `getContext` has many context-specific overloads we can't
  // express in a single signature. Define it via Object.defineProperty so the
  // value slot doesn't have to satisfy every overload simultaneously.
  Object.defineProperty(proto, 'getContext', {
    configurable: true,
    writable: true,
    value: function getContext(
      this: HTMLCanvasElement,
      type: string
    ): MockCanvasRenderingContext2D | null {
      if (type !== '2d') return null;
      let ctx = contexts.get(this);
      if (!ctx) {
        ctx = new MockCanvasRenderingContext2D(this);
        contexts.set(this, ctx);
      }
      return ctx;
    },
  });

  proto.toDataURL = function toDataURL(type?: string): string {
    const mime =
      type === 'image/jpeg' || type === 'image/webp' ? type : 'image/png';
    return `data:${mime};base64,00`;
  };

  proto.toBlob = function toBlob(callback: BlobCallback, type?: string) {
    const mime =
      type === 'image/jpeg' || type === 'image/webp' ? type : 'image/png';
    const length = (this.width || 1) * (this.height || 1) * 4;
    const blob = new window.Blob([new Uint8Array(length)], { type: mime });
    setTimeout(() => callback(blob), 0);
  };
}

installCanvasMock();

declare global {
  interface CanvasRenderingContext2D {
    __getEvents(): CanvasRenderingContext2DEvent[];

    __clearEvents(): void;

    __getDrawCalls(): CanvasRenderingContext2DEvent[];

    __clearDrawCalls(): void;

    __getPath(): CanvasRenderingContext2DEvent[];

    __clearPath(): void;

    __getClippingRegion(): CanvasRenderingContext2DEvent[];
  }
}
