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

import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { Suspense, type ComponentType, type PropsWithChildren } from 'react';
import { ErrorBoundary, type FallbackProps } from 'react-error-boundary';

interface ErrorSuspenseWrapperProps {
  errorComponent: ComponentType<FallbackProps>;
  loadingComponent: ComponentType;
}

/**
 * ErrorSuspenseWrapper is a component that wraps its children in a
 * react-error-boundary ErrorBoundary, a react-query QueryErrorResetBoundary and
 * a Suspense component.
 *
 * It provides an easy way to add a retry mechanism for queries that fail.
 *
 * Example usage:
 *
 * ```tsx
 * <ErrorSuspenseWrapper
 *  errorComponent={MyErrorComponent}
 *  loadingComponent={MyLoadingComponent}
 * >
 *  <MyComponent />
 * </ErrorSuspenseWrapper>
 * ```
 *
 * ```tsx
 * import type { FallbackProps } from 'react-error-boundary';
 *
 * function MyErrorComponent({ error, resetErrorBoundary }: FallbackProps) {
 *   return (
 *     <div>
 *       <div role="alert">Error: {error.message}</div>
 *       <button onClick={resetErrorBoundary}>Retry</button>
 *     </div>
 *   );
 * }
 * ```
 */
export function ErrorSuspenseWrapper(
  props: PropsWithChildren<ErrorSuspenseWrapperProps>
) {
  return (
    <QueryErrorResetBoundary>
      {({ reset }) => (
        <ErrorBoundary FallbackComponent={props.errorComponent} onReset={reset}>
          <Suspense fallback={<props.loadingComponent />}>
            {props.children}
          </Suspense>
        </ErrorBoundary>
      )}
    </QueryErrorResetBoundary>
  );
}
