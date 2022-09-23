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
