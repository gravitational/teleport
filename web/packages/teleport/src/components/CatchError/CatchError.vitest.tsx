/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useState } from 'react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import { render, screen, userEvent } from 'design/utils/testing';

import { CatchError } from './CatchError';

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

function Content({ fail = true }: { fail?: boolean }) {
  if (fail) {
    throw new Error('Rendering failed');
  }
  return <div>Recovered content</div>;
}

function StatefulFallback({ error, retry }: { error: Error; retry(): void }) {
  const [details, setDetails] = useState(false);
  return (
    <div>
      <button onClick={() => setDetails(!details)}>Toggle details</button>
      {details && <p>{error.message}</p>}
      <button onClick={retry}>Retry</button>
    </div>
  );
}

test('renders a fallback component with hooks and retries the failed children', async () => {
  const user = userEvent.setup();
  const onRetry = vi.fn();
  const { rerender } = render(
    <CatchError FallbackComponent={StatefulFallback} onRetry={onRetry}>
      <Content />
    </CatchError>
  );

  await user.click(screen.getByText('Toggle details'));
  expect(screen.getByText('Rendering failed')).toBeInTheDocument();

  rerender(
    <CatchError FallbackComponent={StatefulFallback} onRetry={onRetry}>
      <Content fail={false} />
    </CatchError>
  );
  expect(screen.getByText('Rendering failed')).toBeInTheDocument();
  await user.click(screen.getByText('Retry'));
  expect(screen.getByText('Recovered content')).toBeInTheDocument();
  expect(onRetry).toHaveBeenCalledTimes(1);
});

test('supports render callbacks that return a fallback component', async () => {
  const user = userEvent.setup();
  render(
    <CatchError fallbackFn={props => <StatefulFallback {...props} />}>
      <Content />
    </CatchError>
  );

  await user.click(screen.getByText('Toggle details'));
  expect(screen.getByText('Rendering failed')).toBeInTheDocument();
});

test('renders the default fallback when no custom fallback is provided', () => {
  render(
    <CatchError>
      <Content />
    </CatchError>
  );
  expect(screen.getByText('Rendering failed')).toBeInTheDocument();
});
