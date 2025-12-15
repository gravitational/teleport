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

import { wait } from 'shared/utils/wait';

export interface BufferEntry {
  id: number;
  text: string;
  isCommand: boolean;
  isCurrent: boolean;
}

type FrameFunction = () => Frame;

interface Frame {
  text: string;
  delay?: number;
}

export interface TerminalLine {
  text?: string;
  isCommand: boolean;
  delay?: number;
  hasFinished?: () => boolean;
  frames?: FrameFunction[];
}

export async function* createTerminalContent(
  lines: TerminalLine[],
  lineIndex: number
): AsyncIterableIterator<BufferEntry[]> {
  let linePosition = 0;
  let frameIndex = 0;
  let hasFrame = false;

  const buffer: BufferEntry[] = [];

  if (lineIndex > 0) {
    for (let i = 0; i < lineIndex; i++) {
      buffer.push({
        id: i,
        text: lines[i].text,
        isCommand: lines[i].isCommand,
        isCurrent: i === lineIndex,
      });
    }

    yield buffer;
  }

  while (true) {
    if (lineIndex < lines.length) {
      if (!lines[lineIndex].isCommand) {
        const delay = lines[lineIndex].delay;

        if (!isNaN(delay)) {
          await wait(delay);

          yield buffer;
        }

        const frames = lines[lineIndex].frames;

        if (!frames) {
          buffer.push({
            id: lineIndex,
            text: lines[lineIndex].text,
            isCommand: false,
            isCurrent: false,
          });

          yield buffer;

          linePosition = 0;
          lineIndex += 1;
        } else if (frameIndex < frames.length) {
          const frame = frames[frameIndex]();

          if (frameIndex === 0 && !hasFrame) {
            hasFrame = true;
            buffer.push({
              id: lineIndex,
              text: frame.text,
              isCurrent: false,
              isCommand: false,
            });
          }

          buffer[lineIndex].text = frame.text;

          if (!isNaN(frame.delay)) {
            yield buffer;

            await wait(frame.delay);

            yield buffer;
          }

          frameIndex += 1;
        } else {
          if (hasFrame && lines[lineIndex + 1]) {
            buffer[lineIndex].text = lines[lineIndex].text;

            linePosition = 0;
            frameIndex = 0;
            lineIndex += 1;
            hasFrame = false;
          }

          frameIndex = 0;
        }
      } else if (linePosition > lines[lineIndex].text.length) {
        buffer[lineIndex].isCurrent = lineIndex === lines.length - 1;
        linePosition = 0;

        yield buffer;

        await wait(300);

        lineIndex += 1;
      } else {
        const delay = lines[lineIndex].delay;

        if (!isNaN(delay)) {
          yield buffer;

          await wait(delay);

          yield buffer;
        }

        if (linePosition === 0) {
          await wait(100);

          buffer.push({
            id: lineIndex,
            text: '',
            isCommand: lines[lineIndex].isCommand,
            isCurrent: true,
          });

          yield buffer;

          await wait(600);
        }

        buffer[lineIndex].text = lines[lineIndex].text.substring(
          0,
          linePosition
        );

        linePosition += 1;
      }

      yield buffer;
    } else {
      yield buffer;

      return buffer;
    }
  }
}
