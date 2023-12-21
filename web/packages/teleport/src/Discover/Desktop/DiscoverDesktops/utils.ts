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
