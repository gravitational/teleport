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

export class Path {
  x0: number;
  y0: number;
  x1: number = null;
  y1: number = null;

  path = '';

  moveTo(x: number, y: number) {
    this.path += `M${(this.x0 = this.x1 = +x)},${(this.y0 = this.y1 = +y)}`;
  }

  closePath() {
    if (this.x1 !== null) {
      this.x1 = this.x0;
      this.y1 = this.y0;
      this.path += 'Z';
    }
  }

  lineTo(x: number, y: number) {
    this.path += `L${(this.x1 = +x)},${(this.y1 = +y)}`;
  }

  bezierCurveTo(
    x1: number,
    y1: number,
    x2: number,
    y2: number,
    x: number,
    y: number
  ) {
    this.path += `C${+x1},${+y1},${+x2},${+y2},${(this.x1 = +x)},${(this.y1 =
      +y)}`;
  }
}
