/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { font } from 'design/theme/fonts';

export const LEFT_PADDING = 24; // the left padding of the timeline from the edge of the screen
export const EVENT_ROW_HEIGHT = 28; // height of each row of events
export const EVENT_SECTION_PADDING = 8; // padding between the ruler, events section and the frames section
export const RULER_HEIGHT = 35; // height of the time ruler at the top of the timeline

export const BASE_FRAME_WIDTH = 240; // base width of the frame, used to calculate the scale ratio of the frame

export const DEFAULT_FRAME_HEIGHT = 250; // default height of a frame
export const DEFAULT_MAX_FRAME_WIDTH = 250; // the maximum width a frame can be

export const CANVAS_FONT = font.replace(/;$/, ''); // remove the semicolon at the end
