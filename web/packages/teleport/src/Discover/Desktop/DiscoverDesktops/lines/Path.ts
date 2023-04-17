/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
