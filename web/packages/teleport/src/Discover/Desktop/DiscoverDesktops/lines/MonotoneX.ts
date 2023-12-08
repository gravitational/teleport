/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Path } from './Path';

function sign(x: number) {
  return x < 0 ? -1 : 1;
}

function slope2(line: MonotoneX, t: number) {
  const h = line.x1 - line.x0;

  return h ? ((3 * (line.y1 - line.y0)) / h - t) / 2 : t;
}

function point(line: MonotoneX, t0: number, t1: number) {
  const x0 = line.x0;
  const y0 = line.y0;
  const x1 = line.x1;
  const y1 = line.y1;
  const dx = (x1 - x0) / 3;

  line.path.bezierCurveTo(x0 + dx, y0 + dx * t0, x1 - dx, y1 - dx * t1, x1, y1);
}

function slope3(line: MonotoneX, x2: number, y2: number) {
  const h0 = line.x1 - line.x0;
  const h1 = x2 - line.x1;
  const s0 = (line.y1 - line.y0) / (h0 || (h1 < 0 && -0));
  const s1 = (y2 - line.y1) / (h1 || (h0 < 0 && -0));
  const p = (s0 * h1 + s1 * h0) / (h0 + h1);

  return (
    (sign(s0) + sign(s1)) *
      Math.min(Math.abs(s0), Math.abs(s1), 0.5 * Math.abs(p)) || 0
  );
}

export class MonotoneX {
  line: number;
  x0: number;
  x1: number;
  y0: number;
  y1: number;
  t0: number;
  p: number;

  constructor(public path: Path) {}

  areaStart() {
    this.line = 0;
  }

  areaEnd() {
    this.line = NaN;
  }

  lineStart() {
    this.x0 = this.x1 = this.y0 = this.y1 = this.t0 = NaN;
    this.p = 0;
  }

  lineEnd() {
    switch (this.p) {
      case 2:
        this.path.moveTo(this.x1, this.y1);
        break;
      case 3:
        point(this, this.t0, slope2(this, this.t0));
        break;
    }

    if (this.line || (this.line !== 0 && this.p === 1)) {
      this.path.closePath();
    }

    this.line = 1 - this.line;
  }

  point(x: number, y: number) {
    let t1 = NaN;

    x = +x;
    y = +y;

    if (x === this.x1 && y === this.y1) {
      return;
    }

    switch (this.p) {
      case 0:
        this.p = 1;
        this.line ? this.path.lineTo(x, y) : this.path.moveTo(x, y);
        break;
      case 1:
        this.p = 2;
        break;
      case 2:
        this.p = 3;
        point(this, slope2(this, (t1 = slope3(this, x, y))), t1);
        break;
      default:
        point(this, this.t0, (t1 = slope3(this, x, y)));
        break;
    }

    this.x0 = this.x1;
    this.x1 = x;
    this.y0 = this.y1;
    this.y1 = y;
    this.t0 = t1;
  }
}
