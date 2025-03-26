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

import * as reactQuery from '@tanstack/react-query';
import {
  type DataTag,
  type DefaultError,
  type UseMutationOptions,
  type UseMutationResult,
  type UseQueryOptions,
  type UseQueryResult,
} from '@tanstack/react-query';

export type MutationHook<
  TVariables = void,
  TData = unknown,
  TError = DefaultError,
> = (
  options?: Omit<UseMutationOptions<TData, TError, TVariables>, 'mutationFn'>
) => UseMutationResult<TData, TError, TVariables>;

export function wrapMutation<
  TVariables = void,
  TData = unknown,
  TError = DefaultError,
>(
  mutationFn: (variables: TVariables) => Promise<TData>
): MutationHook<TVariables, TData, TError> {
  return function wrappedMutation(
    options?: Omit<UseMutationOptions<TData, TError, TVariables>, 'mutationFn'>
  ) {
    return reactQuery.useMutation<TData, TError, TVariables>({
      mutationFn,
      ...options,
    });
  };
}

export type QueryHook<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
> = (
  variables?: TVariables,
  options?: Omit<UseQueryOptions<TData, TError>, 'queryKey' | 'queryFn'>
) => UseQueryResult<TData, TError>;

export interface WrappedQuery<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
> {
  createQueryKey: (variables?: TVariables) => string[];
  queryKey: DataTag<string[], TData, TError>;
  useQuery: QueryHook<TData, TVariables, TError>;
}

type SignalOnlyQueryFn<TData> = (signal: AbortSignal) => Promise<TData>;
type VariablesQueryFn<TData, TVariables> = (
  variables: TVariables,
  signal: AbortSignal
) => Promise<TData>;

type QueryFn<TData, TVariables> = TVariables extends void
  ? SignalOnlyQueryFn<TData>
  : VariablesQueryFn<TData, TVariables>;

export function wrapQuery<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
>(
  queryKey: string[],
  queryFn: QueryFn<TData, TVariables>
): WrappedQuery<TData, TVariables, TError> {
  return {
    queryKey: queryKey as DataTag<string[], TData, TError>,
    createQueryKey(variables?: TVariables) {
      const key = [...queryKey];

      if (variables) {
        key.push(JSON.stringify(variables));
      }

      return key;
    },
    useQuery: function wrappedQuery(
      variables?: TVariables,
      options?: Omit<UseQueryOptions<TData, TError>, 'queryKey' | 'queryFn'>
    ) {
      const key = [...queryKey];

      if (variables) {
        key.push(JSON.stringify(variables));
      }

      return reactQuery.useQuery({
        queryKey: key,
        queryFn: ({ signal }) => callQueryFn(queryFn, variables, signal),
        ...options,
      });
    },
  };
}

function isSignalOnlyQueryFn<TData = unknown, TVariables = void>(
  queryFn: QueryFn<TData, unknown>,
  variables: TVariables
): queryFn is SignalOnlyQueryFn<TData> {
  return typeof variables === 'undefined';
}

function callQueryFn<TData = unknown, TVariables = void>(
  queryFn: QueryFn<TData, TVariables>,
  variables: TVariables,
  signal: AbortSignal
) {
  if (isSignalOnlyQueryFn(queryFn, variables)) {
    return queryFn(signal);
  }

  return queryFn(variables, signal);
}
