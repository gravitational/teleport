/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

// vmThreads VM contexts don't inherit Node globals, and MSW 2.x needs the web
// streams and BroadcastChannel at import time.
if (typeof globalThis.WritableStream === 'undefined') {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const streams = require('node:stream/web');
  globalThis.ReadableStream =
    globalThis.ReadableStream || streams.ReadableStream;
  globalThis.WritableStream = streams.WritableStream;
  globalThis.TransformStream =
    globalThis.TransformStream || streams.TransformStream;
}
if (typeof globalThis.BroadcastChannel === 'undefined') {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  globalThis.BroadcastChannel = require('node:worker_threads').BroadcastChannel;
}
// happy-dom doesn't implement requestIdleCallback (SessionRecordings timeline).
if (typeof globalThis.requestIdleCallback === 'undefined') {
  globalThis.requestIdleCallback = ((cb: IdleRequestCallback) =>
    setTimeout(
      () => cb({ didTimeout: false, timeRemaining: () => 50 } as IdleDeadline),
      0
    )) as unknown as typeof globalThis.requestIdleCallback;
  globalThis.cancelIdleCallback = ((id: number) =>
    clearTimeout(id)) as unknown as typeof globalThis.cancelIdleCallback;
}

// The '@testing-library/jest-dom/vitest' entry self-imports 'vitest', which
// pnpm's isolated store can't resolve from inside the jest-dom package, so
// extend expect with the matchers directly.
import * as jestDomMatchers from '@testing-library/jest-dom/matchers';
// Lets toHaveStyle read the injected styled-system CSS and adds toHaveStyleRule.
import 'jest-styled-components';
import '../jest/canvasMock';
import { expect } from 'vitest';
import failOnConsole from 'vitest-fail-on-console';

expect.extend(jestDomMatchers);

failOnConsole();
