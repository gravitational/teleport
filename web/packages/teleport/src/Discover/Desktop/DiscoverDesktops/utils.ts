import { Path } from 'teleport/Discover/Desktop/DiscoverDesktops/lines/Path';
import { MonotoneX } from 'teleport/Discover/Desktop/DiscoverDesktops/lines/MonotoneX';

const STROKE_WIDTH = 4;

export interface Line {
  width: number;
  height: number;
  path: string;
}

export function createLine(
  desktopServiceElement: HTMLDivElement,
  desktopElement: HTMLDivElement,
  containerElement: HTMLDivElement
): Line {
  if (!desktopElement || !desktopServiceElement || !containerElement) {
    return null;
  }

  const desktopServiceRect = desktopServiceElement.getBoundingClientRect();
  const desktopRect = desktopElement.getBoundingClientRect();
  const containerRect = containerElement.getBoundingClientRect();

  const distance = desktopRect.left - desktopServiceRect.right;

  const path = new Path();
  const line = new MonotoneX(path);

  line.lineStart();

  const desktopLinePosition =
    desktopRect.top - containerRect.top + desktopRect.height / 2 - 1;
  const desktopServiceLinePosition =
    desktopServiceRect.top - containerRect.top + desktopServiceRect.height / 2;

  line.point(0, desktopServiceLinePosition - STROKE_WIDTH * 2);
  line.point(40, desktopServiceLinePosition - STROKE_WIDTH * 2);
  line.point(distance - 10, desktopLinePosition + STROKE_WIDTH / 2);
  line.point(distance, desktopLinePosition + STROKE_WIDTH / 2);

  line.lineEnd();

  return {
    width: distance,
    height: containerRect.height,
    path: path.path,
  };
}
