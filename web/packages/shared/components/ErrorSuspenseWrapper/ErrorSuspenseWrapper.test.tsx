/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useSuspenseQuery } from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import type { FallbackProps } from 'react-error-boundary';

import {
  createDeferredResponse,
  render,
  screen,
  testQueryClient,
} from 'design/utils/testing';

import { ErrorSuspenseWrapper } from './ErrorSuspenseWrapper';

const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
});
afterAll(() => server.close());

function TestErrorComponent({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <div>
      <div role="alert">Error: {error.message}</div>
      <button onClick={resetErrorBoundary}>Reset</button>
    </div>
  );
}

function TestLoadingComponent() {
  return <div>Loading...</div>;
}

interface TestChildComponentProps {
  shouldError?: boolean;
}

function TestChildComponent({ shouldError = false }: TestChildComponentProps) {
  const { data } = useSuspenseQuery({
    queryKey: ['test'],
    queryFn: async () => {
      const response = await fetch('/api/test');

      if (!response.ok) {
        throw new Error('Failed to fetch');
      }

      return response.json();
    },
    retry: false,
  });

  if (shouldError) {
    throw new Error('Component error');
  }

  return <div>Data: {data?.message}</div>;
}

test('renders loading component during suspense', async () => {
  const deferred = createDeferredResponse({ message: 'Success' });

  server.use(http.get('/api/test', deferred.handler));

  render(
    <ErrorSuspenseWrapper
      errorComponent={TestErrorComponent}
      loadingComponent={TestLoadingComponent}
    >
      <TestChildComponent />
    </ErrorSuspenseWrapper>
  );

  expect(screen.getByText('Loading...')).toBeInTheDocument();

  deferred.resolve();

  expect(await screen.findByText('Data: Success')).toBeInTheDocument();
});

test('renders children when loaded successfully', async () => {
  server.use(
    http.get('/api/test', () => HttpResponse.json({ message: 'Success' }))
  );

  render(
    <ErrorSuspenseWrapper
      errorComponent={TestErrorComponent}
      loadingComponent={TestLoadingComponent}
    >
      <TestChildComponent />
    </ErrorSuspenseWrapper>
  );

  expect(await screen.findByText('Data: Success')).toBeInTheDocument();

  expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

test('renders error component when query fails', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});

  server.use(http.get('/api/test', () => HttpResponse.error()));

  render(
    <ErrorSuspenseWrapper
      errorComponent={TestErrorComponent}
      loadingComponent={TestLoadingComponent}
    >
      <TestChildComponent />
    </ErrorSuspenseWrapper>
  );

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Error: Failed to fetch'
  );

  expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
});

test('renders error component when child throws', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});

  server.use(
    http.get('/api/test', () => HttpResponse.json({ message: 'Success' }))
  );

  render(
    <ErrorSuspenseWrapper
      errorComponent={TestErrorComponent}
      loadingComponent={TestLoadingComponent}
    >
      <TestChildComponent shouldError={true} />
    </ErrorSuspenseWrapper>
  );

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Error: Component error'
  );
});

test('resets error boundary and refetches query on reset', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});

  server.use(http.get('/api/test', () => HttpResponse.error()));

  render(
    <ErrorSuspenseWrapper
      errorComponent={TestErrorComponent}
      loadingComponent={TestLoadingComponent}
    >
      <TestChildComponent />
    </ErrorSuspenseWrapper>
  );

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Error: Failed to fetch'
  );

  const deferred = createDeferredResponse({ message: 'Success after retry' });

  server.use(http.get('/api/test', deferred.handler));

  await userEvent.click(screen.getByText('Reset'));

  expect(await screen.findByText('Loading...')).toBeInTheDocument();

  deferred.resolve();

  expect(
    await screen.findByText('Data: Success after retry')
  ).toBeInTheDocument();

  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
