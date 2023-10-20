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
