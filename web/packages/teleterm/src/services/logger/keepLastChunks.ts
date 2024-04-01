/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import stream from 'node:stream';

import { RingBuffer } from 'ring-buffer-ts';

/**
 * A writeable stream that keeps the last n chunks.
 *
 * Useful for keeping last lines of stdout/stderr from a child process.
 */
export class KeepLastChunks<Chunk> extends stream.Writable {
  private chunks: RingBuffer<Chunk>;

  constructor(noOfChunks: number) {
    super({
      // Support any kind of objects as chunks.
      objectMode: true,
    });
    this.chunks = new RingBuffer(noOfChunks);
  }

  _write(
    chunk: Chunk,
    encoding: BufferEncoding,
    next: (error?: Error | null) => void
  ): void {
    this.chunks.add(chunk);

    next();
  }

  getChunks(): Chunk[] {
    return this.chunks.toArray();
  }
}
