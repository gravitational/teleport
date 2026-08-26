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

// The '@testing-library/jest-dom/vitest' entry self-imports 'vitest', which
// pnpm's isolated store can't resolve from inside the jest-dom package, so
// extend expect with the matchers directly.
import * as jestDomMatchers from '@testing-library/jest-dom/matchers';
import { cleanup } from '@testing-library/react';

import '../jest/canvasMock';
import { afterAll, afterEach, beforeAll, expect } from 'vitest';
import failOnConsole from 'vitest-fail-on-console';

import { server, testQueryClient } from 'design/utils/testing';

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

expect.extend(jestDomMatchers);

failOnConsole();

// React reads this to decide whether act() is supported. RTL only sets it around its own act() calls, so without
// it React warns on every state update and vitest-fail-on-console turns that into a failure.
(
  globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true;

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterAll(() => server.close());

afterEach(() => {
  cleanup();
  testQueryClient.clear();
  server.resetHandlers();
});
